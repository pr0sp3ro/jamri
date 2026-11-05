package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const maxTimeSound = "/usr/share/sounds/freedesktop/stereo/alarm-clock-elapsed.oga"

func readStatus(db *sql.DB) (string, error) {
	var taskID int64
	var name string
	var elapsedSeconds int64
	var paused bool
	var pendingMaxSeconds sql.NullInt64
	now := time.Now().Unix()
	err := db.QueryRow(`
 SELECT id, name,
 MAX(0, ? - started_at) - COALESCE((
     SELECT SUM(MAX(0, COALESCE(ended_at, ?) - started_at))
     FROM pauses WHERE task_id = tasks.id
 ), 0),
 EXISTS (SELECT 1 FROM pauses WHERE task_id = tasks.id AND ended_at IS NULL),
 CASE WHEN max_notified_at IS NULL THEN max_seconds END
 FROM tasks WHERE ended_at IS NULL
 `, now, now).Scan(&taskID, &name, &elapsedSeconds, &paused, &pendingMaxSeconds)

	if errors.Is(err, sql.ErrNoRows) {
		return "idle", nil
	}

	if err != nil {
		return "", err
	}

	elapsedSeconds = max(int64(0), elapsedSeconds)

	// jamri runs no background process, so the periodic status call from
	// the status bar is what detects the reached limit.
	if pendingMaxSeconds.Valid && elapsedSeconds >= pendingMaxSeconds.Int64 {
		if err := notifyMaxTime(db, taskID, name, pendingMaxSeconds.Int64); err != nil {
			// The alert is best effort and must not hide the status line.
			fmt.Fprintln(os.Stderr, "jamri: max time alert:", err)
		}
	}

	if paused {
		name += " (paused)"
	}

	hours := elapsedSeconds / 3600
	minutes := elapsedSeconds / 60 % 60
	seconds := elapsedSeconds % 60

	return fmt.Sprintf("%s  %02d:%02d:%02d", name, hours, minutes, seconds), nil
}

func notifyMaxTime(db *sql.DB, taskID int64, name string, maxSeconds int64) error {
	// The flag is set before alerting, so concurrent status calls alert once
	// and a failed alert is not repeated every second.
	result, err := db.Exec(`
		UPDATE tasks SET max_notified_at = ?
		WHERE id = ? AND ended_at IS NULL AND max_notified_at IS NULL
	`, time.Now().Unix(), taskID)

	if err != nil {
		return err
	}

	count, err := result.RowsAffected()

	if err != nil || count == 0 {
		return err
	}

	// Commands are started without waiting, so the sound does not delay
	// the status line. Their output goes to /dev/null.
	body := "Max time reached: " + formatDuration(maxSeconds)
	notifyErr := exec.Command("notify-send", "--app-name=jamri", "--", name, body).Start()

	return errors.Join(notifyErr, playMaxTimeSound())
}

func playMaxTimeSound() error {
	if _, err := os.Stat(maxTimeSound); err != nil {
		return err
	}

	err := exec.Command("pw-play", maxTimeSound).Start()

	if errors.Is(err, exec.ErrNotFound) {
		err = exec.Command("paplay", maxTimeSound).Start()
	}

	return err
}
