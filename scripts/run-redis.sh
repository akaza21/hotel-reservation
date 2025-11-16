#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REDIS_DIR="$REPO_ROOT/.tools/redis-7.2.5"
REDIS_BIN="$REDIS_DIR/src/redis-server"
DATA_DIR="$REPO_ROOT/.data/redis"
LOG_DIR="$REPO_ROOT/.logs"
LOG_FILE="$LOG_DIR/redis.log"

if [ ! -x "$REDIS_BIN" ]; then
	echo "Redis binary not found at $REDIS_BIN. Run scripts/env.sh once to download/build toolchains." >&2
	exit 1
fi

mkdir -p "$DATA_DIR" "$LOG_DIR"

echo "Starting Redis (dir: $DATA_DIR)..."
"$REDIS_BIN" \
	--dir "$DATA_DIR" \
	--logfile "$LOG_FILE" \
	--daemonize yes \
	--bind 127.0.0.1 \
	--port 6379

echo "Redis started. Logs: $LOG_FILE"
