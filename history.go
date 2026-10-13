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

func printHistory(db *sql.DB, from, until time.Time) error {
	now := time.Now().Unix()
	rows, err := db.Query(`
		SELECT name, started_at, ended_at
		FROM entries
		WHERE started_at < ?
		  AND (COALESCE(ended_at, ?) > ? OR started_at >= ?)
		ORDER BY started_at, id
	`, until.Unix(), now, from.Unix(), from.Unix())

	if err != nil {
		return err
	}

	defer rows.Close()
	// Buffer day sections because overlapping entries can span multiple days.
	sections := make(map[string][]string)
	totals := make(map[string]int64)

	for rows.Next() {
		var name string
		var startedAt int64
		var endedAt sql.NullInt64

		if err := rows.Scan(&name, &startedAt, &endedAt); err != nil {
			return err
		}

		end := now

		if endedAt.Valid {
			end = endedAt.Int64
		}

		// A backwards clock must not produce a negative duration.
		end = max(end, startedAt)
		start := max(startedAt, from.Unix())
		end = min(end, until.Unix())

		if start >= until.Unix() || end < start {
			continue
		}

		for {
			localStart := time.Unix(start, 0).In(from.Location())
			day := time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, from.Location())
			nextDay := day.AddDate(0, 0, 1)
			segmentEnd := min(end, nextDay.Unix())
			date := day.Format("2006-01-02")
			endLabel := time.Unix(segmentEnd, 0).In(from.Location()).Format("15:04")

			if segmentEnd == nextDay.Unix() {
				endLabel = "24:00"
			} else if !endedAt.Valid && segmentEnd == now {
				endLabel = "active"
			}

			duration := segmentEnd - start
			line := fmt.Sprintf("%s\t%s\t%s\t%s\n", localStart.Format("15:04"), endLabel, formatDuration(duration), name)
			sections[date] = append(sections[date], line)
			totals[date] += duration

			if segmentEnd >= end {
				break
			}

			start = segmentEnd
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if len(sections) == 0 {
		_, err := fmt.Println("No entries.")
		return err
	}

	dates := make([]string, 0, len(sections))

	for date := range sections {
		dates = append(dates, date)
	}

	sort.Strings(dates)

	for _, date := range dates {
		lines := sections[date]

		if _, err := fmt.Printf("DATE\n%s\n\n", date); err != nil {
			return err
		}

		writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		if _, err := fmt.Fprintln(writer, "START\tEND\tDURATION\tTASK"); err != nil {
			return err
		}

		for _, line := range lines {
			if _, err := fmt.Fprint(writer, line); err != nil {
				return err
			}
		}

		if err := writer.Flush(); err != nil {
			return err
		}

		if _, err := fmt.Printf("\nTotal: %s\n\n", formatDuration(totals[date])); err != nil {
			return err
		}
	}

	return nil
}

func formatDuration(seconds int64) string {
	minutes := max(int64(0), seconds) / 60
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}
