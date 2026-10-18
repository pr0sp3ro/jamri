# tickd

Go timer with SQLite history.

## Installation

Requires Go and a C compiler (CGO).

```sh
./install.sh
```

Make sure `~/.local/bin` is in your `PATH`.
The script rebuilds tickd and installs it to `~/.local/bin`.

## Usage

```sh
tickd start "dev"
tickd stop
tickd pause
tickd resume
tickd status
tickd history
tickd --help
```

`start` closes the active entry and creates a new one, even with the same name.
`stop` does nothing when no task is active. Both commands are silent on success.

Optionally describe the task being ended with `--quality` and `--focus`:

```sh
tickd stop --quality "good" --focus "high"
tickd stop --focus "frequent interruptions"
tickd start "next task" --quality "very good"
```

Each option accepts any non-empty single-line text. Either option can be omitted,
and options may appear before or after the task name. No questions are asked in
the terminal. With `start`, answers belong to the previous task, including a paused
task. Providing answers when no task is active returns an error without starting
a new task. Missing values, repeated options, and control characters are rejected.

`pause` asks for an optional reason. Press Enter to leave it blank, or provide
it directly with `tickd pause "lunch"`. EOF or interruption cancels the prompt.
`resume` resumes a paused task. Pausing requires a running task, and resuming
requires a paused task. Both commands are silent on success, apart from the prompt.

While paused, `status` prints `dev (paused)  01:23:45` with a frozen work duration.
Breaks do not count toward work time. `stop` ends the task and any open pause.
`start` replaces the current task, including a paused task, with a new one.

`status` prints `dev  01:23:45` (hours, minutes, and seconds), or `idle`.
Configure your statusbar to run `tickd status` every second to display second updates.

Elapsed time includes sleep and shutdown until the task is stopped.
The optional shutdown service can stop tasks automatically on normal shutdown.

## Storage

The SQLite database is created automatically at
`$XDG_DATA_HOME/tickd/tickd.sqlite3`, defaulting to
`~/.local/share/tickd/tickd.sqlite3`. Relative `XDG_DATA_HOME` values are ignored.
Existing databases are upgraded automatically, preserving task and pause history.
Older tasks have no quality or focus answers and no category.

## Optional shutdown stop

Requires a systemd user manager. Enable automatic task stopping with:

```sh
./enable-shutdown-stop.sh
```

The service runs no background process. It stops the active task when the user
manager shuts down, including normal shutdown, reboot, and usually logout.
Sudden power loss and forced shutdown are not covered.

The service uses the database location selected by `XDG_DATA_HOME` when enabled.

Disable with:

```sh
systemctl --user disable --now tickd-stop.service
```

Disabling the running service also stops the active task.

## Categories

Tasks can optionally belong to one category. Put the category before the task
name when starting a task:

```sh
tickd start dev "programming aris one app"
tickd start free-time "reading"
tickd start "task without category"
```

One positional argument is the task name. Two positional arguments are the
category and the task name. Quote task names containing spaces. Category names
must start with a lowercase ASCII letter (`a-z`) and contain only lowercase ASCII
letters (`a-z`), digits (`0-9`), and hyphens (`-`). Unknown categories are created automatically.
With `start`, the category belongs to the new task, while `--quality` and
`--focus` still describe the previous task.

Manage the saved categories with:

```sh
tickd category list
tickd category add eating
tickd category rename dev coding
```

The list is sorted alphabetically, with one name per line. An empty list prints
`No categories.`. Adding an existing name does nothing. Renaming updates the
category name for all assigned tasks, including historical entries. The old name
must exist and the new name must be available. Renaming a category to its own
name does nothing. Add and rename are silent on success.

History shows a `CATEGORY` column. Tasks without a category show `-`.

## History

```sh
tickd history                        # today
tickd history today
tickd history yesterday
tickd history week                   # current week, starting Monday
tickd history month                  # current calendar month
tickd history 2026-10-10
tickd history 2026-10-01 2026-10-10  # inclusive date range
```

Dates use the local timezone. Each day with tasks gets its own section.
Each task appears once per day, regardless of the number of pauses:

```text
DATE
2026-10-14 • Wednesday

ID  START     END       WORK      PAUSES    CATEGORY  TASK
42  09:00:00  12:00:00  02:15:00  00:45:00  dev       coding xyz

Total work:   02:15:00
Total pauses: 00:45:00
```

Use `--details` with any date filter to show quality and focus answers and individual pauses:

```sh
tickd history --details
tickd history week --details
```

```text
ID  START     END       WORK      PAUSES    CATEGORY  TASK
42  09:00:00  12:00:00  02:15:00  00:45:00  dev       coding xyz
  Quality: good
  Focus: high
  Pause  10:00:00 - 10:30:00  00:30:00  lunch
  Pause  11:00:00 - 11:15:00  00:15:00  (no reason)
```

Quality and focus appear only with `--details`. Unanswered fields are omitted.
For tasks spanning multiple days, answers appear under each daily task entry.

An open task shows `active` or `paused` in END. An open pause shows `ongoing`
in the details. Separate `start` commands create separate tasks, even with the
same name.

Tasks and pauses crossing midnight are clipped to each day for display only,
with `24:00:00` marking its end. Open intervals are counted up to now. Times,
durations, and daily totals include seconds. Days without tasks are omitted. An empty result
prints `No entries.`.

### Editing entries

Use the ID shown in history to edit a completed task:

```sh
tickd edit 42 --name "coding xyz"
tickd edit 42 --quality "good" --focus "high"
tickd edit 42 --focus ""
tickd edit 42 --category dev
tickd edit 42 --category ""
```

Provide at least one of `--name`, `--quality`, `--focus`, or `--category`. Omitted fields keep
their current values. An empty quality or focus value clears that answer.
An empty category removes the assignment. A new category name creates that
category automatically. Names
must be non-empty, and all values must be single-line text without control
characters. Whitespace-only values, repeated options, and missing values are
rejected.

Only completed tasks can be edited. Unknown IDs return an error. Editing does
not change task times or pauses and is silent on success. Tasks spanning multiple
days have the same ID on each day. Editing updates the entire task.

## dmenu example

```sh
#!/usr/bin/env bash

set -euo pipefail

choice=$(printf 'start\npause\nresume\nstop\n' | dmenu -i -p 'tickd:' "$@") || exit 0

case "$choice" in
  start|stop)
      args=("$choice")

      if [[ "$choice" == start ]]; then
          name=$(dmenu -p 'Task:' "$@" </dev/null) || exit 0
          [[ -n "$name" ]] || exit 0
          args+=("$name")
      fi

      status=$(tickd status)

      if [[ "$status" != idle ]]; then
          quality=$(printf '%s\n' 'Skip' 'very poor' 'poor' 'average' 'good' 'very good' |
              dmenu -i -p 'Work quality (optional):' "$@") || exit 0
          focus=$(printf '%s\n' 'Skip' 'none' 'low' 'moderate' 'high' 'full' |
              dmenu -i -p 'Focus (optional):' "$@") || exit 0

          if [[ -n "$quality" && "$quality" != Skip ]]; then
              args+=(--quality "$quality")
          fi

          if [[ -n "$focus" && "$focus" != Skip ]]; then
              args+=(--focus "$focus")
          fi
      fi

      tickd "${args[@]}"

      if [[ "$choice" == stop ]]; then
          notify-send 'tickd' 'Task stopped.'
      fi
      ;;
  pause)
      reason=$(dmenu -p 'Pause reason (optional):' "$@" </dev/null) || exit 0
      tickd pause "$reason"
      ;;
  resume)
      tickd resume
      ;;
esac
```

The quality and focus menus offer five suggestions and accept custom text.
Choose `Skip` or submit an empty answer to omit a field. Esc cancels the entire
operation before any task is stopped or started. These menus appear only when
there is an active task to end. Avoid changing tasks in another terminal while
answering the menus.

Then you can bind it to a key in your window manager, for example in i3/sway:

```text
bindsym $mod+t exec --no-startup-id "dm-tickd"
```

## i3status example

```sh
#!/bin/sh

CONFIG="$HOME/.config/i3status/config"

i3status --config "$CONFIG" | {
  read -r line && printf '%s\n' "$line"
  read -r line && printf '%s\n' "$line"

  while read -r line; do
      prefix=""
      case "$line" in
          ,*)
              prefix=","
              line=${line#,}
              ;;
      esac

      tickd_status=$(tickd status 2>/dev/null) || tickd_status="unavailable"

      case "$line" in
          ""|"[" )
              printf '%s%s\n' "$prefix" "$line"
              continue
              ;;
      esac

      line=$(printf '%s\n' "$line" | jq -c --arg tickd "$tickd_status" \
          '[{name: "tickd", full_text: $tickd}] + .')
      printf '%s%s\n' "$prefix" "$line"
  done
}
```
