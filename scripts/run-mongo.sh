#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MONGO_DIR="$REPO_ROOT/.tools/mongodb-7.0.12"
MONGO_BIN="$MONGO_DIR/bin/mongod"
DATA_DIR="$REPO_ROOT/.data/mongodb"
LOG_DIR="$REPO_ROOT/.logs"
LOG_FILE="$LOG_DIR/mongod.log"

if [ ! -x "$MONGO_BIN" ]; then
	echo "MongoDB binary not found at $MONGO_BIN. Run scripts/env.sh once to download toolchains." >&2
	exit 1
fi

mkdir -p "$DATA_DIR" "$LOG_DIR"

echo "Starting MongoDB (data dir: $DATA_DIR)..."
"$MONGO_BIN" \
	--dbpath "$DATA_DIR" \
	--logpath "$LOG_FILE" \
	--bind_ip 127.0.0.1 \
	--port 27017 \
	--fork

echo "MongoDB started. Logs: $LOG_FILE"
