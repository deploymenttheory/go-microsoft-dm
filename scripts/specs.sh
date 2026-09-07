#!/usr/bin/env bash
# specs.sh: download the pinned specifications listed in
# third_party/specs/MANIFEST.json and verify each file's SHA-256.
#
# Downloads are gitignored (the OMA and Microsoft documents carry their own
# use terms); only the manifest is committed. With SPECS_RECORD=1 an entry
# whose sha256 is empty is filled in from the fetched file and the manifest is
# rewritten. Requires curl, jq and shasum.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/third_party/specs"
MANIFEST="$DIR/MANIFEST.json"
command -v jq >/dev/null || { echo "specs.sh: jq is required (brew install jq)" >&2; exit 2; }
mkdir -p "$DIR"
fail=0
updated="$(cat "$MANIFEST")"
count="$(jq '.specs | length' "$MANIFEST")"
for ((i = 0; i < count; i++)); do
  name="$(jq -r ".specs[$i].name" "$MANIFEST")"
  url="$(jq -r ".specs[$i].url" "$MANIFEST")"
  want="$(jq -r ".specs[$i].sha256" "$MANIFEST")"
  out="$DIR/$name"
  if [ ! -f "$out" ]; then
    echo "fetching $name"
    curl -fsSL --retry 3 -o "$out" "$url" || { echo "specs.sh: download failed: $url" >&2; fail=1; continue; }
  fi
  got="$(shasum -a 256 "$out" | cut -d' ' -f1)"
  if [ -z "$want" ] || [ "$want" = "null" ]; then
    if [ "${SPECS_RECORD:-}" = "1" ]; then
      echo "recording $name $got"
      updated="$(jq --arg i "$i" --arg s "$got" '.specs[($i|tonumber)].sha256 = $s' <<< "$updated")"
    else
      echo "specs.sh: $name has no recorded sha256 (run with SPECS_RECORD=1)" >&2; fail=1
    fi
  elif [ "$got" != "$want" ]; then
    echo "specs.sh: $name sha256 $got does not match manifest $want" >&2; fail=1
  else
    echo "ok $name"
  fi
done
if [ "${SPECS_RECORD:-}" = "1" ]; then
  printf '%s\n' "$updated" > "$MANIFEST"
fi
exit $fail
