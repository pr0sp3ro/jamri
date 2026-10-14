package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func readStatus(db *sql.DB) (string, error) {
	var name string
	var elapsedSeconds int64
	var paused bool
	now := time.Now().Unix()
	err := db.QueryRow(`
 SELECT name,
 MAX(0, ? - started_at) - COALESCE((
     SELECT SUM(MAX(0, COALESCE(ended_at, ?) - started_at))
     FROM pauses WHERE task_id = tasks.id
 ), 0),
 EXISTS (SELECT 1 FROM pauses WHERE task_id = tasks.id AND ended_at IS NULL)
 FROM tasks WHERE ended_at IS NULL
 `, now, now).Scan(&name, &elapsedSeconds, &paused)

	if errors.Is(err, sql.ErrNoRows) {
		return "idle", nil
	}

	if err != nil {
		return "", err
	}

	if paused {
		name += " (paused)"
	}

	elapsedSeconds = max(int64(0), elapsedSeconds)
	hours := elapsedSeconds / 3600
	minutes := elapsedSeconds / 60 % 60
	seconds := elapsedSeconds % 60

	return fmt.Sprintf("%s  %02d:%02d:%02d", name, hours, minutes, seconds), nil
}
