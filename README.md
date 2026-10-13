# tickd

Go timer with SQLite history.

## Installation

Requires Go and a C compiler (CGO).

```sh
CGO_ENABLED=1 go build -o tickd .
install -m 755 tickd ~/.local/bin/tickd
```

Make sure `~/.local/bin` is in your `PATH`.

## Usage

```sh
tickd start "dev"
tickd stop
tickd status
tickd --help
```

`start` closes the active entry and creates a new one, even with the same name.
`stop` does nothing when no task is active. Both commands are silent on success.

`status` prints `dev 01:23` (hours and full minutes), or `idle`.
Configure your statusbar to run `tickd status` periodically, for example every minute.

Elapsed time includes sleep and shutdown until `stop` or another `start`.
The database is created automatically at `$XDG_DATA_HOME/tickd/tickd.sqlite3`,
defaulting to `~/.local/share/tickd/tickd.sqlite3`. Relative `XDG_DATA_HOME`
values are ignored. The `entries` table stores task names and Unix timestamps
in seconds. `ended_at = NULL` marks the active task. History stays in SQLite,
with no reporting command yet.

## dmenu example

```sh
name=$(printf '\n' | dmenu -p 'Task:') || exit
[ -n "$name" ] || exit
tickd start "$name"
```
