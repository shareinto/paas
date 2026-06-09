#!/usr/bin/env bash
set -euo pipefail

missing=0

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "缺少命令: $name"
    missing=1
  else
    echo "命令可用: $name"
  fi
}

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    echo "缺少环境变量: $name"
    missing=1
  else
    echo "环境变量已设置: $name"
  fi
}

require_command go
require_command npm
require_command mvn
require_command curl
require_command kubectl

require_env PAAS_URL
require_env MYSQL_HOST
require_env MYSQL_PORT
require_env MYSQL_DATABASE
require_env MYSQL_USER
require_env MYSQL_PASSWORD
require_env GITLAB_BASE_URL
require_env GITLAB_TOKEN
require_env JENKINS_BASE_URL
require_env JENKINS_USERNAME
require_env JENKINS_TOKEN
require_env REGISTRY_ENDPOINT
require_env PAAS_CLUSTER_ID
require_env PAAS_AGENT_TOKEN

if [ "$missing" -ne 0 ]; then
  echo "预检失败：请补齐缺失命令或环境变量。"
  exit 1
fi

echo "预检通过：未打印任何凭据值。"

