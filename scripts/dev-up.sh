#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${PAAS_DEV_ENV_FILE:-$ROOT_DIR/deploy/config/paas-dev.env}"
RECREATE_DB_ARG=""
SEED_DEV_DATA_ARG=""

usage() {
  cat <<'EOF'
Usage:
  scripts/dev-up.sh [options]

Options:
  --recreate-db        启动前删除并重建 MYSQL_DATABASE，然后执行全量 migration 建表；默认注入开发测试数据。
  --no-recreate-db     不重建数据库，按当前 PAAS_AUTO_MIGRATE 设置启动。
  --seed-dev-data      后端 ready 后注入 SBG/MACC 开发测试数据。
  --no-seed-dev-data   跳过开发测试数据注入。
  -h, --help           显示帮助。

Examples:
  ./scripts/dev-up.sh
  ./scripts/dev-up.sh --recreate-db
  ./scripts/dev-up.sh --recreate-db --no-seed-dev-data
  ./scripts/dev-up.sh --seed-dev-data
  PAAS_AUTO_MIGRATE=true ./scripts/dev-up.sh
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --recreate-db)
      RECREATE_DB_ARG=true
      shift
      ;;
    --no-recreate-db)
      RECREATE_DB_ARG=false
      shift
      ;;
    --seed-dev-data)
      SEED_DEV_DATA_ARG=true
      shift
      ;;
    --no-seed-dev-data)
      SEED_DEV_DATA_ARG=false
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

RECREATE_DB="${PAAS_DEV_RECREATE_DB:-false}"
if [[ -n "$RECREATE_DB_ARG" ]]; then
  RECREATE_DB="$RECREATE_DB_ARG"
fi
case "$RECREATE_DB" in
  true|false)
    ;;
  *)
    echo "PAAS_DEV_RECREATE_DB 只能为 true 或 false，当前值: $RECREATE_DB" >&2
    exit 2
    ;;
esac

SEED_DEV_DATA="${PAAS_DEV_SEED_DATA:-}"
if [[ -z "$SEED_DEV_DATA" ]]; then
  if [[ "$RECREATE_DB" == "true" ]]; then
    SEED_DEV_DATA=true
  else
    SEED_DEV_DATA=false
  fi
fi
if [[ -n "$SEED_DEV_DATA_ARG" ]]; then
  SEED_DEV_DATA="$SEED_DEV_DATA_ARG"
fi
case "$SEED_DEV_DATA" in
  true|false)
    ;;
  *)
    echo "PAAS_DEV_SEED_DATA 只能为 true 或 false，当前值: $SEED_DEV_DATA" >&2
    exit 2
    ;;
esac

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

# GitOps 清单仓库和平台 Chart 仓库可以使用独立 GitLab。
# 示例：
#   GITOPS_GITLAB_BASE_URL=http://gitops
#   GITOPS_GITLAB_TOKEN=glpat-xxx
#   GITLAB_MANIFEST_PROJECT_ID=99
#   GITLAB_MANIFEST_REPO_URL=ssh://git@gitops:2422/paas/manifests.git
#   GITLAB_CHART_PROJECT_ID=100
#   GITLAB_CHART_REPO_URL=ssh://git@gitops:2422/paas/charts.git
export GITOPS_GITLAB_BASE_URL="${GITOPS_GITLAB_BASE_URL:-}"
export GITOPS_GITLAB_TOKEN="${GITOPS_GITLAB_TOKEN:-}"
export GITOPS_GITLAB_TIMEOUT_SECONDS="${GITOPS_GITLAB_TIMEOUT_SECONDS:-}"
export GITOPS_GITLAB_RETRY_MAX="${GITOPS_GITLAB_RETRY_MAX:-}"
export GITLAB_MANIFEST_PROJECT_ID="${GITLAB_MANIFEST_PROJECT_ID:-}"
export GITLAB_MANIFEST_REPO_URL="${GITLAB_MANIFEST_REPO_URL:-}"
export GITLAB_CHART_PROJECT_ID="${GITLAB_CHART_PROJECT_ID:-}"
export GITLAB_CHART_REPO_URL="${GITLAB_CHART_REPO_URL:-}"
export PAAS_PLATFORM_CHART_NAME="${PAAS_PLATFORM_CHART_NAME:-paas-app}"
export PAAS_PLATFORM_CHART_VERSION="${PAAS_PLATFORM_CHART_VERSION:-0.1.0}"

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
BACKEND_LOCAL_BASE_URL="${PAAS_DEV_BACKEND_LOCAL_BASE_URL:-http://127.0.0.1:${PAAS_HTTP_PORT}}"
export VITE_API_BASE_URL="${VITE_API_BASE_URL:-$BACKEND_LOCAL_BASE_URL}"

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

recreate_database() {
  case "$MYSQL_DATABASE" in
    ""|"mysql"|"information_schema"|"performance_schema"|"sys")
      echo "拒绝重建系统数据库: ${MYSQL_DATABASE:-<empty>}" >&2
      exit 1
      ;;
  esac

  echo "即将重建开发数据库: $MYSQL_DATABASE@$MYSQL_HOST:$MYSQL_PORT"
  echo "该操作会删除该库中的应用、构建、Freight、Stage、审计等数据。"

  local tmp
  tmp="$(mktemp /tmp/paas-recreate-db-XXXX.go)"
  cat > "$tmp" <<'EOF'
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	host := os.Getenv("MYSQL_HOST")
	port := os.Getenv("MYSQL_PORT")
	user := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")
	database := os.Getenv("MYSQL_DATABASE")
	if strings.TrimSpace(database) == "" {
		panic("MYSQL_DATABASE is required")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=true&loc=Local&interpolateParams=true", user, password, host, port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		panic(err)
	}
	quoted := "`" + strings.ReplaceAll(database, "`", "``") + "`"
	if _, err := db.Exec("DROP DATABASE IF EXISTS " + quoted); err != nil {
		panic(err)
	}
	if _, err := db.Exec("CREATE DATABASE " + quoted + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		panic(err)
	}
	fmt.Printf("已重建开发数据库: %s\n", database)
}
EOF
  if ! (cd "$ROOT_DIR" && go run "$tmp"); then
    rm -f "$tmp"
    return 1
  fi
  rm -f "$tmp"
}

wait_for_backend_ready() {
  local ready_url="${BACKEND_LOCAL_BASE_URL}/readyz"
  local attempts="${PAAS_DEV_READY_RETRIES:-60}"
  local interval="${PAAS_DEV_READY_INTERVAL_SECONDS:-1}"
  echo "等待后端 ready: $ready_url"
  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$ready_url" >/dev/null 2>&1; then
      echo "后端已 ready"
      return 0
    fi
    sleep "$interval"
  done
  echo "等待后端 ready 超时: $ready_url" >&2
  return 1
}

if [[ "$RECREATE_DB" == "true" ]]; then
  export PAAS_AUTO_MIGRATE=true
  recreate_database
fi

echo "PaaS dev environment"
echo "  env file: $ENV_FILE"
echo "  backend:  $BACKEND_LOCAL_BASE_URL"
echo "  frontend: http://${WEB_HOST}:${WEB_PORT}"
echo "  API base: $VITE_API_BASE_URL"
echo "  migrate:  $PAAS_AUTO_MIGRATE"
echo "  recreate: $RECREATE_DB"
echo "  dev seed: $SEED_DEV_DATA"
if [[ -n "$GITLAB_BASE_URL" && -n "$GITLAB_TOKEN" ]]; then
  echo "  Source GitLab: real adapter"
else
  echo "  Source GitLab: fake adapter"
fi
GITOPS_EFFECTIVE_GITLAB_BASE_URL="${GITOPS_GITLAB_BASE_URL:-$GITLAB_BASE_URL}"
GITOPS_EFFECTIVE_GITLAB_TOKEN="${GITOPS_GITLAB_TOKEN:-$GITLAB_TOKEN}"
if [[ -n "$GITOPS_EFFECTIVE_GITLAB_BASE_URL" && -n "$GITOPS_EFFECTIVE_GITLAB_TOKEN" && -n "$GITLAB_MANIFEST_PROJECT_ID" && -n "$GITLAB_CHART_PROJECT_ID" ]]; then
  echo "  GitOps GitLab: real adapter"
  echo "  Manifest repo: ${GITLAB_MANIFEST_REPO_URL:-${GITOPS_EFFECTIVE_GITLAB_BASE_URL%/}/${GITLAB_MANIFEST_PROJECT_ID}.git}"
  echo "  Chart repo:    ${GITLAB_CHART_REPO_URL:-${GITOPS_EFFECTIVE_GITLAB_BASE_URL%/}/${GITLAB_CHART_PROJECT_ID}.git}"
else
  echo "  GitOps GitLab: fake adapter"
fi
if [[ -n "$JENKINS_BASE_URL" && -n "$JENKINS_TOKEN" ]]; then
  echo "  Jenkins:  real adapter"
else
  echo "  Jenkins:  fake adapter"
fi

if curl -fsS "${BACKEND_LOCAL_BASE_URL}/readyz" >/dev/null 2>&1; then
  echo "后端已在运行，跳过启动: $BACKEND_LOCAL_BASE_URL"
else
  (
    cd "$ROOT_DIR"
    go run ./cmd/paas-server
  ) &
  pids+=("$!")
fi

if [[ "$SEED_DEV_DATA" == "true" ]]; then
  wait_for_backend_ready
  "$ROOT_DIR/scripts/dev-seed.sh" --api-base "$VITE_API_BASE_URL"
fi

(
  cd "$ROOT_DIR/web/console"
  npm run dev -- --host "$WEB_HOST" --port "$WEB_PORT"
) &
pids+=("$!")

wait -n "${pids[@]}"
cleanup
