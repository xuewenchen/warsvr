#!/bin/bash
set -e
ROOT=$(cd "$(dirname "$0")/.." && pwd)
BIN="$ROOT/bin"
PID_DIR="$BIN/.pids"

CMD="${1:-}"
SVC="${2:-all}"
CONF="${3:-config.yml}"
ID="${4:-}"

mkdir -p "$BIN" "$PID_DIR"

build_one() { echo ">>> Building $1..."; cd "$ROOT"; go build -o "$BIN/$1" "./apps/$1/cmd/"; echo "  -> $BIN/$1"; }
pid_file() { echo "$PID_DIR/${1}.pid"; }
is_running() { [ -f "$(pid_file "$1")" ] && kill -0 "$(cat "$(pid_file "$1")")" 2>/dev/null; }

do_build() {
  case "$1" in
    chatsvr|gateway) build_one "$1" ;;
    all) build_one chatsvr; build_one gateway; echo "Done." ;;
    *) echo "Usage: $0 build <chatsvr|gateway|all>" ;;
  esac
}

do_start() {
  case "$1" in
    chatsvr|gateway)
      if is_running "$1"; then echo "$1 is already running"; return; fi
      if [ ! -f "$BIN/$1" ]; then build_one "$1"; fi
      echo ">>> Starting $1 (conf=$CONF, id=${ID:-default})..."
      local LOG="$ROOT/log/$1.log"
      mkdir -p "$ROOT/log"
      if [ -n "$ID" ]; then
        nohup "$BIN/$1" -conf "$CONF" -id "$ID" >> "$LOG" 2>&1 &
      else
        nohup "$BIN/$1" -conf "$CONF" >> "$LOG" 2>&1 &
      fi
      echo $! > "$(pid_file "$1")"
      echo "  PID: $(cat "$(pid_file "$1")")  log: $LOG"
      ;;
    all)
      do_start chatsvr; sleep 1; do_start gateway
      ;;
    *) echo "Usage: $0 start <chatsvr|gateway|all> [config] [id]" ;;
  esac
}

do_stop() {
  case "$1" in
    chatsvr|gateway)
      if is_running "$1"; then
        echo ">>> Stopping $1 (PID: $(cat "$(pid_file "$1")"))..."
        kill "$(cat "$(pid_file "$1")")" 2>/dev/null
        sleep 0.5
        rm -f "$(pid_file "$1")"
      else
        echo "$1 is not running"
      fi
      ;;
    all)
      do_stop chatsvr; do_stop gateway
      ;;
    *) echo "Usage: $0 stop <chatsvr|gateway|all>" ;;
  esac
}

do_restart() { do_stop "$1"; sleep 1; do_start "$1"; }
do_reboot()  { do_stop "$1"; do_build "$1"; do_start "$1"; }

case "$CMD" in
  build)   do_build "$SVC" ;;
  start)   do_start "$SVC" ;;
  stop)    do_stop "$SVC" ;;
  restart) do_restart "$SVC" ;;
  reboot)  do_reboot "$SVC" ;;
  *)
    echo "Usage: $0 <build|start|stop|restart|reboot> <chatsvr|gateway|all> [config] [id]"
    echo ""
    echo "Commands:"
    echo "  build xxx    - compile binary"
    echo "  start xxx    - run binary (auto-builds if missing)"
    echo "  stop xxx     - kill process"
    echo "  restart xxx  - stop + start"
    echo "  reboot xxx   - stop + build + start"
    echo ""
    echo "Targets: chatsvr | gateway | all"
    echo "Config:  path to config yml (default: config.yml)"
    echo "ID:      instance ID in config services array (default: first entry)"
    echo ""
    echo "Examples:"
    echo "  $0 build all"
    echo "  $0 start chatsvr"
    echo "  $0 start gateway prod.yml"
    echo "  $0 start gateway prod.yml gw-1"
    echo "  $0 restart chatsvr prod.yml cs-2"
    echo "  $0 reboot all prod.yml"
    ;;
esac
