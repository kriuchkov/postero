#!/usr/bin/env bash
#
# Regenerates the Postero TUI demo GIF used in the README (docs/demo.gif).
#
# It builds the binary and renders docs/demo.tape with VHS, which drives the
# first-run setup wizard into demo mode (ctrl+d) and walks the inbox, a message,
# folder navigation and the composer — no real account required.
#
# Requirements: vhs (github.com/charmbracelet/vhs) and ffmpeg.
#   macOS:  brew install vhs
#
# Usage: scripts/demo.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

command -v vhs >/dev/null 2>&1 || {
  echo "error: 'vhs' is required — install with: brew install vhs"
  exit 1
}

echo "==> building pstr"
go build -o bin/pstr ./cmd/pstr

echo "==> rendering docs/demo.gif from docs/demo.tape"
vhs docs/demo.tape

echo "==> done: docs/demo.gif"
