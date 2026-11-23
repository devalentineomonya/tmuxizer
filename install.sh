#!/bin/sh
# Build tmuxizer from this checkout, install it, then run `tmuxizer setup`.
#
#   ./install.sh            install to ~/.local/bin and run setup
#   ./install.sh -y         the same, answering yes to setup's questions
#   PREFIX=/usr/local ./install.sh
#
# The binary remembers this checkout, so `tmuxizer update` can pull and rebuild.
set -eu

cd "$(dirname "$0")"
src=$(pwd)
prefix=${PREFIX:-$HOME/.local}

need() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "tmuxizer: $1 is required. $2" >&2
		exit 1
	}
}
need go "Install Go 1.26 or newer: https://go.dev/dl/"
need tmux "Install it with your package manager."

version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
mkdir -p "$prefix/bin"
go build -ldflags "-s -w -X main.version=$version -X main.source=$src" \
	-o "$prefix/bin/tmuxizer" ./cmd/tmuxizer
echo "installed tmuxizer $version to $prefix/bin/tmuxizer"

case ":$PATH:" in
*":$prefix/bin:"*) ;;
*) echo "note: $prefix/bin is not on your PATH" ;;
esac

exec "$prefix/bin/tmuxizer" setup "$@"
