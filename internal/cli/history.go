package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/pr0sp3ro/jamri/internal/store"
)

func historyRange(args []string, now time.Time) (time.Time, time.Time, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	period := "today"

	if len(args) > 0 {
		period = args[0]
	}

	if len(args) < 2 {
		switch period {
		case "today":
			return today, today.AddDate(0, 0, 1), nil
		case "yesterday":
			return today.AddDate(0, 0, -1), today, nil
		case "week":
			daysSinceMonday := (int(today.Weekday()) + 6) % 7
			from := today.AddDate(0, 0, -daysSinceMonday)

			return from, from.AddDate(0, 0, 7), nil
		case "month":
			from := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())

			return from, from.AddDate(0, 1, 0), nil
		}
	}

	from, err := time.ParseInLocation("2006-01-02", period, now.Location())

	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD or today, yesterday, week, month", period)
	}

	lastDay := from

	if len(args) == 2 {
		lastDay, err = time.ParseInLocation("2006-01-02", args[1], now.Location())

		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD", args[1])
		}
	}

	if lastDay.Before(from) {
		return time.Time{}, time.Time{}, errors.New("end date must not precede start date")
	}

	return from, lastDay.AddDate(0, 0, 1), nil
}

func historyArguments(args []string) ([]string, bool, error) {
	var dates []string
	details := false

	for _, argument := range args {
		if argument == "--details" {
			if details {
				return nil, false, errors.New("--details specified more than once")
			}

			details = true
		} else {
			dates = append(dates, argument)
		}
	}

	if len(dates) > 2 {
		return nil, false, errors.New("history accepts a period, a date, or two dates")
	}

	return dates, details, nil
}

type historyDay struct {
	lines  []string
	work   int64
	pauses int64
}

func printHistory(db *store.Store, from, until time.Time, details bool) error {
	now := time.Now().Unix()
	tasks, err := db.History(from, until, now)

	if err != nil {
		return err
	}

	days := make(map[string]*historyDay)

	for _, task := range tasks {
		end := now

		if task.End.Valid {
			end = task.End.Int64
		}

		end = min(max(end, task.Start), until.Unix())
		start := max(task.Start, from.Unix())

		if end < start || start >= until.Unix() {
			continue
		}

		for {
			localStart := time.Unix(start, 0).In(from.Location())
			dayStart := time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, from.Location())
			nextDay := dayStart.AddDate(0, 0, 1).Unix()
			segmentEnd := min(end, nextDay)
			date := dayStart.Format("2006-01-02")
			day := days[date]

			if day == nil {
				day = &historyDay{}
				days[date] = day
			}

			var pauseSeconds int64
			var pauseLines []string
			paused := false

			for _, pause := range task.Pauses {
				pauseEnd := now

				if pause.End.Valid {
					pauseEnd = pause.End.Int64
				} else {
					paused = true
				}

				pauseStart := max(start, pause.Start)
				pauseEnd = min(segmentEnd, max(pause.Start, pauseEnd))

				if pauseEnd <= pauseStart {
					continue
				}

				duration := pauseEnd - pauseStart
				pauseSeconds += duration

				if details {
					reason := pause.Reason

					if reason == "" {
						reason = "(no reason)"
					}

					label := historyTime(pauseEnd, nextDay, from.Location())

					if !pause.End.Valid && pauseEnd == now {
						label = "ongoing"
					}

					pauseLines = append(pauseLines, fmt.Sprintf("  Pause  %s - %s  %s  %s\n",
						time.Unix(pauseStart, 0).In(from.Location()).Format("15:04:05"), label, formatDuration(duration), reason))
				}
			}

			work := max(int64(0), segmentEnd-start-pauseSeconds)
			label := historyTime(segmentEnd, nextDay, from.Location())

			if !task.End.Valid && segmentEnd == now && segmentEnd != nextDay {
				label = "active"

				if paused {
					label = "paused"
				}
			}

			line := fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s\t%s\n", task.ID, localStart.Format("15:04:05"), label,
				formatDuration(work), formatDuration(pauseSeconds), task.Category, task.Name)
			day.lines = append(day.lines, line)

			if details {
				if task.Quality.Valid {
					day.lines = append(day.lines, fmt.Sprintf("  Quality: %s\n", task.Quality.String))
				}

				if task.Focus.Valid {
					day.lines = append(day.lines, fmt.Sprintf("  Focus: %s\n", task.Focus.String))
				}
			}

			day.lines = append(day.lines, pauseLines...)
			day.work += work
			day.pauses += pauseSeconds

			if segmentEnd >= end {
				break
			}

			start = segmentEnd
		}
	}

	if len(days) == 0 {
		_, err := fmt.Println("No entries.")

		return err
	}

	dates := make([]string, 0, len(days))

	for date := range days {
		dates = append(dates, date)
	}

	sort.Strings(dates)

	for _, date := range dates {
		day := days[date]

		dayDate, err := time.Parse("2006-01-02", date)

		if err != nil {
			return err
		}

		if _, err := fmt.Printf("DATE\n%s • %s\n\n", date, dayDate.Weekday()); err != nil {
			return err
		}

		writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		if _, err := fmt.Fprintln(writer, "ID\tSTART\tEND\tWORK\tPAUSES\tCATEGORY\tTASK"); err != nil {
			return err
		}

		for _, line := range day.lines {
			if _, err := fmt.Fprint(writer, line); err != nil {
				return err
			}
		}

		if err := writer.Flush(); err != nil {
			return err
		}

		if _, err := fmt.Printf("\nTotal work:   %s\nTotal pauses: %s\n\n", formatDuration(day.work), formatDuration(day.pauses)); err != nil {
			return err
		}
	}

	return nil
}

func historyTime(timestamp, nextDay int64, location *time.Location) string {
	if timestamp == nextDay {
		return "24:00:00"
	}

	return time.Unix(timestamp, 0).In(location).Format("15:04:05")
}

func formatDuration(seconds int64) string {
	seconds = max(int64(0), seconds)

	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
}
