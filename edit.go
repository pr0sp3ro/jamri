package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type taskEdit struct {
	id      int64
	name    *string
	quality *string
	focus   *string
}

func editArguments(args []string) (taskEdit, error) {
	var edit taskEdit

	if len(args) == 0 {
		return edit, errors.New("edit requires a positive task ID and at least one option")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)

	if err != nil || id <= 0 {
		return edit, errors.New("edit requires a positive task ID")
	}

	edit.id = id

	for i := 1; i < len(args); i++ {
		option := args[i]
		var field **string

		switch option {
		case "--name":
			field = &edit.name
		case "--quality":
			field = &edit.quality
		case "--focus":
			field = &edit.focus
		default:
			return edit, fmt.Errorf("unknown edit option %q", option)
		}

		if *field != nil {
			return edit, fmt.Errorf("%s specified more than once", option)
		}

		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return edit, fmt.Errorf("%s requires a value", option)
		}

		i++
		value := args[i]

		if strings.TrimSpace(value) == "" && (option == "--name" || value != "") {
			return edit, fmt.Errorf("%s requires non-empty text", option)
		}

		for _, character := range value {
			if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
				return edit, fmt.Errorf("%s must be a single line without control characters", option)
			}
		}

		*field = &value
	}

	if edit.name == nil && edit.quality == nil && edit.focus == nil {
		return edit, errors.New("edit requires at least one of --name, --quality, --focus")
	}

	return edit, nil
}

func editTask(db *sql.DB, edit taskEdit) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	var endedAt sql.NullInt64
	err = tx.QueryRow("SELECT ended_at FROM tasks WHERE id = ?", edit.id).Scan(&endedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("task %d not found", edit.id)
	}

	if err != nil {
		return err
	}

	if !endedAt.Valid {
		return errors.New("only completed tasks can be edited")
	}

	_, err = tx.Exec(`
		UPDATE tasks SET name = COALESCE(?, name),
		    quality = CASE WHEN ? THEN NULLIF(?, '') ELSE quality END,
		    focus = CASE WHEN ? THEN NULLIF(?, '') ELSE focus END
		WHERE id = ?
	`, edit.name, edit.quality != nil, edit.quality, edit.focus != nil, edit.focus, edit.id)

	if err != nil {
		return err
	}

	return tx.Commit()
}
