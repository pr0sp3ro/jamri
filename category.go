package main

import (
	"database/sql"
	"errors"
	"fmt"
)

func validateCategoryName(name string) error {
	if name == "" {
		return errors.New("category name must not be empty")
	}

	if name[0] < 'a' || name[0] > 'z' {
		return errors.New("category name must start with a lowercase ASCII letter (a-z)")
	}

	for _, character := range name {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
			return errors.New("category names may contain only a-z, 0-9, and hyphens")
		}
	}

	return nil
}

func categoryArguments(args []string) error {
	if len(args) == 1 && args[0] == "list" {
		return nil
	}

	if len(args) == 2 && args[0] == "add" || len(args) == 3 && args[0] == "rename" {
		for _, name := range args[1:] {
			if err := validateCategoryName(name); err != nil {
				return err
			}
		}

		return nil
	}

	return errors.New("use category list, category add NAME, or category rename OLD NEW")
}

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

func manageCategory(db *sql.DB, args []string) error {
	if args[0] == "list" {
		rows, err := db.Query("SELECT name FROM categories ORDER BY name")

		if err != nil {
			return err
		}

		defer rows.Close()
		found := false

		for rows.Next() {
			var name string

			if err := rows.Scan(&name); err != nil {
				return err
			}

			if _, err := fmt.Println(name); err != nil {
				return err
			}

			found = true
		}

		if err := rows.Err(); err != nil {
			return err
		}

		if !found {
			_, err = fmt.Println("No categories.")
		}

		return err
	}

	tx, err := db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	if args[0] == "add" {
		if _, err := ensureCategory(tx, args[1]); err != nil {
			return err
		}
	} else {
		var id int64
		err := tx.QueryRow("SELECT id FROM categories WHERE name = ?", args[1]).Scan(&id)

		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("category %q not found", args[1])
		}

		if err != nil {
			return err
		}

		var exists bool

		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE name = ? AND id != ?)", args[2], id).Scan(&exists); err != nil {
			return err
		}

		if exists {
			return fmt.Errorf("category %q already exists", args[2])
		}

		if _, err := tx.Exec("UPDATE categories SET name = ? WHERE id = ?", args[2], id); err != nil {
			return err
		}
	}

	return tx.Commit()
}
