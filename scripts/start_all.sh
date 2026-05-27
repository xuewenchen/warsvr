#!/bin/bash
ROOT=$(cd "$(dirname "$0")/.." && pwd)
CONF="${1:-$ROOT/config.yml}"
[ -f "$CONF" ] || { echo "ERROR: config file not found: $CONF"; exit 1; }

echo "=== Starting all services ==="
echo "Config: $CONF"

cd "$ROOT"
go run ./apps/chatsvr/cmd/ -conf "$CONF" &
CS_PID=$!
echo "ChatSvr PID: $CS_PID"

sleep 2

go run ./apps/gateway/cmd/ -conf "$CONF" &
GW_PID=$!
echo "Gateway PID: $GW_PID"

echo "=== Both services running ==="
echo "ChatSvr PID: $CS_PID"
echo "Gateway PID: $GW_PID"
echo "Press Ctrl+C to stop"

trap "kill $CS_PID $GW_PID 2>/dev/null; echo 'Stopped.'" EXIT
wait
