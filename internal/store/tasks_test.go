package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddTask(t *testing.T) {
	for _, state := range []string{"empty", "adjacent before", "adjacent after", "active adjacent", "overlap", "contained", "contains", "duplicate", "active", "paused"} {
		t.Run(state, func(t *testing.T) {
			db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "jamri.sqlite3")+"?_txlock=immediate&_foreign_keys=on")

			if err != nil {
				t.Fatal(err)
			}

			defer db.Close()

			if err := migrate(db); err != nil {
				t.Fatal(err)
			}

			start := time.Date(2026, 10, 29, 14, 0, 0, 0, time.UTC)
			end := start.Add(90 * time.Minute)
			addition := CompletedTask{Name: "task", Category: "dev", Quality: "good", Focus: "high", Start: start, End: end}
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

			err = (&Store{db: db}).AddTask(addition)

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

				if name != addition.Name || category != addition.Category || quality != addition.Quality || focus != addition.Focus || savedStart != start.Unix() || savedEnd != end.Unix() || pauses != 0 {
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
