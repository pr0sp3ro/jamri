#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT
trap 'exit 1' HUP INT TERM

cd "$project_dir"
CGO_ENABLED=1 go build -o "$build_dir/jamri" .

mkdir -p "$HOME/.local/bin"
install -m 755 "$build_dir/jamri" "$HOME/.local/bin/jamri"
printf 'Installed jamri to %s/.local/bin/jamri\n' "$HOME"
