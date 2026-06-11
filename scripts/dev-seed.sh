#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${PAAS_DEV_ENV_FILE:-$ROOT_DIR/deploy/config/paas-dev.env}"

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

export PAAS_HTTP_ADDR="${PAAS_HTTP_ADDR:-:8080}"
export MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
export MYSQL_PORT="${MYSQL_PORT:-3306}"
export MYSQL_DATABASE="${MYSQL_DATABASE:-paas}"
export MYSQL_USER="${MYSQL_USER:-paas}"
export MYSQL_PASSWORD="${MYSQL_PASSWORD:-change-me}"

paas_http_port() {
  local addr="$1"
  if [[ "$addr" == :* ]]; then
    printf '%s\n' "${addr#:}"
    return
  fi
  if [[ "$addr" == *:* ]]; then
    printf '%s\n' "${addr##*:}"
    return
  fi
  printf '%s\n' "$addr"
}

PAAS_HTTP_PORT="$(paas_http_port "$PAAS_HTTP_ADDR")"
export PAAS_DEV_SEED_API_BASE="${PAAS_DEV_SEED_API_BASE:-${VITE_API_BASE_URL:-http://127.0.0.1:${PAAS_HTTP_PORT}}}"

cd "$ROOT_DIR"
go run ./cmd/dev-seed "$@"
