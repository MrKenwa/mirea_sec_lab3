#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
cat > "$TMP/experiments.tsv" <<'EOF'
case_id	algorithm	label	charset	min_length	max_length	timeout
md5-unlimited	md5	easy	digits	6	6	0
EOF
EXPERIMENTS_FILE="$TMP/experiments.tsv" OUTPUT="$TMP/results.csv" MULTI_WORKERS=2 "$ROOT/scripts/run-go.sh" >/dev/null
rows=0
while IFS=, read -r timestamp tool version algorithm label status rest; do
	[[ "$timestamp" == timestamp ]] && continue
	[[ "$status" == found ]] || { echo "unexpected status: $status" >&2; exit 1; }
	rows=$((rows + 1))
done < "$TMP/results.csv"
[[ $rows -eq 2 ]] || { echo "expected 2 result rows, got $rows" >&2; exit 1; }
