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

export DOCKERFILE_REPOSITORY_URL=ssh://git@gitops:2422/paas/dockerfiles.git
export PAAS_HTTP_ADDR="${PAAS_HTTP_ADDR:-:8080}"
export PAAS_AUTO_MIGRATE="${PAAS_AUTO_MIGRATE:-false}"
export PAAS_REPOSITORY_DRIVER="mysql"

export MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
export MYSQL_PORT="${MYSQL_PORT:-3306}"
export MYSQL_DATABASE="${MYSQL_DATABASE:-paas}"
export MYSQL_USER="${MYSQL_USER:-paas}"
export MYSQL_PASSWORD="${MYSQL_PASSWORD:-change-me}"

export GITLAB_BASE_URL="${GITLAB_BASE_URL:-}"
export GITLAB_TOKEN="${GITLAB_TOKEN:-}"
export GITLAB_ROOT_GROUP_PATH="${GITLAB_ROOT_GROUP_PATH:-paas}"
export GITLAB_WEBHOOK_CALLBACK_URL="${GITLAB_WEBHOOK_CALLBACK_URL:-}"
export GITLAB_TIMEOUT_SECONDS="${GITLAB_TIMEOUT_SECONDS:-10}"
export GITLAB_RETRY_MAX="${GITLAB_RETRY_MAX:-2}"

export JENKINS_BASE_URL="${JENKINS_BASE_URL:-}"
export JENKINS_USERNAME="${JENKINS_USERNAME:-}"
export JENKINS_TOKEN="${JENKINS_TOKEN:-}"
export JENKINS_TIMEOUT_SECONDS="${JENKINS_TIMEOUT_SECONDS:-10}"
export JENKINS_RETRY_MAX="${JENKINS_RETRY_MAX:-2}"
export JENKINS_DEFAULT_TEMPLATE_ID="${JENKINS_DEFAULT_TEMPLATE_ID:-java-unified-v1}"
export JENKINS_CALLBACK_BASE_URL=http://122.152.196.135:18080

export IMAGE_REPOSITORY=cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg
export DOCKERFILE_REPOSITORY_URL="${DOCKERFILE_REPOSITORY_URL:-ssh://git@192.168.100.80:2422/paas/dockerfiles.git}"
export DOCKERFILE_REPOSITORY_REF="${DOCKERFILE_REPOSITORY_REF:-main}"
export DOCKERFILE_REPOSITORY_CREDENTIALS_ID="${DOCKERFILE_REPOSITORY_CREDENTIALS_ID:-}"

WEB_HOST="${WEB_HOST:-0.0.0.0}"
WEB_PORT="${WEB_PORT:-5173}"
export VITE_API_BASE_URL="${VITE_API_BASE_URL:-http://127.0.0.1${PAAS_HTTP_ADDR}}"

pids=()

cleanup() {
  local code=$?
  trap - EXIT INT TERM
  for pid in "${pids[@]}"; do
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
  wait >/dev/null 2>&1 || true
  exit "$code"
}

trap cleanup EXIT INT TERM

echo "PaaS dev environment"
echo "  env file: $ENV_FILE"
echo "  backend:  http://127.0.0.1${PAAS_HTTP_ADDR}"
echo "  frontend: http://127.0.0.1:${WEB_PORT}"
echo "  API base: $VITE_API_BASE_URL"
if [[ -n "$GITLAB_BASE_URL" && -n "$GITLAB_TOKEN" ]]; then
  echo "  GitLab:   real adapter"
else
  echo "  GitLab:   fake adapter"
fi
if [[ -n "$JENKINS_BASE_URL" && -n "$JENKINS_TOKEN" ]]; then
  echo "  Jenkins:  real adapter"
else
  echo "  Jenkins:  fake adapter"
fi

(
  cd "$ROOT_DIR"
  go run ./cmd/paas-server
) &
pids+=("$!")

(
  cd "$ROOT_DIR/web/console"
  npm run dev -- --host "$WEB_HOST" --port "$WEB_PORT"
) &
pids+=("$!")

wait -n "${pids[@]}"
cleanup
