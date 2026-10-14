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

START     END       WORK      PAUSES    TASK
09:00:00  12:00:00  02:15:00  00:45:00  coding xyz

Total work:   02:15:00
Total pauses: 00:45:00
```

Use `--details` with any date filter to show individual pauses:

```sh
tickd history --details
tickd history week --details
```

```text
START     END       WORK      PAUSES    TASK
09:00:00  12:00:00  02:15:00  00:45:00  coding xyz
  Pause  10:00:00 - 10:30:00  00:30:00  lunch
  Pause  11:00:00 - 11:15:00  00:15:00  (no reason)
```

An open task shows `active` or `paused` in END. An open pause shows `ongoing`
in the details. Separate `start` commands create separate tasks, even with the
same name.

Tasks and pauses crossing midnight are clipped to each day for display only,
with `24:00:00` marking its end. Open intervals are counted up to now. Times,
durations, and daily totals include seconds. Days without tasks are omitted. An empty result
prints `No entries.`.

## dmenu example

```sh
#!/usr/bin/env bash

set -euo pipefail

choice=$(printf 'start\npause\nresume\nstop\n' | dmenu -i -p 'tickd:' "$@") || exit 0

case "$choice" in
  start)
      name=$(dmenu -p 'Task:' "$@" </dev/null) || exit 0
      [[ -n "$name" ]] || exit 0
      tickd start "$name"
      ;;
  pause)
      reason=$(dmenu -p 'Pause reason (optional):' "$@" </dev/null) || exit 0
      tickd pause "$reason"
      ;;
  resume)
      tickd resume
      ;;
  stop)
      tickd stop
      notify-send 'tickd' 'Task stopped.'
      ;;
esac
```

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
