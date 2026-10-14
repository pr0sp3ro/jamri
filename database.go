package main

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openDatabase() (*sql.DB, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")

	if dataHome == "" || !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()

		if err != nil {
			return nil, err
		}

		dataHome = filepath.Join(home, ".local", "share")
	}

	dir := filepath.Join(dataHome, "tickd")

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	dsn := url.URL{Scheme: "file", Path: filepath.Join(dir, "tickd.sqlite3")}
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
	_, err = db.Exec(`
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
		db.Close()

		return nil, err
	}

	return db, nil
}

func startTask(db *sql.DB, name string) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	now := time.Now().Unix()

	if err := closeTask(tx, now); err != nil {
		return err
	}

	if _, err := tx.Exec("INSERT INTO tasks (name, started_at) VALUES (?, ?)", name, now); err != nil {
		return err
	}

	return tx.Commit()
}

func stopTask(db *sql.DB) error {
	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	if err := closeTask(tx, time.Now().Unix()); err != nil {
		return err
	}

	return tx.Commit()
}

func closeTask(tx *sql.Tx, now int64) error {
	if _, err := tx.Exec("UPDATE pauses SET ended_at = MAX(started_at, ?) WHERE ended_at IS NULL", now); err != nil {
		return err
	}

	_, err := tx.Exec("UPDATE tasks SET ended_at = MAX(started_at, ?) WHERE ended_at IS NULL", now)

	return err
}
