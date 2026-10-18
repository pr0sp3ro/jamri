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
  tickd start "task name" [--quality "text"] [--focus "text"]
  tickd stop [--quality "text"] [--focus "text"]
  tickd pause ["reason"]
  tickd resume
  tickd status
  tickd history [--details] [today|yesterday|week|month|YYYY-MM-DD [YYYY-MM-DD]]
  tickd edit ID [--name "text"] [--quality "text"] [--focus "text"]
  tickd help

Quality and focus describe the task being ended. Both are optional single-line
answers and accept any non-empty text. An active task is required when provided.

Edit updates completed tasks only and requires at least one option. Omitted fields
stay unchanged. Use --quality "" or --focus "" to clear an answer.

Examples:
  tickd stop --quality "good" --focus "high"
  tickd stop --focus "frequent interruptions"
  tickd start "next task" --quality "very good"
  tickd edit 42 --name "coding xyz" --quality "good"
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tickd:", err)
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
	case "start":
		name, quality, focus, _ := taskArguments(command, args[1:])

		return startTask(db, name, quality, focus)
	case "stop":
		_, quality, focus, _ := taskArguments(command, args[1:])

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
	case "pause":
		if len(args) > 2 {
			return "", errors.New("pause accepts one optional reason")
		}
	case "start", "stop":
		if _, _, _, err := taskArguments(command, args[1:]); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown command %q\n%s", command, usage)
	}

	return command, nil
}

func taskArguments(command string, args []string) (name, quality, focus string, err error) {
	nameSet := false

	for i := 0; i < len(args); i++ {
		argument := args[i]

		if argument == "--quality" || argument == "--focus" {
			answer := &quality

			if argument == "--focus" {
				answer = &focus
			}

			if *answer != "" {
				return "", "", "", fmt.Errorf("%s specified more than once", argument)
			}

			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", "", "", fmt.Errorf("%s requires a value", argument)
			}

			i++
			*answer = args[i]

			if strings.TrimSpace(*answer) == "" {
				return "", "", "", fmt.Errorf("%s requires non-empty text", argument)
			}

			for _, character := range *answer {
				if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
					return "", "", "", fmt.Errorf("%s must be a single line without control characters", argument)
				}
			}
		} else {
			if strings.HasPrefix(argument, "--") {
				return "", "", "", fmt.Errorf("unknown option %q", argument)
			}

			if command != "start" || nameSet {
				return "", "", "", fmt.Errorf("unexpected argument %q", argument)
			}

			name = argument
			nameSet = true
		}
	}

	if command == "start" {
		if strings.TrimSpace(name) == "" {
			return "", "", "", errors.New("start requires one non-empty task name")
		}

		for _, character := range name {
			if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
				return "", "", "", errors.New("task name must not contain control characters or line separators")
			}
		}
	}

	return name, quality, focus, nil
}
