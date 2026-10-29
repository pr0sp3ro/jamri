package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type taskAddition struct {
	name     string
	category string
	quality  string
	focus    string
	start    time.Time
	end      time.Time
}

func addArguments(args []string, now time.Time) (taskAddition, error) {
	var addition taskAddition
	values := make(map[string]string)
	var taskArgs []string

	for i := 0; i < len(args); i++ {
		option := args[i]

		if !strings.HasPrefix(option, "--") {
			taskArgs = append(taskArgs, option)
			continue
		}

		switch option {
		case "--date", "--from", "--to", "--quality", "--focus":
		default:
			return addition, fmt.Errorf("unknown add option %q", option)
		}

		if _, exists := values[option]; exists {
			return addition, fmt.Errorf("%s specified more than once", option)
		}

		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return addition, fmt.Errorf("%s requires a value", option)
		}

		i++
		values[option] = args[i]

		if option == "--quality" || option == "--focus" {
			taskArgs = append(taskArgs, option, args[i])
		}
	}

	if values["--from"] == "" || values["--to"] == "" {
		return addition, errors.New("add requires --from HH:MM and --to HH:MM")
	}

	var err error
	addition.name, addition.category, addition.quality, addition.focus, err = taskArguments("start", taskArgs)

	if err != nil {
		return addition, fmt.Errorf("add: %w", err)
	}

	date, provided := values["--date"]

	if !provided {
		date = now.Format("2006-01-02")
	}

	const layout = "2006-01-02 15:04"

	for _, field := range []struct {
		option string
		target *time.Time
	}{{"--from", &addition.start}, {"--to", &addition.end}} {
		input := date + " " + values[field.option]
		parsed, err := time.ParseInLocation(layout, input, now.Location())

		if err != nil || parsed.Format(layout) != input {
			return addition, fmt.Errorf("invalid date or time for %s: use --date YYYY-MM-DD and HH:MM in the local timezone", field.option)
		}

		*field.target = parsed
	}

	if !addition.end.After(addition.start) {
		return addition, errors.New("end time must be after start time on the same day")
	}

	if addition.end.After(now) {
		return addition, errors.New("end time must not be in the future")
	}

	return addition, nil
}

func addTask(db *sql.DB, addition taskAddition) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	var conflictID int64
	err = tx.QueryRow(`
		SELECT id FROM tasks
		WHERE started_at < ? AND (ended_at IS NULL OR ended_at > ?)
		ORDER BY started_at, id LIMIT 1
	`, addition.end.Unix(), addition.start.Unix()).Scan(&conflictID)

	if err == nil {
		return fmt.Errorf("time range overlaps task %d", conflictID)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	categoryID, err := ensureCategory(tx, addition.category)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO tasks (name, started_at, ended_at, category_id, quality, focus)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))
	`, addition.name, addition.start.Unix(), addition.end.Unix(), categoryID, addition.quality, addition.focus)

	if err != nil {
		return err
	}

	return tx.Commit()
}
