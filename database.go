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
	dsn.RawQuery = query.Encode()
	db, err := sql.Open("sqlite3", dsn.String())

	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS entries (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    ended_at INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS entries_one_active
    ON entries ((1)) WHERE ended_at IS NULL;
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

	if _, err := tx.Exec("UPDATE entries SET ended_at = ? WHERE ended_at IS NULL", now); err != nil {
		return err
	}

	if _, err := tx.Exec("INSERT INTO entries (name, started_at) VALUES (?, ?)", name, now); err != nil {
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

	if _, err := tx.Exec("UPDATE entries SET ended_at = ? WHERE ended_at IS NULL", time.Now().Unix()); err != nil {
		return err
	}

	return tx.Commit()
}
