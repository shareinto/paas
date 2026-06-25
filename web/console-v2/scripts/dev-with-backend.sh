#!/usr/bin/env sh
set -eu

# 新版控制台本地启动脚本。
# 默认连接本机 paas-server；可通过环境变量覆盖。

API_BASE_URL="${VITE_API_BASE_URL:-${API_BASE_URL:-http://127.0.0.1:8080}}"
WEB_HOST="${WEB_HOST:-0.0.0.0}"
WEB_PORT="${WEB_PORT:-5174}"

export VITE_API_BASE_URL="$API_BASE_URL"

echo "PaaS Console V2"
echo "  Backend: $VITE_API_BASE_URL"
echo "  Web:     http://127.0.0.1:$WEB_PORT"
echo

npm run dev -- --host "$WEB_HOST" --port "$WEB_PORT"
