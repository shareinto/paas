#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

IMAGE_REPOSITORY="${PAAS_AGENT_IMAGE_REPOSITORY:-cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/paas-agent}"
IMAGE_PLATFORM="${PAAS_AGENT_IMAGE_PLATFORM:-linux/arm64}"
DOCKERFILE="${PAAS_AGENT_DOCKERFILE:-$ROOT_DIR/deploy/paas-agent/Dockerfile}"
CONTEXT_DIR="${PAAS_AGENT_CONTEXT_DIR:-$ROOT_DIR}"
IMAGE_TAG="${PAAS_AGENT_IMAGE_TAG:-}"
GO_IMAGE="${PAAS_AGENT_GO_IMAGE:-golang:1.25}"
RUNTIME_IMAGE="${PAAS_AGENT_RUNTIME_IMAGE:-gcr.io/distroless/static-debian12:nonroot}"

usage() {
  cat <<'EOF'
Usage:
  scripts/build-paas-agent-image.sh [options]

Options:
  --tag <tag>          指定镜像 tag。默认: YYYYMMDD-<git短SHA>-arm64
  --repository <repo>  指定镜像仓库。
  --platform <value>   指定 buildx platform。默认: linux/arm64
  -h, --help           显示帮助。

Environment:
  PAAS_AGENT_IMAGE_REPOSITORY
  PAAS_AGENT_IMAGE_TAG
  PAAS_AGENT_IMAGE_PLATFORM
  PAAS_AGENT_DOCKERFILE
  PAAS_AGENT_CONTEXT_DIR
  PAAS_AGENT_GO_IMAGE
  PAAS_AGENT_RUNTIME_IMAGE

Example:
  ./scripts/build-paas-agent-image.sh
  PAAS_AGENT_IMAGE_TAG=20260612-test-arm64 ./scripts/build-paas-agent-image.sh
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag)
      IMAGE_TAG="${2:-}"
      shift 2
      ;;
    --repository)
      IMAGE_REPOSITORY="${2:-}"
      shift 2
      ;;
    --platform)
      IMAGE_PLATFORM="${2:-}"
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

if [[ -z "$IMAGE_TAG" ]]; then
  short_sha="$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
  arch_suffix="${IMAGE_PLATFORM##*/}"
  IMAGE_TAG="$(date +%Y%m%d)-${short_sha}-${arch_suffix}"
fi

if [[ -z "$IMAGE_REPOSITORY" ]]; then
  echo "镜像仓库不能为空" >&2
  exit 2
fi
if [[ -z "$IMAGE_TAG" ]]; then
  echo "镜像 tag 不能为空" >&2
  exit 2
fi
if [[ ! -f "$DOCKERFILE" ]]; then
  echo "Dockerfile 不存在: $DOCKERFILE" >&2
  exit 2
fi

image="${IMAGE_REPOSITORY}:${IMAGE_TAG}"

echo "构建并推送 paas-agent 镜像:"
echo "  image: $image"
echo "  platform: $IMAGE_PLATFORM"
echo "  dockerfile: $DOCKERFILE"
echo "  go image: $GO_IMAGE"
echo "  runtime image: $RUNTIME_IMAGE"

docker buildx build \
  --platform "$IMAGE_PLATFORM" \
  --build-arg "GO_IMAGE=$GO_IMAGE" \
  --build-arg "RUNTIME_IMAGE=$RUNTIME_IMAGE" \
  -f "$DOCKERFILE" \
  -t "$image" \
  --push \
  "$CONTEXT_DIR"

echo "镜像已推送: $image"
echo "更新 Helm values 时使用:"
echo "  image.repository: $IMAGE_REPOSITORY"
echo "  image.tag: $IMAGE_TAG"
