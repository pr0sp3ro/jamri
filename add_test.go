package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddArguments(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")

	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 29, 18, 0, 0, 0, location)

	for _, date := range [][]string{nil, {"--date", "2026-10-29"}} {
		args := append([]string{"--focus", "high", "dev", "working on an app", "--from", "14:00", "--quality", "good", "--to", "15:30"}, date...)
		addition, err := addArguments(args, now)

		if err != nil {
			t.Fatal(err)
		}

		if addition.name != "working on an app" || addition.category != "dev" || addition.focus != "high" || addition.quality != "good" {
			t.Fatalf("unexpected task: %+v", addition)
		}

		wantStart := time.Date(2026, 10, 29, 14, 0, 0, 0, location)

		if !addition.start.Equal(wantStart) || addition.end.Sub(addition.start) != 90*time.Minute {
			t.Fatalf("unexpected range: %v - %v", addition.start, addition.end)
		}
	}

	addition, err := addArguments([]string{"task", "--from", "14:00", "--to", "18:00"}, now)

	if err != nil {
		t.Fatal(err)
	}

	if addition.category != "" || addition.quality != "" || addition.focus != "" || !addition.end.Equal(now) {
		t.Fatalf("unexpected defaults: %+v", addition)
	}

	invalid := []struct {
		name string
		args []string
	}{
		{"missing name", nil},
		{"extra positional", []string{"dev", "task", "extra"}},
		{"empty name", []string{" "}},
		{"invalid category", []string{"Dev", "task"}},
		{"control character", []string{"task\nname"}},
		{"unknown option", []string{"task", "--other", "value"}},
		{"duplicate time", []string{"task", "--from", "13:00"}},
		{"duplicate date", []string{"task", "--date", "2026-10-28", "--date", "2026-10-29"}},
		{"duplicate focus", []string{"task", "--focus", "high", "--focus", "low"}},
		{"missing value", []string{"task", "--focus"}},
		{"option as value", []string{"task", "--focus", "--quality", "good"}},
		{"empty focus", []string{"task", "--focus", ""}},
		{"blank quality", []string{"task", "--quality", " "}},
		{"multiline focus", []string{"task", "--focus", "high\nlow"}},
		{"invalid date", []string{"task", "--date", "2026-02-30"}},
		{"empty date", []string{"task", "--date", ""}},
		{"relative date", []string{"task", "--date", "yesterday"}},
	}

	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"--from", "14:00", "--to", "15:30"}, test.args...)

			if _, err := addArguments(args, now); err == nil {
				t.Fatal("expected invalid arguments to be rejected")
			}
		})
	}

	for _, test := range []struct {
		name string
		from string
		to   string
		date string
	}{
		{"missing start", "", "15:30", "2026-10-29"},
		{"missing end", "14:00", "", "2026-10-29"},
		{"short hour", "9:00", "15:30", "2026-10-29"},
		{"invalid hour", "24:00", "15:30", "2026-10-29"},
		{"invalid minute", "14:60", "15:30", "2026-10-29"},
		{"seconds", "14:00:00", "15:30", "2026-10-29"},
		{"equal times", "14:00", "14:00", "2026-10-29"},
		{"overnight", "23:00", "01:00", "2026-10-28"},
		{"future time", "17:00", "18:01", "2026-10-29"},
		{"future date", "14:00", "15:30", "2026-10-30"},
		{"nonexistent DST time", "02:30", "04:00", "2026-03-29"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"task", "--date", test.date}

			if test.from != "" {
				args = append(args, "--from", test.from)
			}

			if test.to != "" {
				args = append(args, "--to", test.to)
			}

			if _, err := addArguments(args, now); err == nil {
				t.Fatal("expected invalid time range to be rejected")
			}
		})
	}
}

func TestAddTask(t *testing.T) {
	for _, state := range []string{"empty", "adjacent before", "adjacent after", "active adjacent", "overlap", "contained", "contains", "duplicate", "active", "paused"} {
		t.Run(state, func(t *testing.T) {
			db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "jamri.sqlite3")+"?_txlock=immediate&_foreign_keys=on")

			if err != nil {
				t.Fatal(err)
			}

			defer db.Close()

			if err := migrateDatabase(db); err != nil {
				t.Fatal(err)
			}

			start := time.Date(2026, 10, 29, 14, 0, 0, 0, time.UTC)
			end := start.Add(90 * time.Minute)
			addition := taskAddition{name: "task", category: "dev", quality: "good", focus: "high", start: start, end: end}
			conflict := state != "empty" && state != "adjacent before" && state != "adjacent after" && state != "active adjacent"
			var existingEnd any = end.Unix()
			existingStart := start.Unix()

			switch state {
			case "adjacent before":
				existingStart = start.Add(-time.Hour).Unix()
				existingEnd = start.Unix()
			case "adjacent after":
				existingStart = end.Unix()
				existingEnd = end.Add(time.Hour).Unix()
			case "active adjacent":
				existingStart = end.Unix()
				existingEnd = nil
			case "overlap":
				existingStart = start.Add(-time.Hour).Unix()
				existingEnd = start.Add(time.Minute).Unix()
			case "contained":
				existingStart = start.Add(-time.Hour).Unix()
				existingEnd = end.Add(time.Hour).Unix()
			case "contains":
				existingStart = start.Add(time.Minute).Unix()
				existingEnd = end.Add(-time.Minute).Unix()
			case "active", "paused":
				existingEnd = nil
			}

			if state != "empty" {
				if _, err := db.Exec("INSERT INTO tasks (id, name, started_at, ended_at) VALUES (42, 'existing', ?, ?)", existingStart, existingEnd); err != nil {
					t.Fatal(err)
				}
			}

			if state == "paused" {
				if _, err := db.Exec("INSERT INTO pauses (task_id, started_at, reason) VALUES (42, ?, 'break')", start.Unix()); err != nil {
					t.Fatal(err)
				}
			}

			err = addTask(db, addition)

			if conflict {
				if err == nil || !strings.Contains(err.Error(), "task 42") {
					t.Fatalf("expected conflict with task 42, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			var tasks, categories, pauses int

			if err := db.QueryRow("SELECT (SELECT COUNT(*) FROM tasks WHERE id != 42), (SELECT COUNT(*) FROM categories), (SELECT COUNT(*) FROM pauses)").Scan(&tasks, &categories, &pauses); err != nil {
				t.Fatal(err)
			}

			wantAdded := 1

			if conflict {
				wantAdded = 0
			}

			if tasks != wantAdded || categories != wantAdded {
				t.Fatalf("unexpected rows: tasks=%d categories=%d", tasks, categories)
			}

			if !conflict {
				var name, category, quality, focus string
				var savedStart, savedEnd int64
				err := db.QueryRow(`SELECT t.name, c.name, t.quality, t.focus, t.started_at, t.ended_at
					FROM tasks t JOIN categories c ON c.id = t.category_id WHERE t.id != 42`).Scan(&name, &category, &quality, &focus, &savedStart, &savedEnd)

				if err != nil {
					t.Fatal(err)
				}

				if name != addition.name || category != addition.category || quality != addition.quality || focus != addition.focus || savedStart != start.Unix() || savedEnd != end.Unix() || pauses != 0 {
					t.Fatal("saved task differs from the requested completed task without pauses")
				}
			}

			if state == "active" || state == "paused" || state == "active adjacent" {
				var name string
				var savedStart int64
				var savedEnd sql.NullInt64

				if err := db.QueryRow("SELECT name, started_at, ended_at FROM tasks WHERE id = 42").Scan(&name, &savedStart, &savedEnd); err != nil {
					t.Fatal(err)
				}

				if name != "existing" || savedStart != existingStart || savedEnd.Valid {
					t.Fatal("active task was changed")
				}
			}

			if state == "paused" {
				var pauseStart int64
				var pauseEnd sql.NullInt64
				var reason string

				if err := db.QueryRow("SELECT started_at, ended_at, reason FROM pauses WHERE task_id = 42").Scan(&pauseStart, &pauseEnd, &reason); err != nil {
					t.Fatal(err)
				}

				if pauses != 1 || pauseStart != start.Unix() || pauseEnd.Valid || reason != "break" {
					t.Fatal("active pause was changed")
				}
			}
		})
	}
}
