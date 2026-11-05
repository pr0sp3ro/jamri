package cli

import (
	"errors"
	"fmt"

	"github.com/pr0sp3ro/jamri/internal/store"
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

func manageCategory(db *store.Store, args []string) error {
	switch args[0] {
	case "add":
		return db.AddCategory(args[1])
	case "rename":
		return db.RenameCategory(args[1], args[2])
	}

	names, err := db.Categories()

	if err != nil {
		return err
	}

	if len(names) == 0 {
		_, err := fmt.Println("No categories.")

		return err
	}

	for _, name := range names {
		if _, err := fmt.Println(name); err != nil {
			return err
		}
	}

	return nil
}
