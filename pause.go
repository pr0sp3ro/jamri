package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
)

func pauseTask(db *sql.DB, args []string) error {
	var taskID int64
	err := db.QueryRow(`
		SELECT id FROM tasks
		WHERE ended_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM pauses WHERE ended_at IS NULL)
	`).Scan(&taskID)

	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("no running task to pause")
	}

	if err != nil {
		return err
	}

	reason := ""

	if len(args) == 1 {
		reason = args[0]
	} else {
		if _, err := fmt.Fprint(os.Stderr, "Pause reason (optional): "); err != nil {
			return err
		}

		line, err := bufio.NewReader(os.Stdin).ReadString('\n')

		if err != nil {
			return fmt.Errorf("pause cancelled: %w", err)
		}

		reason = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}

	for _, character := range reason {
		if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
			return errors.New("pause reason must be a single line without control characters")
		}
	}

	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	now, err := operationTime(tx)

	if err != nil {
		return err
	}

	result, err := tx.Exec(`
		INSERT INTO pauses (task_id, started_at, reason)
		SELECT id, MAX(started_at, ?), ? FROM tasks
		WHERE id = ? AND ended_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM pauses WHERE ended_at IS NULL)
	`, now, reason, taskID)

	if err != nil {
		return err
	}

	count, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if count != 1 {
		return errors.New("task changed while entering the reason, pause cancelled")
	}

	return tx.Commit()
}

func resumeTask(db *sql.DB) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	now, err := operationTime(tx)

	if err != nil {
		return err
	}

	result, err := tx.Exec("UPDATE pauses SET ended_at = MAX(started_at, ?) WHERE ended_at IS NULL", now)

	if err != nil {
		return err
	}

	count, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if count == 0 {
		return errors.New("no paused task to resume")
	}

	return tx.Commit()
}
