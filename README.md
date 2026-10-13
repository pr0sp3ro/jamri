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
tickd status
tickd history
tickd --help
```

`start` closes the active entry and creates a new one, even with the same name.
`stop` does nothing when no task is active. Both commands are silent on success.

`status` prints `dev  01:23:45` (hours, minutes, and seconds), or `idle`.
Configure your statusbar to run `tickd status` every second to display second updates.

Elapsed time includes sleep and shutdown until `stop` or another `start`.
The database is created automatically at `$XDG_DATA_HOME/tickd/tickd.sqlite3`,
defaulting to `~/.local/share/tickd/tickd.sqlite3`. Relative `XDG_DATA_HOME`
values are ignored. The `entries` table stores task names and Unix timestamps
in seconds. `ended_at = NULL` marks the active task. History can be displayed with `history`.

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

Dates use the local timezone. Each day with entries gets its own section:

```text
DATE
YYYY-MM-DD

START  END     DURATION  TASK
09:00  10:30   01:30     dev
10:45  active  00:25     documentation

Total: 01:55
```

Entries crossing midnight are split into daily portions for display only.
Times are clipped to each day, with `24:00` marking its end. Active entries
are counted up to now. Durations show full minutes. Daily totals are calculated
from seconds before rounding, so they may differ from the sum of displayed minutes.
Days without entries are omitted. An empty result prints `No entries.`.

## dmenu example

```sh
#!/usr/bin/env bash

set -euo pipefail

choice=$(printf 'start\nstop\n' | dmenu -i -p 'tickd:' "$@") || exit 0

case "$choice" in
  start)
      name=$(dmenu -p 'Task:' "$@" </dev/null) || exit 0
      [[ -n "$name" ]] || exit 0
      tickd start "$name"
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
