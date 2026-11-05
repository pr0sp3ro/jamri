package store

import (
	"database/sql"
	"errors"
	"fmt"
)

func ensureCategory(tx *sql.Tx, name string) (sql.NullInt64, error) {
	var id sql.NullInt64

	if name == "" {
		return id, nil
	}

	if _, err := tx.Exec("INSERT INTO categories (name) VALUES (?) ON CONFLICT(name) DO NOTHING", name); err != nil {
		return id, err
	}

	err := tx.QueryRow("SELECT id FROM categories WHERE name = ?", name).Scan(&id)

	return id, err
}

// Categories returns category names sorted alphabetically.
func (s *Store) Categories() ([]string, error) {
	rows, err := s.db.Query("SELECT name FROM categories ORDER BY name")

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var names []string

	for rows.Next() {
		var name string

		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		names = append(names, name)
	}

	return names, rows.Err()
}

// AddCategory does nothing when the category already exists.
func (s *Store) AddCategory(name string) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := ensureCategory(tx, name); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) RenameCategory(oldName, newName string) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()
	var id int64
	err = tx.QueryRow("SELECT id FROM categories WHERE name = ?", oldName).Scan(&id)

	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("category %q not found", oldName)
	}

	if err != nil {
		return err
	}

	var exists bool

	if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE name = ? AND id != ?)", newName, id).Scan(&exists); err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("category %q already exists", newName)
	}

	if _, err := tx.Exec("UPDATE categories SET name = ? WHERE id = ?", newName, id); err != nil {
		return err
	}

	return tx.Commit()
}
