#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MYSQL_CONTAINER_ID=""
TSBUILDINFO="$ROOT_DIR/web/console/tsconfig.tsbuildinfo"
TSBUILDINFO_WAS_CLEAN=0

cleanup() {
  local code=$?
  trap - EXIT INT TERM
  if [[ -n "$MYSQL_CONTAINER_ID" ]]; then
    docker stop "$MYSQL_CONTAINER_ID" >/dev/null 2>&1 || true
  fi
  if [[ "$TSBUILDINFO_WAS_CLEAN" -eq 1 ]]; then
    git -C "$ROOT_DIR" restore -- "$TSBUILDINFO" >/dev/null 2>&1 || true
  fi
  exit "$code"
}

trap cleanup EXIT INT TERM

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "缺少命令: $name" >&2
    exit 1
  fi
}

wait_for_mysql() {
  local container_id="$1"
  local password="$2"
  local ready=0

  echo "等待测试 MySQL 启动..."
  for _ in $(seq 1 90); do
    if docker exec "$container_id" mysqladmin ping -uroot -p"$password" --silent >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done

  if [[ "$ready" -ne 1 ]]; then
    echo "测试 MySQL 启动超时" >&2
    docker logs "$container_id" >&2 || true
    exit 1
  fi
}

start_mysql_if_needed() {
  if [[ -n "${PAAS_TEST_MYSQL_DSN:-}" ]]; then
    echo "使用已有 PAAS_TEST_MYSQL_DSN，不启动测试 MySQL 容器。"
    return
  fi

  require_command docker

  local image="${PAAS_TEST_MYSQL_IMAGE:-m.daocloud.io/docker.io/library/mysql:8.0}"
  local password="${PAAS_TEST_MYSQL_PASSWORD:-password}"

  echo "启动测试 MySQL 容器: $image"
  MYSQL_CONTAINER_ID="$(docker run -d --rm \
    -e MYSQL_ROOT_PASSWORD="$password" \
    -p 127.0.0.1::3306 \
    "$image")"

  wait_for_mysql "$MYSQL_CONTAINER_ID" "$password"

  local port
  port="$(docker port "$MYSQL_CONTAINER_ID" 3306/tcp | sed 's/.*://')"
  export PAAS_TEST_MYSQL_DSN="root:${password}@tcp(127.0.0.1:${port})/mysql?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=true&loc=Local&interpolateParams=true"
  echo "测试 MySQL 就绪: 127.0.0.1:$port"
}

remember_tsbuildinfo_state() {
  if [[ -f "$TSBUILDINFO" ]] &&
    git -C "$ROOT_DIR" diff --quiet -- "$TSBUILDINFO" &&
    git -C "$ROOT_DIR" diff --cached --quiet -- "$TSBUILDINFO"; then
    TSBUILDINFO_WAS_CLEAN=1
  fi
}

require_command go
require_command npm
remember_tsbuildinfo_state
start_mysql_if_needed

cd "$ROOT_DIR"
go test -p "${PAAS_GO_TEST_P:-1}" -count=1 ./...

cd "$ROOT_DIR/web/console"
npm test -- --coverage
npm run build
