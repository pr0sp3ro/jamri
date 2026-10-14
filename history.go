package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"
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

type historyPause struct {
	start  int64
	end    sql.NullInt64
	reason string
}

type historyTask struct {
	id     int64
	name   string
	start  int64
	end    sql.NullInt64
	pauses []historyPause
}

func readHistory(db *sql.DB, from, until time.Time, now int64) ([]historyTask, error) {
	rows, err := db.Query(`
 SELECT t.id, t.name, t.started_at, t.ended_at,
        p.started_at, p.ended_at, p.reason
 FROM tasks t LEFT JOIN pauses p ON p.task_id = t.id
 WHERE t.started_at < ?
   AND (COALESCE(t.ended_at, ?) > ? OR t.started_at >= ?)
 ORDER BY t.started_at, t.id, p.started_at, p.id
 `, until.Unix(), now, from.Unix(), from.Unix())

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var tasks []historyTask

	for rows.Next() {
		var task historyTask
		var pauseStart, pauseEnd sql.NullInt64
		var reason sql.NullString

		if err := rows.Scan(&task.id, &task.name, &task.start, &task.end, &pauseStart, &pauseEnd, &reason); err != nil {
			return nil, err
		}

		if len(tasks) == 0 || tasks[len(tasks)-1].id != task.id {
			tasks = append(tasks, task)
		}

		if pauseStart.Valid {
			last := &tasks[len(tasks)-1]
			last.pauses = append(last.pauses, historyPause{pauseStart.Int64, pauseEnd, reason.String})
		}
	}

	return tasks, rows.Err()
}

type historyDay struct {
	lines  []string
	work   int64
	pauses int64
}

func printHistory(db *sql.DB, from, until time.Time, details bool) error {
	now := time.Now().Unix()
	tasks, err := readHistory(db, from, until, now)

	if err != nil {
		return err
	}

	days := make(map[string]*historyDay)

	for _, task := range tasks {
		end := now

		if task.end.Valid {
			end = task.end.Int64
		}

		end = min(max(end, task.start), until.Unix())
		start := max(task.start, from.Unix())

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

			for _, pause := range task.pauses {
				pauseEnd := now

				if pause.end.Valid {
					pauseEnd = pause.end.Int64
				} else {
					paused = true
				}

				pauseStart := max(start, pause.start)
				pauseEnd = min(segmentEnd, max(pause.start, pauseEnd))

				if pauseEnd <= pauseStart {
					continue
				}

				duration := pauseEnd - pauseStart
				pauseSeconds += duration

				if details {
					reason := pause.reason

					if reason == "" {
						reason = "(no reason)"
					}

					label := historyTime(pauseEnd, nextDay, from.Location())

					if !pause.end.Valid && pauseEnd == now {
						label = "ongoing"
					}

					pauseLines = append(pauseLines, fmt.Sprintf("  Pause  %s - %s  %s  %s\n",
						time.Unix(pauseStart, 0).In(from.Location()).Format("15:04:05"), label, formatDuration(duration), reason))
				}
			}

			work := max(int64(0), segmentEnd-start-pauseSeconds)
			label := historyTime(segmentEnd, nextDay, from.Location())

			if !task.end.Valid && segmentEnd == now && segmentEnd != nextDay {
				label = "active"

				if paused {
					label = "paused"
				}
			}

			line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\n", localStart.Format("15:04:05"), label,
				formatDuration(work), formatDuration(pauseSeconds), task.name)
			day.lines = append(day.lines, line)
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

		if _, err := fmt.Fprintln(writer, "START\tEND\tWORK\tPAUSES\tTASK"); err != nil {
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
