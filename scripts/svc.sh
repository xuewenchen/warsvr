#!/bin/bash
ROOT=$(cd "$(dirname "$0")/.." && pwd)
if [ ! -f "$ROOT/bin/svchelper" ]; then
  echo "Building svchelper..."
  cd "$ROOT" && go build -o bin/svchelper ./tools/svchelper/ 2>/dev/null
fi
"$ROOT/bin/svchelper" "$@"
