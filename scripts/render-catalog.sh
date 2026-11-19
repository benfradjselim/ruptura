#!/usr/bin/env bash
# Writes the channel and bundle of a file-based catalog from one bundle image,
# next to the static package.yaml, then validates the catalog.
#
#   scripts/render-catalog.sh ghcr.io/benfradjselim/ruptura-operator-bundle:v1.4.9 [v1.4.9]
#
# With a second argument, refuses a bundle whose version differs, so a catalog
# never names one release and holds another. Needs opm on PATH and pull access
# to the bundle image.
set -euo pipefail
bundle=${1:?bundle image}
dir=$(dirname "$0")/../catalog/ruptura-operator
rendered=$(opm render "$bundle" --output yaml)
name=$(printf '%s\n' "$rendered" | sed -n 's/^name: \(ruptura-operator\.v[0-9][^ ]*\)$/\1/p' | head -1)
[ -n "$name" ] || { echo "no ruptura-operator bundle in $bundle" >&2; exit 1; }
if [ -n "${2:-}" ] && [ "$name" != "ruptura-operator.$2" ]; then
  echo "bundle $bundle holds $name, not ruptura-operator.$2: bump bundle/ before the tag" >&2
  exit 1
fi
{
  printf -- '---\nschema: olm.channel\nname: stable\npackage: ruptura-operator\nentries:\n  - name: %s\n' "$name"
  printf -- '---\n%s\n' "$rendered"
} > "$dir/catalog.yaml"
opm validate "$dir"
echo "catalog: $name"
