#!/bin/sh
# Fails when the branch changes a file outside the mission's allowed paths.
# Usage: scripts/mission/guard.sh <base-ref> <allowed-path> [allowed-path...]
base=$1
shift
pattern=$(printf '%s\n' "$@" | sed 's/[.[\*^$/]/\\&/g' | paste -sd '|' -)
outside=$(git diff --name-only "$base"...HEAD | grep -Ev "^($pattern)" || true)
if [ -n "$outside" ]; then
  echo "Files outside mission scope:"
  echo "$outside"
  exit 1
fi
