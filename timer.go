package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func readStatus(db *sql.DB) (string, error) {
	var startedAt, carriedSeconds int64
	var paused bool
	var name string
	row := db.QueryRow(`
		SELECT name, paused_at, elapsed_seconds, 1
		FROM task_state WHERE paused_at IS NOT NULL
		UNION ALL
		SELECT name, started_at,
		    COALESCE((SELECT elapsed_seconds FROM task_state WHERE id = 1), 0), 0
		FROM entries WHERE ended_at IS NULL
	`)
	err := row.Scan(&name, &startedAt, &carriedSeconds, &paused)

	if errors.Is(err, sql.ErrNoRows) {
		return "idle", nil
	}

	if err != nil {
		return "", err
	}

	elapsedSeconds := carriedSeconds

	if paused {
		name += " (paused)"
	} else {
		elapsedSeconds += max(int64(0), time.Now().Unix()-startedAt)
	}
	hours := elapsedSeconds / 3600
	minutes := elapsedSeconds / 60 % 60
	seconds := elapsedSeconds % 60

	return fmt.Sprintf("%s  %02d:%02d:%02d", name, hours, minutes, seconds), nil
}
