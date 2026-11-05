package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pr0sp3ro/jamri/internal/store"
)

func addArguments(args []string, now time.Time) (store.CompletedTask, error) {
	var addition store.CompletedTask
	values := make(map[string]string)
	var taskArgs []string

	for i := 0; i < len(args); i++ {
		option := args[i]

		if !strings.HasPrefix(option, "--") {
			taskArgs = append(taskArgs, option)
			continue
		}

		switch option {
		case "--date", "--from", "--to", "--quality", "--focus":
		default:
			return addition, fmt.Errorf("unknown add option %q", option)
		}

		if _, exists := values[option]; exists {
			return addition, fmt.Errorf("%s specified more than once", option)
		}

		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return addition, fmt.Errorf("%s requires a value", option)
		}

		i++
		values[option] = args[i]

		if option == "--quality" || option == "--focus" {
			taskArgs = append(taskArgs, option, args[i])
		}
	}

	if values["--from"] == "" || values["--to"] == "" {
		return addition, errors.New("add requires --from HH:MM and --to HH:MM")
	}

	var err error
	addition.Name, addition.Category, addition.Quality, addition.Focus, err = taskArguments("start", taskArgs)

	if err != nil {
		return addition, fmt.Errorf("add: %w", err)
	}

	date, provided := values["--date"]

	if !provided {
		date = now.Format("2006-01-02")
	}

	const layout = "2006-01-02 15:04"

	for _, field := range []struct {
		option string
		target *time.Time
	}{{"--from", &addition.Start}, {"--to", &addition.End}} {
		input := date + " " + values[field.option]
		parsed, err := time.ParseInLocation(layout, input, now.Location())

		if err != nil || parsed.Format(layout) != input {
			return addition, fmt.Errorf("invalid date or time for %s: use --date YYYY-MM-DD and HH:MM in the local timezone", field.option)
		}

		*field.target = parsed
	}

	if !addition.End.After(addition.Start) {
		return addition, errors.New("end time must be after start time on the same day")
	}

	if addition.End.After(now) {
		return addition, errors.New("end time must not be in the future")
	}

	return addition, nil
}
