#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${APP_DIR}"

# Run Flutter Web development server on port 3000 by default, passing through any extra flags
if command -v flutter >/dev/null 2>&1; then
  exec flutter run -d web-server --web-port 3000 "$@"
elif command -v mise >/dev/null 2>&1; then
  exec mise exec -- flutter run -d web-server --web-port 3000 "$@"
else
  echo "Error: 'flutter' is required for the app dev server. Please install flutter or run with mise." >&2
  exit 1
fi
