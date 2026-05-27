#!/bin/bash
set -e
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

build() {
  echo "Building $1..."
  mkdir -p "$ROOT/bin"
  go build -o "$ROOT/bin/$1" "./apps/$1/cmd/"
  echo "  > $ROOT/bin/$1"
}

case "${1:-all}" in
  all)      build chatsvr; build gateway; echo "Done. Binaries at $ROOT/bin/" ;;
  chatsvr)  build chatsvr ;;
  gateway)  build gateway ;;
  *)        echo "Usage: $0 [all|chatsvr|gateway]" ;;
esac
