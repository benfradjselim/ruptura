#!/usr/bin/env bash
# Fails if docs/REFERENCE.md documents a RUPTURA_* env var that no Go source reads.
# Guards against doc-code drift (FABLE.md FBL-A0-3).
set -euo pipefail
cd "$(dirname "$0")/.."
fail=0
vars=$(grep -oE '^RUPTURA_[A-Z_]+' docs/REFERENCE.md | sort -u)
for v in $vars; do
  if ! grep -rq "$v" workdir --include="*.go"; then
    echo "::error::$v is documented in docs/REFERENCE.md but never read in workdir/*.go"
    fail=1
  fi
done
[ $fail -eq 0 ] && echo "docs-env check OK: all documented env vars exist in code"
exit $fail
