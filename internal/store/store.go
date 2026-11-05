// Package store keeps tasks, pauses and categories in SQLite.
package store

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func Open() (*Store, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")

	if dataHome == "" || !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()

		if err != nil {
			return nil, err
		}

		dataHome = filepath.Join(home, ".local", "share")
	}

	dir := filepath.Join(dataHome, "jamri")

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	dsn := url.URL{Scheme: "file", Path: filepath.Join(dir, "jamri.sqlite3")}
	query := url.Values{}
	query.Set("_busy_timeout", "5000")
	query.Set("_txlock", "immediate")
	query.Set("_foreign_keys", "on")
	dsn.RawQuery = query.Encode()
	db, err := sql.Open("sqlite3", dsn.String())

	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()

		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	var current bool
	err := db.QueryRow(`
		SELECT
		    (SELECT COUNT(*) FROM sqlite_master WHERE
		        (type = 'table' AND name IN ('categories', 'tasks', 'pauses')) OR
		        (type = 'index' AND name IN ('tasks_one_active', 'pauses_one_active', 'pauses_task'))
		    ) = 6
		    AND (SELECT COUNT(*) FROM pragma_table_info('tasks')
		        WHERE name IN ('quality', 'focus', 'category_id', 'max_seconds', 'max_notified_at')) = 5
	`).Scan(&current)

	if err != nil || current {
		return err
	}

	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	_, err = tx.Exec(`
CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    ended_at INTEGER
);
CREATE TABLE IF NOT EXISTS pauses (
    id INTEGER PRIMARY KEY,
    task_id INTEGER NOT NULL REFERENCES tasks(id),
    started_at INTEGER NOT NULL,
    ended_at INTEGER,
    reason TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS tasks_one_active
    ON tasks ((1)) WHERE ended_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS pauses_one_active
    ON pauses ((1)) WHERE ended_at IS NULL;
CREATE INDEX IF NOT EXISTS pauses_task ON pauses (task_id, started_at);
`)

	if err != nil {
		return err
	}

	for _, column := range []string{"quality", "focus", "category_id", "max_seconds", "max_notified_at"} {
		var exists bool

		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM pragma_table_info('tasks') WHERE name = ?)", column).Scan(&exists); err != nil {
			return err
		}

		if !exists {
			definition := "TEXT"

			switch column {
			case "category_id":
				definition = "INTEGER REFERENCES categories(id)"
			case "max_seconds", "max_notified_at":
				definition = "INTEGER"
			}

			if _, err := tx.Exec("ALTER TABLE tasks ADD COLUMN " + column + " " + definition); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func operationTime(tx *sql.Tx) (int64, error) {
	var last sql.NullInt64
	err := tx.QueryRow(`
		SELECT MAX(timestamp) FROM (
		    SELECT MAX(started_at) AS timestamp FROM tasks
		    UNION ALL SELECT MAX(ended_at) FROM tasks
		    UNION ALL SELECT MAX(started_at) FROM pauses
		    UNION ALL SELECT MAX(ended_at) FROM pauses
		)
	`).Scan(&last)

	return max(time.Now().Unix(), last.Int64), err
}
