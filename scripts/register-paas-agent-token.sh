#!/usr/bin/env bash
set -euo pipefail

PAAS_URL="${PAAS_URL:-http://122.152.196.135:18080}"
ACTOR_ID="${PAAS_ACTOR_ID:-usr_admin}"
CLUSTER_NAME="${PAAS_AGENT_CLUSTER_NAME:-llt-arm-cluster}"
CLUSTER_REGION="${PAAS_AGENT_CLUSTER_REGION:-llt}"
VALUES_FILE="${PAAS_AGENT_VALUES_FILE:-/windows/go/src/github.com/shareinto/manifests/paas-agent/values.yaml}"
UPDATE_VALUES=false

usage() {
  cat <<'EOF'
Usage:
  scripts/register-paas-agent-token.sh [options]

Options:
  --paas-url <url>        paas-server 地址。默认: http://122.152.196.135:18080
  --actor-id <id>         注册集群使用的 actor ID。默认: usr_admin
  --cluster-name <name>   集群名称。默认: llt-arm-cluster
  --region <region>       集群 region。默认: llt
  --update-values         同步更新 manifests/paas-agent/values.yaml 中的 clusterID 和 token。
  --values-file <path>    指定要更新的 values.yaml。
  -h, --help              显示帮助。

Environment:
  PAAS_URL
  PAAS_ACTOR_ID
  PAAS_AGENT_CLUSTER_NAME
  PAAS_AGENT_CLUSTER_REGION
  PAAS_AGENT_VALUES_FILE

Example:
  ./scripts/register-paas-agent-token.sh
  ./scripts/register-paas-agent-token.sh --update-values
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --paas-url)
      PAAS_URL="${2:-}"
      shift 2
      ;;
    --actor-id)
      ACTOR_ID="${2:-}"
      shift 2
      ;;
    --cluster-name)
      CLUSTER_NAME="${2:-}"
      shift 2
      ;;
    --region)
      CLUSTER_REGION="${2:-}"
      shift 2
      ;;
    --update-values)
      UPDATE_VALUES=true
      shift
      ;;
    --values-file)
      VALUES_FILE="${2:-}"
      shift 2
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

for command_name in curl jq; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "缺少命令: $command_name" >&2
    exit 2
  fi
done

if [[ -z "$PAAS_URL" ]]; then
  echo "paas-server 地址不能为空" >&2
  exit 2
fi

PAAS_URL="${PAAS_URL%/}"

tenants_json="$(curl -fsS "$PAAS_URL/api/tenants?page=1&page_size=1")"
tenant_id="$(printf '%s' "$tenants_json" | jq -r '.items[0].id // empty')"
if [[ -z "$tenant_id" ]]; then
  echo "未查询到 tenant，无法注册 paas-agent 集群" >&2
  exit 1
fi

role_binding_payload="$(jq -n \
  --arg subject_id "$ACTOR_ID" \
  '{subject_type:"user", subject_id:$subject_id, role_id:"platform_admin", scope_kind:"platform", scope_id:""}')"
role_binding_code="$(curl -sS -o /tmp/paas-agent-role-binding.json -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "$role_binding_payload" \
  "$PAAS_URL/api/role-bindings")"
if [[ "$role_binding_code" != "201" && "$role_binding_code" != "409" ]]; then
  echo "创建平台管理员 RoleBinding 失败，HTTP $role_binding_code" >&2
  jq '.error // .' /tmp/paas-agent-role-binding.json >&2 || true
  exit 1
fi

register_payload="$(jq -n \
  --arg actor_id "$ACTOR_ID" \
  --arg tenant_id "$tenant_id" \
  --arg cluster_name "$CLUSTER_NAME" \
  --arg region "$CLUSTER_REGION" \
  '{actor:{Type:"user", ID:$actor_id}, tenant_id:$tenant_id, name:$cluster_name, region:$region}')"
register_json="$(curl -fsS \
  -H 'Content-Type: application/json' \
  -d "$register_payload" \
  "$PAAS_URL/api/clusters")"

cluster_id="$(printf '%s' "$register_json" | jq -r '.cluster.id // empty')"
agent_token="$(printf '%s' "$register_json" | jq -r '.agent_token // empty')"
if [[ -z "$cluster_id" || -z "$agent_token" ]]; then
  echo "注册响应缺少 cluster.id 或 agent_token" >&2
  exit 1
fi

cat <<EOF
tenant_id=$tenant_id
cluster_id=$cluster_id
agent_token=$agent_token
EOF

if [[ "$UPDATE_VALUES" == "true" ]]; then
  if [[ ! -f "$VALUES_FILE" ]]; then
    echo "values 文件不存在: $VALUES_FILE" >&2
    exit 2
  fi
  values_tmp="$(mktemp)"
  awk -v cluster_id="$cluster_id" -v agent_token="$agent_token" '
    /^  clusterID:/ { print "  clusterID: " cluster_id; next }
    /^  token:/ { print "  token: " agent_token; next }
    { print }
  ' "$VALUES_FILE" >"$values_tmp"
  mv "$values_tmp" "$VALUES_FILE"
  echo "已更新 values 文件: $VALUES_FILE"
fi

cat <<EOF

下一步:
  1. 将 config.clusterID 更新为: $cluster_id
  2. 将 secret.token 更新为上方 agent_token
  3. 提交并推送 /windows/go/src/github.com/shareinto/manifests/paas-agent
  4. 刷新 Argo CD:
     kubectl -n argocd annotate application paas-agent argocd.argoproj.io/refresh=hard --overwrite
EOF
