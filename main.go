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
  tickd start "task name"
  tickd stop
  tickd pause ["reason"]
  tickd resume
  tickd status
  tickd history [--details] [today|yesterday|week|month|YYYY-MM-DD [YYYY-MM-DD]]
  tickd help
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
	case "start":
		return startTask(db, args[1])
	case "stop":
		return stopTask(db)
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
	case "help", "--help", "stop", "status", "resume":
		if len(args) != 1 {
			return "", fmt.Errorf("%s does not accept arguments", command)
		}
	case "history":
		if _, _, err := historyArguments(args[1:]); err != nil {
			return "", err
		}
	case "pause":
		if len(args) > 2 {
			return "", errors.New("pause accepts one optional reason")
		}
	case "start":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return "", errors.New("start requires one non-empty task name")
		}

		for _, character := range args[1] {
			if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
				return "", errors.New("task name must not contain control characters or line separators")
			}
		}
	default:
		return "", fmt.Errorf("unknown command %q\n%s", command, usage)
	}

	return command, nil
}
