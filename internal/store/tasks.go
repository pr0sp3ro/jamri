package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CompletedTask is a finished task recorded with an explicit time range.
type CompletedTask struct {
	Name     string
	Category string
	Quality  string
	Focus    string
	Start    time.Time
	End      time.Time
}

// TaskEdit lists changes to a completed task. Nil fields stay unchanged.
type TaskEdit struct {
	ID       int64
	Name     *string
	Quality  *string
	Focus    *string
	Category *string
}

type ActiveTask struct {
	ID             int64
	Name           string
	ElapsedSeconds int64
	Paused         bool

	// PendingMaxSeconds is the max time that was not notified yet, or 0.
	PendingMaxSeconds int64
}

func (s *Store) StartTask(name, category, quality, focus string, maxSeconds int64) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	now, err := operationTime(tx)

	if err != nil {
		return err
	}

	if err := closeTask(tx, now, quality, focus); err != nil {
		return err
	}

	categoryID, err := ensureCategory(tx, category)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO tasks (name, started_at, category_id, max_seconds)
		VALUES (?, ?, ?, NULLIF(?, 0))
	`, name, now, categoryID, maxSeconds)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) StopTask(quality, focus string) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	now, err := operationTime(tx)

	if err != nil {
		return err
	}

	if err := closeTask(tx, now, quality, focus); err != nil {
		return err
	}

	return tx.Commit()
}

func closeTask(tx *sql.Tx, now int64, quality, focus string) error {
	if _, err := tx.Exec("UPDATE pauses SET ended_at = MAX(started_at, ?) WHERE ended_at IS NULL", now); err != nil {
		return err
	}

	result, err := tx.Exec(`
		UPDATE tasks SET ended_at = MAX(started_at, ?),
		    quality = NULLIF(?, ''), focus = NULLIF(?, '')
		WHERE ended_at IS NULL
	`, now, quality, focus)

	if err != nil {
		return err
	}

	if quality != "" || focus != "" {
		count, err := result.RowsAffected()

		if err != nil {
			return err
		}

		if count == 0 {
			return errors.New("no active task to rate")
		}
	}

	return nil
}

func (s *Store) AddTask(task CompletedTask) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	var conflictID int64
	err = tx.QueryRow(`
		SELECT id FROM tasks
		WHERE started_at < ? AND (ended_at IS NULL OR ended_at > ?)
		ORDER BY started_at, id LIMIT 1
	`, task.End.Unix(), task.Start.Unix()).Scan(&conflictID)

	if err == nil {
		return fmt.Errorf("time range overlaps task %d", conflictID)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	categoryID, err := ensureCategory(tx, task.Category)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO tasks (name, started_at, ended_at, category_id, quality, focus)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))
	`, task.Name, task.Start.Unix(), task.End.Unix(), categoryID, task.Quality, task.Focus)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) EditTask(edit TaskEdit) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	var endedAt sql.NullInt64
	err = tx.QueryRow("SELECT ended_at FROM tasks WHERE id = ?", edit.ID).Scan(&endedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("task %d not found", edit.ID)
	}

	if err != nil {
		return err
	}

	if !endedAt.Valid {
		return errors.New("only completed tasks can be edited")
	}

	var categoryID sql.NullInt64

	if edit.Category != nil {
		categoryID, err = ensureCategory(tx, *edit.Category)

		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
		UPDATE tasks SET name = COALESCE(?, name),
		    quality = CASE WHEN ? THEN NULLIF(?, '') ELSE quality END,
		    focus = CASE WHEN ? THEN NULLIF(?, '') ELSE focus END,
		    category_id = CASE WHEN ? THEN ? ELSE category_id END
		WHERE id = ?
	`, edit.Name, edit.Quality != nil, edit.Quality, edit.Focus != nil, edit.Focus, edit.Category != nil, categoryID, edit.ID)

	if err != nil {
		return err
	}

	return tx.Commit()
}

// ActiveTask returns false when no task is active.
func (s *Store) ActiveTask() (ActiveTask, bool, error) {
	var task ActiveTask
	var pendingMaxSeconds sql.NullInt64
	now := time.Now().Unix()
	err := s.db.QueryRow(`
 SELECT id, name,
 MAX(0, ? - started_at) - COALESCE((
     SELECT SUM(MAX(0, COALESCE(ended_at, ?) - started_at))
     FROM pauses WHERE task_id = tasks.id
 ), 0),
 EXISTS (SELECT 1 FROM pauses WHERE task_id = tasks.id AND ended_at IS NULL),
 CASE WHEN max_notified_at IS NULL THEN max_seconds END
 FROM tasks WHERE ended_at IS NULL
 `, now, now).Scan(&task.ID, &task.Name, &task.ElapsedSeconds, &task.Paused, &pendingMaxSeconds)

	if errors.Is(err, sql.ErrNoRows) {
		return task, false, nil
	}

	if err != nil {
		return task, false, err
	}

	task.ElapsedSeconds = max(int64(0), task.ElapsedSeconds)
	task.PendingMaxSeconds = pendingMaxSeconds.Int64

	return task, true, nil
}

// MarkMaxTimeNotified returns false when the task has ended or was already
// marked, so concurrent callers can alert only once.
func (s *Store) MarkMaxTimeNotified(taskID int64) (bool, error) {
	result, err := s.db.Exec(`
		UPDATE tasks SET max_notified_at = ?
		WHERE id = ? AND ended_at IS NULL AND max_notified_at IS NULL
	`, time.Now().Unix(), taskID)

	if err != nil {
		return false, err
	}

	count, err := result.RowsAffected()

	if err != nil {
		return false, err
	}

	return count == 1, nil
}
