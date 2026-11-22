#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
HASHES_FILE=${HASHES_FILE:-"$ROOT/testdata/hashes.tsv"}; EXPERIMENTS_FILE=${EXPERIMENTS_FILE:-"$ROOT/testdata/experiments.example.tsv"}; OUTPUT=${OUTPUT:-"$ROOT/results/hashcat.csv"}; DEVICE_TYPE=${DEVICE_TYPE:-cpu}
command -v hashcat >/dev/null 2>&1 || { echo 'hashcat is required' >&2; exit 2; }
hashcat --version; VERSION=$(hashcat --version); hashcat -I
[[ "$DEVICE_TYPE" == cpu || "$DEVICE_TYPE" == gpu ]] || { echo 'DEVICE_TYPE must be cpu or gpu' >&2; exit 2; }
DEVICE=1; [[ "$DEVICE_TYPE" == gpu ]] && DEVICE=2
IFS=$'\t' read -r h1 h2 h3 < "$HASHES_FILE"; [[ "$h1" == algorithm && "$h2" == label && "$h3" == hash ]] || { echo 'invalid hashes header' >&2; exit 2; }
IFS=$'\t' read -r e1 e2 e3 e4 e5 e6 e7 < "$EXPERIMENTS_FILE"; [[ "$e1" == case_id && "$e2" == algorithm && "$e3" == label && "$e4" == charset ]] || { echo 'invalid experiments header' >&2; exit 2; }
mkdir -p "$(dirname "$OUTPUT")" "$ROOT/results/hashcat-logs"
csv_escape(){ local x=${1//\"/\"\"}; printf '"%s"' "$x"; }
csv_row(){ local first=1; for field in "$@"; do [[ $first == 1 ]] || printf ',' >> "$OUTPUT"; csv_escape "$field" >> "$OUTPUT"; first=0; done; printf '\n' >> "$OUTPUT"; }
[[ -s "$OUTPUT" ]] || printf 'case_id,timestamp,tool,version,algorithm,label,status,password,charset,min_length,max_length,elapsed_seconds,exit_code,device,log_path\n' > "$OUTPUT"
lookup_hash(){ local algorithm=$1 label=$2 a l hash; while IFS=$'\t' read -r a l hash; do [[ "$a" == "$algorithm" && "$l" == "$label" ]] && { printf '%s' "$hash"; return 0; }; done < <(tail -n +2 "$HASHES_FILE"); return 1; }
seconds(){ [[ "$1" == 0 ]] && { printf 0; return 0; }; local n=${1%?} unit=${1: -1}; [[ "$1" =~ ^[1-9][0-9]*[smh]$ ]] || return 1; case $unit in s) printf %s "$n";;m) printf %s "$((n*60))";;h) printf %s "$((n*3600))";;esac; }
while IFS=$'\t' read -r case algorithm label charset min max timeout; do
 [[ -z "$case" ]] && continue; runtime=$(seconds "$timeout") || { echo "invalid timeout: $timeout" >&2; exit 2; }; runtime_args=(); [[ "$runtime" == 0 ]] || runtime_args=(--runtime "$runtime"); hash=$(lookup_hash "$algorithm" "$label") || { echo "missing hash: $algorithm/$label" >&2; exit 2; }
 case "$algorithm" in md5) mode=0;;sha1) mode=100;;bcrypt) mode=3200;;argon2id) mode=34000;;*) echo "unsupported algorithm: $algorithm" >&2; exit 2;;esac
 case "$charset" in digits) mask=$(printf '?d%.0s' $(seq 1 "$max")); extra=();;lower) mask=$(printf '?l%.0s' $(seq 1 "$max")); extra=();;alnum) mask=$(printf '?1%.0s' $(seq 1 "$max")); extra=(-1 '?l?u?d');;printable) mask=$(printf '?a%.0s' $(seq 1 "$max")); extra=();;*) echo "unsupported charset: $charset" >&2; exit 2;;esac
 tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT; printf '%s\n' "$hash" > "$tmp/hash"; outfile="$tmp/out"; timefile="$tmp/time"; log="$ROOT/results/hashcat-logs/${case}-${DEVICE_TYPE}.log"; set +e; /usr/bin/time -p -o "$timefile" hashcat -m "$mode" -a 3 -D "$DEVICE" --increment --increment-min "$min" --increment-max "$max" "${runtime_args[@]}" --potfile-disable --restore-disable --outfile "$outfile" --outfile-format 2 "${extra[@]}" "$tmp/hash" "$mask" >"$log" 2>&1; code=$?; set -e
 status=error; password=; case $code in 0) status=found; IFS= read -r password < "$outfile" || true;;1) status=exhausted;;4) status=timeout;;2|3|5) status=cancelled;;esac; elapsed=; while read -r key value; do [[ "$key" == real ]] && elapsed=$value; done < "$timefile"; csv_row "$case" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" hashcat "$VERSION" "$algorithm" "$label" "$status" "$password" "$charset" "$min" "$max" "$elapsed" "$code" "$DEVICE_TYPE" "$log"
 [[ $status != error ]] || exit 2; rm -rf "$tmp"; trap - EXIT
done < <(tail -n +2 "$EXPERIMENTS_FILE")
