package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
)

const usage = `Usage:
  jamri start [category] "task name" [--quality "text"] [--focus "text"]
  jamri stop [--quality "text"] [--focus "text"]
  jamri pause ["reason"]
  jamri resume
  jamri status
  jamri history [--details] [today|yesterday|week|month|YYYY-MM-DD [YYYY-MM-DD]]
  jamri edit ID [--name "text"] [--quality "text"] [--focus "text"] [--category NAME]
  jamri category list
  jamri category add NAME
  jamri category rename OLD NEW
  jamri help

With start and stop, quality and focus describe the task being ended. Both accept
optional non-empty single-line text. An active task is required when provided.

Edit updates completed tasks only and requires at least one option. Omitted fields
stay unchanged. Use --quality "" or --focus "" to clear an answer.
Use --category "" to remove a category. Category names must start with a-z
and contain only a-z, 0-9, and hyphens.
Unknown categories are created automatically by start and edit.

Examples:
  jamri stop --quality "good" --focus "high"
  jamri stop --focus "frequent interruptions"
  jamri start "next task" --quality "very good"
  jamri start dev "working on an app"
  jamri edit 42 --name "coding xyz" --quality "good"
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "jamri:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command, err := validateArguments(args)

	if err != nil {
		return err
	}

	if command == "help" || command == "--help" {
		_, err := fmt.Print(usage)

		return err
	}

	var from, until time.Time
	var details bool

	if command == "history" {
		var dates []string
		dates, details, err = historyArguments(args[1:])

		if err == nil {
			from, until, err = historyRange(dates, time.Now())
		}

		if err != nil {
			return err
		}
	}

	db, err := openDatabase()

	if err != nil {
		return err
	}

	defer db.Close()

	switch command {
	case "edit":
		edit, _ := editArguments(args[1:])

		return editTask(db, edit)
	case "category":
		return manageCategory(db, args[1:])
	case "start":
		name, category, quality, focus, _ := taskArguments(command, args[1:])

		return startTask(db, name, category, quality, focus)
	case "stop":
		_, _, quality, focus, _ := taskArguments(command, args[1:])

		return stopTask(db, quality, focus)
	case "pause":
		return pauseTask(db, args[1:])
	case "resume":
		return resumeTask(db)
	case "history":
		return printHistory(db, from, until, details)
	case "status":
		line, err := readStatus(db)

		if err != nil {
			return err
		}

		_, err = fmt.Println(line)

		return err
	}

	return nil
}

func validateArguments(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New(usage)
	}

	command := args[0]

	switch command {
	case "help", "--help", "status", "resume":
		if len(args) != 1 {
			return "", fmt.Errorf("%s does not accept arguments", command)
		}
	case "history":
		if _, _, err := historyArguments(args[1:]); err != nil {
			return "", err
		}
	case "edit":
		if _, err := editArguments(args[1:]); err != nil {
			return "", err
		}
	case "category":
		if err := categoryArguments(args[1:]); err != nil {
			return "", err
		}
	case "pause":
		if len(args) > 2 {
			return "", errors.New("pause accepts one optional reason")
		}
	case "start", "stop":
		if _, _, _, _, err := taskArguments(command, args[1:]); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown command %q\n%s", command, usage)
	}

	return command, nil
}

func taskArguments(command string, args []string) (name, category, quality, focus string, err error) {
	var positional []string

	for i := 0; i < len(args); i++ {
		argument := args[i]

		if argument == "--quality" || argument == "--focus" {
			answer := &quality

			if argument == "--focus" {
				answer = &focus
			}

			if *answer != "" {
				return "", "", "", "", fmt.Errorf("%s specified more than once", argument)
			}

			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", "", "", "", fmt.Errorf("%s requires a value", argument)
			}

			i++
			*answer = args[i]

			if strings.TrimSpace(*answer) == "" {
				return "", "", "", "", fmt.Errorf("%s requires non-empty text", argument)
			}

			for _, character := range *answer {
				if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
					return "", "", "", "", fmt.Errorf("%s must be a single line without control characters", argument)
				}
			}
		} else {
			if strings.HasPrefix(argument, "--") {
				return "", "", "", "", fmt.Errorf("unknown option %q", argument)
			}

			if command != "start" || len(positional) == 2 {
				return "", "", "", "", fmt.Errorf("unexpected argument %q", argument)
			}

			positional = append(positional, argument)
		}
	}

	if command == "start" {
		if len(positional) > 0 {
			name = positional[len(positional)-1]
		}

		if len(positional) == 2 {
			category = positional[0]

			if err := validateCategoryName(category); err != nil {
				return "", "", "", "", err
			}
		}

		if strings.TrimSpace(name) == "" {
			return "", "", "", "", errors.New("start requires one non-empty task name")
		}

		for _, character := range name {
			if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
				return "", "", "", "", errors.New("task name must not contain control characters or line separators")
			}
		}
	}

	return name, category, quality, focus, nil
}
