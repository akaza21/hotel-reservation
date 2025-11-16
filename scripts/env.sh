#!/usr/bin/env bash
#
# Usage: source scripts/env.sh
# Sets up the Go toolchain/GOPATH for this repo without touching global shell dotfiles.

__get_script_dir() {
	# Works for both Bash ($BASH_SOURCE) and Zsh (${(%):-%N})
	local source="${BASH_SOURCE[0]:-${(%):-%N}}"
	# Resolve symlinks
	while [ -L "$source" ]; do
		local dir
		dir="$(cd -P "$(dirname "$source")" && pwd)"
		source="$(readlink "$source")"
		[[ $source != /* ]] && source="$dir/$source"
	done
	cd -P "$(dirname "$source")" && pwd
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
	echo "This script is meant to be sourced, not executed."
	echo "Run:  source scripts/env.sh"
	exit 1
fi

SCRIPT_DIR="$(__get_script_dir)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TOOLS_DIR="$REPO_ROOT/.tools"
GO_VERSION="1.24.4"
GO_DIST="go${GO_VERSION}.linux-amd64.tar.gz"
LOCAL_GOROOT="$TOOLS_DIR/go${GO_VERSION}"

mkdir -p "$TOOLS_DIR"

if [ ! -d "$LOCAL_GOROOT" ]; then
	if ! command -v curl >/dev/null 2>&1; then
		echo "curl is required to bootstrap Go ${GO_VERSION}. Please install curl first." >&2
		return 1
	fi
	echo "→ Installing Go ${GO_VERSION} into ${LOCAL_GOROOT}"
	TMP_DIR="$(mktemp -d)"
	curl -fsSL "https://go.dev/dl/${GO_DIST}" -o "$TMP_DIR/${GO_DIST}"
	tar -C "$TMP_DIR" -xzf "$TMP_DIR/${GO_DIST}"
	mv "$TMP_DIR/go" "$LOCAL_GOROOT"
	rm -rf "$TMP_DIR"
fi

export GOROOT="$LOCAL_GOROOT"
export GOPATH="${GOPATH:-$REPO_ROOT/.gopath}"
export GOMODCACHE="$GOPATH/pkg/mod"
export PATH="$GOROOT/bin:$GOPATH/bin:${PATH}"
mkdir -p "$GOPATH"/{pkg/mod,bin}

export HOTEL_RESERVATION_ENV="local-dev"

cat <<'EOF'
Go development environment configured for hotel-reservation.
  - Run `go version` to confirm
  - RUN_DB_TESTS=1 enables Mongo-backed tests (requires local MongoDB)
EOF
