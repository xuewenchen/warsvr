#!/bin/bash
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
CONF="config.yml"
ID=""
# If first arg ends with .yml/.yaml it's the config, otherwise it's the ID
if [[ "$1" =~ \.ya?ml$ ]]; then
  CONF="$1"; ID="${2:-}"
else
  ID="${1:-}"
fi
[ -f "$CONF" ] || { echo "ERROR: config file not found: $CONF"; exit 1; }
echo ">>> Starting Gateway (conf=$CONF, id=${ID:-default})"
if [ -n "$ID" ]; then
  go run ./apps/gateway/cmd/ -conf "$CONF" -id "$ID"
else
  go run ./apps/gateway/cmd/ -conf "$CONF"
fi
