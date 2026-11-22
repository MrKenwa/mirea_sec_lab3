#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
HASHES_FILE=${HASHES_FILE:-"$ROOT/testdata/hashes.tsv"}
EXPERIMENTS_FILE=${EXPERIMENTS_FILE:-"$ROOT/testdata/experiments.example.tsv"}
OUTPUT=${OUTPUT:-"$ROOT/results/go.csv"}
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
go build -o "$TMP/hashbrute" "$ROOT/cmd/hashbrute"
IFS=$'\t' read -r h1 h2 h3 < "$HASHES_FILE"; [[ "$h1" == algorithm && "$h2" == label && "$h3" == hash ]] || { echo 'invalid hashes header' >&2; exit 2; }
IFS=$'\t' read -r e1 e2 e3 e4 e5 e6 e7 < "$EXPERIMENTS_FILE"; [[ "$e1" == case_id && "$e2" == algorithm && "$e3" == label ]] || { echo 'invalid experiments header' >&2; exit 2; }
if [[ -n ${MULTI_WORKERS:-} ]]; then multi=$MULTI_WORKERS; elif [[ $(uname) == Darwin ]]; then multi=$(sysctl -n hw.logicalcpu); else multi=$(getconf _NPROCESSORS_ONLN); fi
lookup_hash(){ local algorithm=$1 label=$2 a l hash; while IFS=$'\t' read -r a l hash; do [[ "$a" == "$algorithm" && "$l" == "$label" ]] && { printf '%s' "$hash"; return 0; }; done < <(tail -n +2 "$HASHES_FILE"); return 1; }
while IFS=$'\t' read -r case algorithm label charset min max timeout; do
 [[ -z "$case" ]] && continue; [[ "$timeout" == 0 || "$timeout" =~ ^[1-9][0-9]*[smh]$ ]] || { echo "invalid timeout: $timeout" >&2; exit 2; }; hash=$(lookup_hash "$algorithm" "$label") || { echo "missing hash: $algorithm/$label" >&2; exit 2; }
 for workers in 1 "$multi"; do [[ "$algorithm" == argon2id && -z ${ARGON2_WORKERS+x} ]] && workers=1; set +e; "$TMP/hashbrute" -algorithm "$algorithm" -hash "$hash" -label "$label" -charset "$charset" -min-length "$min" -max-length "$max" -timeout "$timeout" -workers "$workers" -output "$OUTPUT"; status=$?; set -e; [[ $status == 0 || $status == 1 ]] || exit "$status"; done
done < <(tail -n +2 "$EXPERIMENTS_FILE")
