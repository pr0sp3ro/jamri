package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/pr0sp3ro/jamri/internal/store"
)

func editArguments(args []string) (store.TaskEdit, error) {
	var edit store.TaskEdit

	if len(args) == 0 {
		return edit, errors.New("edit requires a positive task ID and at least one option")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)

	if err != nil || id <= 0 {
		return edit, errors.New("edit requires a positive task ID")
	}

	edit.ID = id

	for i := 1; i < len(args); i++ {
		option := args[i]
		var field **string

		switch option {
		case "--name":
			field = &edit.Name
		case "--quality":
			field = &edit.Quality
		case "--focus":
			field = &edit.Focus
		case "--category":
			field = &edit.Category
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

		if option == "--category" && value != "" {
			if err := validateCategoryName(value); err != nil {
				return edit, err
			}
		}

		*field = &value
	}

	if edit.Name == nil && edit.Quality == nil && edit.Focus == nil && edit.Category == nil {
		return edit, errors.New("edit requires at least one of --name, --quality, --focus, --category")
	}

	return edit, nil
}
