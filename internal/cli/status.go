package cli

import (
	"fmt"
	"os"

	"github.com/pr0sp3ro/jamri/internal/notify"
	"github.com/pr0sp3ro/jamri/internal/store"
)

func printStatus(db *store.Store) error {
	task, active, err := db.ActiveTask()

	if err != nil {
		return err
	}

	if !active {
		_, err := fmt.Println("idle")

		return err
	}

	// jamri runs no background process, so the periodic status call from
	// the status bar is what detects the reached limit.
	if task.PendingMaxSeconds > 0 && task.ElapsedSeconds >= task.PendingMaxSeconds {
		if err := alertMaxTime(db, task); err != nil {
			// The alert is best effort and must not hide the status line.
			fmt.Fprintln(os.Stderr, "jamri: max time alert:", err)
		}
	}

	name := task.Name

	if task.Paused {
		name += " (paused)"
	}

	_, err = fmt.Printf("%s  %s\n", name, formatDuration(task.ElapsedSeconds))

	return err
}

func alertMaxTime(db *store.Store, task store.ActiveTask) error {
	// The flag is set before alerting, so concurrent status calls alert once
	// and a failed alert is not repeated every second.
	notified, err := db.MarkMaxTimeNotified(task.ID)

	if err != nil || !notified {
		return err
	}

	return notify.Alarm(task.Name, "Max time reached: "+formatDuration(task.PendingMaxSeconds))
}
