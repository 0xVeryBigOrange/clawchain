#!/usr/bin/env bash
# Generate SHA256 checksums for all skill/ files.
# Output: CHECKSUMS.txt in the repo root.
#
# Usage:
#   cd /path/to/clawchain
#   bash scripts/gen_checksums.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECKSUMS_FILE="$REPO_ROOT/CHECKSUMS.txt"

cd "$REPO_ROOT"

echo "Generating SHA256 checksums for skill/ files..."

# Find all files in skill/ (exclude __pycache__ and .pyc)
find skill -type f \
  ! -path '*__pycache__*' \
  ! -name '*.pyc' \
  ! -name '.DS_Store' \
  | sort \
  | xargs shasum -a 256 \
  > "$CHECKSUMS_FILE"

echo "✅ Checksums written to $CHECKSUMS_FILE"
echo "   $(wc -l < "$CHECKSUMS_FILE" | tr -d ' ') files checksummed"
cat "$CHECKSUMS_FILE"
