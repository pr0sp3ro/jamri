package cli

import (
	"testing"
	"time"
)

func TestAddArguments(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")

	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 29, 18, 0, 0, 0, location)

	for _, date := range [][]string{nil, {"--date", "2026-10-29"}} {
		args := append([]string{"--focus", "high", "dev", "working on an app", "--from", "14:00", "--quality", "good", "--to", "15:30"}, date...)
		addition, err := addArguments(args, now)

		if err != nil {
			t.Fatal(err)
		}

		if addition.Name != "working on an app" || addition.Category != "dev" || addition.Focus != "high" || addition.Quality != "good" {
			t.Fatalf("unexpected task: %+v", addition)
		}

		wantStart := time.Date(2026, 10, 29, 14, 0, 0, 0, location)

		if !addition.Start.Equal(wantStart) || addition.End.Sub(addition.Start) != 90*time.Minute {
			t.Fatalf("unexpected range: %v - %v", addition.Start, addition.End)
		}
	}

	addition, err := addArguments([]string{"task", "--from", "14:00", "--to", "18:00"}, now)

	if err != nil {
		t.Fatal(err)
	}

	if addition.Category != "" || addition.Quality != "" || addition.Focus != "" || !addition.End.Equal(now) {
		t.Fatalf("unexpected defaults: %+v", addition)
	}

	invalid := []struct {
		name string
		args []string
	}{
		{"missing name", nil},
		{"extra positional", []string{"dev", "task", "extra"}},
		{"empty name", []string{" "}},
		{"invalid category", []string{"Dev", "task"}},
		{"control character", []string{"task\nname"}},
		{"unknown option", []string{"task", "--other", "value"}},
		{"duplicate time", []string{"task", "--from", "13:00"}},
		{"duplicate date", []string{"task", "--date", "2026-10-28", "--date", "2026-10-29"}},
		{"duplicate focus", []string{"task", "--focus", "high", "--focus", "low"}},
		{"missing value", []string{"task", "--focus"}},
		{"option as value", []string{"task", "--focus", "--quality", "good"}},
		{"empty focus", []string{"task", "--focus", ""}},
		{"blank quality", []string{"task", "--quality", " "}},
		{"multiline focus", []string{"task", "--focus", "high\nlow"}},
		{"invalid date", []string{"task", "--date", "2026-02-30"}},
		{"empty date", []string{"task", "--date", ""}},
		{"relative date", []string{"task", "--date", "yesterday"}},
	}

	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"--from", "14:00", "--to", "15:30"}, test.args...)

			if _, err := addArguments(args, now); err == nil {
				t.Fatal("expected invalid arguments to be rejected")
			}
		})
	}

	for _, test := range []struct {
		name string
		from string
		to   string
		date string
	}{
		{"missing start", "", "15:30", "2026-10-29"},
		{"missing end", "14:00", "", "2026-10-29"},
		{"short hour", "9:00", "15:30", "2026-10-29"},
		{"invalid hour", "24:00", "15:30", "2026-10-29"},
		{"invalid minute", "14:60", "15:30", "2026-10-29"},
		{"seconds", "14:00:00", "15:30", "2026-10-29"},
		{"equal times", "14:00", "14:00", "2026-10-29"},
		{"overnight", "23:00", "01:00", "2026-10-28"},
		{"future time", "17:00", "18:01", "2026-10-29"},
		{"future date", "14:00", "15:30", "2026-10-30"},
		{"nonexistent DST time", "02:30", "04:00", "2026-03-29"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := []string{"task", "--date", test.date}

			if test.from != "" {
				args = append(args, "--from", test.from)
			}

			if test.to != "" {
				args = append(args, "--to", test.to)
			}

			if _, err := addArguments(args, now); err == nil {
				t.Fatal("expected invalid time range to be rejected")
			}
		})
	}
}
