package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/pr0sp3ro/jamri/internal/store"
)

func pause(db *store.Store, args []string) error {
	taskID, err := db.TaskToPause()

	if err != nil {
		return err
	}

	reason := ""

	if len(args) == 1 {
		reason = args[0]
	} else {
		if _, err := fmt.Fprint(os.Stderr, "Pause reason (optional): "); err != nil {
			return err
		}

		line, err := bufio.NewReader(os.Stdin).ReadString('\n')

		if err != nil {
			return fmt.Errorf("pause cancelled: %w", err)
		}

		reason = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}

	for _, character := range reason {
		if unicode.IsControl(character) || character == ' ' || character == ' ' {
			return errors.New("pause reason must be a single line without control characters")
		}
	}

	return db.Pause(taskID, reason)
}
