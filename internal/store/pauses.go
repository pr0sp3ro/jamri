package store

import (
	"database/sql"
	"errors"
)

// TaskToPause returns the ID of the running task that is not paused.
func (s *Store) TaskToPause() (int64, error) {
	var taskID int64
	err := s.db.QueryRow(`
		SELECT id FROM tasks
		WHERE ended_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM pauses WHERE ended_at IS NULL)
	`).Scan(&taskID)

	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("no running task to pause")
	}

	if err != nil {
		return 0, err
	}

	return taskID, nil
}

// Pause fails when the task from TaskToPause changed in the meantime, for
// example while the reason was being entered.
func (s *Store) Pause(taskID int64, reason string) error {
	tx, err := s.db.Begin()

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

func (s *Store) Resume() error {
	tx, err := s.db.Begin()

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
