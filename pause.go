package main

import (
	"database/sql"
	"errors"
	"time"
)

func togglePause(db *sql.DB) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	var name string
	var elapsedSeconds int64
	var pausedAt sql.NullInt64
	err = tx.QueryRow("SELECT name, elapsed_seconds, paused_at FROM task_state WHERE id = 1").Scan(&name, &elapsedSeconds, &pausedAt)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	now := time.Now().Unix()

	if pausedAt.Valid {
		if _, err := tx.Exec("INSERT INTO entries (name, started_at) VALUES (?, ?)", name, now); err != nil {
			return err
		}

		if _, err := tx.Exec("UPDATE task_state SET paused_at = NULL WHERE id = 1"); err != nil {
			return err
		}
	} else {
		var startedAt int64
		err = tx.QueryRow("SELECT name, started_at FROM entries WHERE ended_at IS NULL").Scan(&name, &startedAt)

		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no active task to pause")
		}

		if err != nil {
			return err
		}

		elapsedSeconds += max(int64(0), now-startedAt)

		if _, err := tx.Exec("UPDATE entries SET ended_at = ? WHERE ended_at IS NULL", now); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO task_state (id, name, elapsed_seconds, paused_at)
			VALUES (1, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
			    name = excluded.name,
			    elapsed_seconds = excluded.elapsed_seconds,
			    paused_at = excluded.paused_at
		`, name, elapsedSeconds, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}
