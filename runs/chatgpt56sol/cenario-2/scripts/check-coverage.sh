#!/bin/sh
set -eu
profile="${1:-coverage.out}"
minimum="${2:-95}"
go tool cover -func="$profile"
actual="$(go tool cover -func="$profile" | awk '/^total:/ {gsub("%", "", $3); print $3}')"
awk -v actual="$actual" -v minimum="$minimum" 'BEGIN { if (actual + 0 < minimum + 0) { printf "coverage %.1f%% is below %.1f%%\n", actual, minimum > "/dev/stderr"; exit 1 } }'
printf 'coverage gate passed: %s%% >= %s%%\n' "$actual" "$minimum"
