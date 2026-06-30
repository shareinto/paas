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
  --seed-dev-data      后端 ready 后通过 MySQL 客户端注入 SBG/MACC 开发测试数据。
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
export JENKINS_CALLBACK_BASE_URL=http://10.0.97.152:8080

export IMAGE_REPOSITORY=cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg
export DOCKERFILE_REPOSITORY_URL="${DOCKERFILE_REPOSITORY_URL:-ssh://git@192.168.100.80:2422/paas/dockerfiles.git}"
export DOCKERFILE_REPOSITORY_REF="${DOCKERFILE_REPOSITORY_REF:-main}"
export DOCKERFILE_REPOSITORY_CREDENTIALS_ID="${DOCKERFILE_REPOSITORY_CREDENTIALS_ID:-}"

WEB_HOST="${WEB_HOST:-0.0.0.0}"
WEB_PORT="${WEB_PORT:-5173}"
DEV_SEED_SOURCE_URL="${PAAS_DEV_SEED_SOURCE_URL:-http://192.168.100.80/paas/sbg/macc/log-receiver.git}"
DEV_SEED_ARTIFACT_COPY_COMMAND='cp log-receiver/build/libs/log-receiver.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"'
MYSQL_CLIENT_IMAGE="${PAAS_MYSQL_CLIENT_IMAGE:-m.daocloud.io/docker.io/library/mysql:8.0}"

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
pid_labels=()

cleanup() {
  local code=$?
  trap - EXIT INT TERM
  if ((${#pids[@]} > 0)); then
    for pid in "${pids[@]}"; do
      if kill -0 "$pid" >/dev/null 2>&1; then
        kill "$pid" >/dev/null 2>&1 || true
      fi
    done
  fi
  wait >/dev/null 2>&1 || true
  exit "$code"
}

trap cleanup EXIT INT TERM

track_pid() {
  local label="$1"
  local pid="$2"
  pids+=("$pid")
  pid_labels+=("$label")
}

kill_pids() {
  local label="$1"
  shift
  local pids=("$@")
  local pid alive=()
  if ((${#pids[@]} == 0)); then
    return
  fi

  echo "清理已存在的${label}进程: ${pids[*]}"
  for pid in "${pids[@]}"; do
    if [[ "$pid" != "$$" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done

  sleep 1
  for pid in "${pids[@]}"; do
    if [[ "$pid" != "$$" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      alive+=("$pid")
    fi
  done
  if ((${#alive[@]} > 0)); then
    echo "强制清理未退出的${label}进程: ${alive[*]}"
    kill -9 "${alive[@]}" >/dev/null 2>&1 || true
  fi
}

kill_processes_on_port() {
  local label="$1"
  local port="$2"
  local raw_pids pids=() pid existing found
  if [[ -z "$port" || "$port" == "0" ]]; then
    return
  fi
  if ! command -v lsof >/dev/null 2>&1; then
    echo "未找到 lsof，跳过${label}端口 ${port} 的旧进程清理。" >&2
    return
  fi

  raw_pids="$(lsof -nP -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [[ -z "$raw_pids" ]]; then
    return
  fi
  while IFS= read -r pid; do
    [[ -z "$pid" ]] && continue
    found=false
    if ((${#pids[@]} > 0)); then
      for existing in "${pids[@]}"; do
        if [[ "$existing" == "$pid" ]]; then
          found=true
          break
        fi
      done
    fi
    if [[ "$found" == "false" ]]; then
      pids+=("$pid")
    fi
  done <<<"$raw_pids"
  if ((${#pids[@]} > 0)); then
    kill_pids "$label" "${pids[@]}"
  fi
}

kill_existing_dev_processes() {
  kill_processes_on_port "后端" "$PAAS_HTTP_PORT"
  kill_processes_on_port "前端" "$WEB_PORT"
}

ensure_frontend_dependencies() {
  local web_dir="$ROOT_DIR/web/console"
  if [[ -x "$web_dir/node_modules/.bin/vite" ]]; then
    return
  fi
  if ! command -v npm >/dev/null 2>&1; then
    echo "未找到 npm，无法启动前端开发服务器。请先安装 Node.js 和 npm。" >&2
    return 127
  fi
  echo "前端依赖缺失，正在安装 web/console 依赖..."
  (
    cd "$web_dir"
    npm install --no-audit --no-fund
  )
}

mysql_tcp_ready() {
  (exec 3<>"/dev/tcp/${MYSQL_HOST}/${MYSQL_PORT}") >/dev/null 2>&1
}

ensure_mysql_reachable() {
  local attempts="${PAAS_DEV_MYSQL_RETRIES:-5}"
  local interval="${PAAS_DEV_MYSQL_INTERVAL_SECONDS:-1}"
  local i
  echo "检查 MySQL 连接: $MYSQL_HOST:$MYSQL_PORT"
  for ((i = 1; i <= attempts; i++)); do
    if mysql_tcp_ready; then
      return
    fi
    sleep "$interval"
  done
  cat >&2 <<EOF
无法连接 MySQL: $MYSQL_HOST:$MYSQL_PORT
请先启动 MySQL 8.0，或修改 $ENV_FILE 中的 MYSQL_HOST/MYSQL_PORT。
如果需要按当前 migration 重建开发库，可在 MySQL 可连接后执行:
  ./scripts/dev-up.sh --recreate-db
EOF
  return 2
}

mysql_client() {
  local database="${1:-}"
  local args=(
    --protocol=tcp
    -h "$MYSQL_HOST"
    -P "$MYSQL_PORT"
    -u "$MYSQL_USER"
    --default-character-set=utf8mb4
    --binary-mode
    --connect-timeout=5
  )
  if [[ -n "$database" ]]; then
    args+=("$database")
  fi
  if command -v mysql >/dev/null 2>&1; then
    MYSQL_PWD="$MYSQL_PASSWORD" mysql "${args[@]}"
    return
  fi
  if command -v docker >/dev/null 2>&1; then
    local docker_host="$MYSQL_HOST"
    local docker_args=(run --rm -i -e MYSQL_PWD)
    if [[ "$docker_host" == "127.0.0.1" || "$docker_host" == "localhost" || "$docker_host" == "::1" ]]; then
      if [[ "$(uname -s)" == "Linux" ]]; then
        docker_args+=(--network host)
      else
        docker_host="host.docker.internal"
      fi
    fi
    args[2]="$docker_host"
    echo "未找到本机 mysql 客户端，改用 Docker 镜像 ${MYSQL_CLIENT_IMAGE} 执行 mysql 客户端。" >&2
    MYSQL_PWD="$MYSQL_PASSWORD" docker "${docker_args[@]}" "$MYSQL_CLIENT_IMAGE" mysql "${args[@]}"
    return
  fi
  echo "未找到 mysql 客户端，也未找到 docker，无法执行开发库重建或数据注入。" >&2
  return 127
}

mysql_quote_identifier() {
  local value="$1"
  value="${value//\`/\`\`}"
  printf '`%s`' "$value"
}

mysql_escape_literal() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\'/\'\'}"
  printf '%s' "$value"
}

recreate_database() {
  case "$MYSQL_DATABASE" in
    ""|"mysql"|"information_schema"|"performance_schema"|"sys")
      echo "拒绝重建系统数据库: ${MYSQL_DATABASE:-<empty>}" >&2
      exit 1
      ;;
  esac

  echo "即将重建开发数据库: $MYSQL_DATABASE@$MYSQL_HOST:$MYSQL_PORT"
  echo "该操作会删除该库中的应用、构建、Freight、Stage、审计等数据。"

  local quoted_database
  quoted_database="$(mysql_quote_identifier "$MYSQL_DATABASE")"
  mysql_client <<SQL
DROP DATABASE IF EXISTS ${quoted_database};
CREATE DATABASE ${quoted_database} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
SQL
  echo "已重建开发数据库: $MYSQL_DATABASE"
}

wait_for_started_backend_ready() {
  local pid="$1"
  local ready_url="${BACKEND_LOCAL_BASE_URL}/readyz"
  local attempts="${PAAS_DEV_READY_RETRIES:-60}"
  local interval="${PAAS_DEV_READY_INTERVAL_SECONDS:-1}"
  local i code
  echo "等待后端 ready: $ready_url"
  for ((i = 1; i <= attempts; i++)); do
    if backend_ready; then
      echo "后端已 ready"
      return 0
    fi
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      set +e
      wait "$pid"
      code=$?
      set -e
      echo "后端进程已退出: pid=$pid exit_code=$code" >&2
      return "$code"
    fi
    sleep "$interval"
  done
  echo "等待后端 ready 超时: $ready_url" >&2
  return 1
}

backend_ready() {
  curl -fsS "${BACKEND_LOCAL_BASE_URL}/readyz" >/dev/null 2>&1
}

ensure_backend_stopped_for_recreate() {
  if backend_ready; then
    echo "检测到后端已在运行，拒绝重建数据库: $BACKEND_LOCAL_BASE_URL" >&2
    echo "请先停止旧的 paas-server 或 dev-up 进程，再重新执行 --recreate-db。" >&2
    return 1
  fi
}

seed_dev_data_with_mysql_client() {
  local seed_sql_file="${ROOT_DIR}/scripts/seed-data.sql"
  if [[ ! -f "$seed_sql_file" ]]; then
    echo "未找到 seed 数据文件: $seed_sql_file" >&2
    return 1
  fi

  echo "通过 MySQL 客户端注入开发测试数据: $seed_sql_file"
  mysql_client "$MYSQL_DATABASE" < "$seed_sql_file"
  echo "已注入开发测试数据"
}

wait_for_any_process_exit() {
  local i pid label running code
  while :; do
    running="$(jobs -pr)"
    for i in "${!pids[@]}"; do
      pid="${pids[$i]}"
      label="${pid_labels[$i]}"
      if ! printf '%s\n' "$running" | grep -qx "$pid"; then
        set +e
        wait "$pid"
        code=$?
        set -e
        echo "${label}进程已退出: pid=$pid exit_code=$code" >&2
        return "$code"
      fi
    done
    sleep 1
  done
}

kill_existing_dev_processes
ensure_mysql_reachable

if [[ "$RECREATE_DB" == "true" ]]; then
  export PAAS_AUTO_MIGRATE=true
  ensure_backend_stopped_for_recreate
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

if backend_ready; then
  echo "后端已在运行，跳过启动: $BACKEND_LOCAL_BASE_URL"
else
  (
    cd "$ROOT_DIR"
    go run ./cmd/paas-server
  ) &
  track_pid "后端" "$!"
  wait_for_started_backend_ready "$!"
fi

if [[ "$SEED_DEV_DATA" == "true" ]]; then
  seed_dev_data_with_mysql_client
fi

ensure_frontend_dependencies
(
  cd "$ROOT_DIR/web/console"
  npm run dev -- --host "$WEB_HOST" --port "$WEB_PORT"
) &
track_pid "前端" "$!"

wait_for_any_process_exit
cleanup
