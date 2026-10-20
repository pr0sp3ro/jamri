#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

if [ ! -x "$HOME/.local/bin/jamri" ]; then
    printf 'Install jamri first with ./install.sh\n' >&2
    exit 1
fi

unit_dir="$HOME/.config/systemd/user"
mkdir -p "$unit_dir/jamri-stop.service.d"
install -m 644 "$project_dir/jamri-stop.service" "$unit_dir/jamri-stop.service"

# Preserve the CLI database location, including a custom XDG_DATA_HOME.
data_home=${XDG_DATA_HOME:-"$HOME/.local/share"}
case "$data_home" in
    /*) ;;
    *) data_home="$HOME/.local/share" ;;
esac

# Escape special characters used in systemd Environment= values.
escaped_data_home=$(printf '%s' "$data_home" | sed 's/\\/\\\\/g; s/"/\\"/g; s/%/%%/g')
printf '[Service]\nEnvironment="XDG_DATA_HOME=%s"\n' "$escaped_data_home" \
    > "$unit_dir/jamri-stop.service.d/data-home.conf"

systemctl --user daemon-reload
systemctl --user enable --now jamri-stop.service
printf 'Enabled automatic task stop at shutdown.\n'
