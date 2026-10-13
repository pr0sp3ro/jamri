package main

import (
	"database/sql"
	"errors"
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
	return name + "  " + formatDuration(elapsedSeconds), nil
}
