#!/usr/bin/env bash
set -e

# Resolve repository and server directory paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
ROOT_DIR="$(cd "${SERVER_DIR}/../.." && pwd)"

cd "${SERVER_DIR}"

# Locate and source .env file
if [ -f "${SERVER_DIR}/.env" ]; then
  echo "==> Loading environment variables from ${SERVER_DIR}/.env"
  set -a
  # shellcheck disable=SC1091
  source "${SERVER_DIR}/.env"
  set +a
elif [ -f "${ROOT_DIR}/.env" ]; then
  echo "==> Loading environment variables from ${ROOT_DIR}/.env"
  set -a
  # shellcheck disable=SC1091
  source "${ROOT_DIR}/.env"
  set +a
fi

# Run air in watch mode
if command -v air >/dev/null 2>&1; then
  exec air
elif command -v mise >/dev/null 2>&1; then
  exec mise exec -- air
else
  echo "Error: 'air' is required for server watch mode. Please install air or run with mise." >&2
  exit 1
fi
