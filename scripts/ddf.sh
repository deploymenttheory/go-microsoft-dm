#!/usr/bin/env bash
# ddf.sh: download a DDF v2 drop into third_party/ddf and print the manifest
# entry for it. Drops are added beside the pinned one, never in its place, so
# `ddfgen diff` can compare them. Append the printed entry to MANIFEST.json
# and run `make verify`. Requires curl, jq, unzip and shasum.
#
# Usage: scripts/ddf.sh <url>
set -euo pipefail
URL="${1:-}"
[ -n "$URL" ] || { echo "usage: $0 <url>  (or make ddf DDF_URL=<url>)" >&2; exit 2; }
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/third_party/ddf"
name="${URL##*/}"
out="$DIR/$name"
if [ -f "$out" ]; then echo "ddf.sh: $out already exists; drops are never replaced" >&2; exit 1; fi
headers="$(mktemp)"
curl -fsSL --retry 3 -D "$headers" -o "$out" "$URL"
sha="$(shasum -a 256 "$out" | cut -d' ' -f1)"
size="$(stat -f %z "$out" 2>/dev/null || stat -c %s "$out")"
lm="$(grep -i '^last-modified:' "$headers" | tail -1 | cut -d' ' -f2- | tr -d '\r')"
etag="$(grep -i '^etag:' "$headers" | tail -1 | cut -d' ' -f2- | tr -d '\r"')"
rm -f "$headers"
iso="$(date -u -j -f '%a, %d %b %Y %H:%M:%S %Z' "$lm" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "$lm" +%Y-%m-%dT%H:%M:%SZ)"
top="$(unzip -Z1 "$out" | cut -d/ -f1 | sort -u | head -1)/"
files="$(unzip -Z1 "$out" | grep -ci '\.xml$' || true)"
jq -n --arg file "$name" --arg url "$URL" --arg sha "$sha" --argjson size "$size" \
  --arg lm "$iso" --arg etag "$etag" --arg top "$top" --argjson files "$files" \
  '{file:$file,url:$url,sha256:$sha,size:$size,lastModified:$lm,etag:$etag,topFolder:$top,files:$files}'
