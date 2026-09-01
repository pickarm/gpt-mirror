#!/bin/sh
set -eu

runtime_dir="$(dirname "${BROWSER_SOCKET_PATH:-/run/gpt-mirror/browser.sock}")"
mkdir -p "$runtime_dir"
chown -R pwuser:pwuser "$runtime_dir"

command -v node >/dev/null
command -v Xvfb >/dev/null
id pwuser >/dev/null

# Keep the Unix-socket control plane independent from xvfb-run. Xvfb is only
# display infrastructure for headed Chromium and must not gate /health.
DISPLAY_NUM="${BROWSER_DISPLAY:-99}"
export DISPLAY=":${DISPLAY_NUM}"
rm -f "/tmp/.X${DISPLAY_NUM}-lock"

gosu pwuser Xvfb "$DISPLAY" -screen 0 1440x1000x24 -nolisten tcp >/tmp/gpt-mirror-xvfb.log 2>&1 &
xvfb_pid=$!

cleanup() {
  kill "$xvfb_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

for _ in $(seq 1 20); do
  if [ -S "/tmp/.X11-unix/X${DISPLAY_NUM}" ]; then
    break
  fi
  if ! kill -0 "$xvfb_pid" 2>/dev/null; then
    echo "browser-worker: Xvfb exited during startup" >&2
    cat /tmp/gpt-mirror-xvfb.log >&2 || true
    exit 1
  fi
  sleep 0.1
done

if [ ! -S "/tmp/.X11-unix/X${DISPLAY_NUM}" ]; then
  echo "browser-worker: Xvfb display socket did not become ready" >&2
  cat /tmp/gpt-mirror-xvfb.log >&2 || true
  exit 1
fi

echo "browser-worker: starting node control plane on ${BROWSER_SOCKET_PATH:-/run/gpt-mirror/browser.sock} display=$DISPLAY" >&2
exec gosu pwuser "$@"
