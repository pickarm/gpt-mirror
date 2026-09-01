#!/bin/sh
set -eu

runtime_dir="$(dirname "${BROWSER_SOCKET_PATH:-/run/gpt-mirror/browser.sock}")"
mkdir -p "$runtime_dir"
chown -R pwuser:pwuser "$runtime_dir"

exec gosu pwuser "$@"
