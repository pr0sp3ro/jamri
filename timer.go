package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func readStatus(db *sql.DB) (string, error) {
	var startedAt int64
	var name string
	row := db.QueryRow(`
		SELECT name, started_at
		FROM entries
		WHERE ended_at IS NULL
	`)
	err := row.Scan(&name, &startedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return "idle", nil
	}

	if err != nil {
		return "", err
	}

	elapsedSeconds := max(int64(0), time.Now().Unix()-startedAt)
	hours := elapsedSeconds / 3600
	minutes := elapsedSeconds / 60 % 60
	seconds := elapsedSeconds % 60

	return fmt.Sprintf("%s  %02d:%02d:%02d", name, hours, minutes, seconds), nil
}
