#!/bin/sh
set -eu

config_dir="${APP_CONF:-/app/data/}"
config_dir="${config_dir%/}"
mkdir -p "$config_dir"

if [ ! -f "$config_dir/config.json" ]; then
  cp /app/config.default.json "$config_dir/config.json"
fi

if [ "$(id -u)" = "0" ]; then
  chown -R gptmirror:gptmirror "$config_dir"
  exec su-exec gptmirror "$@"
fi

exec "$@"
