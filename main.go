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
  jamri start [category] "task name" [--max-time DURATION] [--quality "text"] [--focus "text"]
  jamri add [category] "task name" --from HH:MM --to HH:MM [--date YYYY-MM-DD] [--quality "text"] [--focus "text"]
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

With start, --max-time sets a work time limit for the new task, for example 45m
or 1h30m. Pauses do not count. Once the limit is reached, jamri status shows
a notification and plays a short sound, once per task. The alert requires
jamri status to run periodically, for example from a status bar.

Add records a completed task without pauses. Quality and focus describe that task.
The date defaults to today in the local timezone. Both times must be on the same
day, with the end after the start and not in the future. Overlaps with existing
tasks, including active or paused tasks, are rejected. Adjacent ranges are allowed.
Add does not change the active timer and is silent on success.

Edit updates completed tasks only and requires at least one option. Omitted fields
stay unchanged. Use --quality "" or --focus "" to clear an answer.
Use --category "" to remove a category. Category names must start with a-z
and contain only a-z, 0-9, and hyphens.
Unknown categories are created automatically by start, add, and edit.

Examples:
  jamri stop --quality "good" --focus "high"
  jamri stop --focus "frequent interruptions"
  jamri start "next task" --quality "very good"
  jamri start dev "working on an app"
  jamri start dev "working on an app" --max-time 45m
  jamri add dev "working on an app" --date 2026-10-29 --from 14:00 --to 15:30 --focus high
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
	var addition taskAddition

	if command == "add" {
		addition, err = addArguments(args[1:], time.Now())

		if err != nil {
			return err
		}
	}

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
	case "add":
		return addTask(db, addition)
	case "edit":
		edit, _ := editArguments(args[1:])

		return editTask(db, edit)
	case "category":
		return manageCategory(db, args[1:])
	case "start":
		maxSeconds, taskArgs, _ := maxTimeArgument(args[1:])
		name, category, quality, focus, _ := taskArguments(command, taskArgs)

		return startTask(db, name, category, quality, focus, maxSeconds)
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
	case "add":
		if _, err := addArguments(args[1:], time.Now()); err != nil {
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
		taskArgs := args[1:]

		if command == "start" {
			_, rest, err := maxTimeArgument(taskArgs)

			if err != nil {
				return "", err
			}

			taskArgs = rest
		}

		if _, _, _, _, err := taskArguments(command, taskArgs); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown command %q\n%s", command, usage)
	}

	return command, nil
}

// maxTimeArgument extracts --max-time from start arguments and returns the
// remaining arguments for taskArguments, which is shared with add.
func maxTimeArgument(args []string) (int64, []string, error) {
	var maxSeconds int64
	var rest []string

	for i := 0; i < len(args); i++ {
		if args[i] != "--max-time" {
			rest = append(rest, args[i])
			continue
		}

		if maxSeconds != 0 {
			return 0, nil, errors.New("--max-time specified more than once")
		}

		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return 0, nil, errors.New("--max-time requires a value")
		}

		i++
		duration, err := time.ParseDuration(args[i])

		if err != nil || duration <= 0 || duration%time.Second != 0 {
			return 0, nil, errors.New("--max-time requires a positive duration in whole seconds, for example 45m or 1h30m")
		}

		maxSeconds = int64(duration / time.Second)
	}

	return maxSeconds, rest, nil
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
