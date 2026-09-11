#!/usr/bin/env bash
# 本地构建镜像的快速脚本，避免在命令行反复输入构建参数。
# 默认打本地专用 tag（sub2api-local），避免与线上 weishaw/sub2api:latest 混淆。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

IMAGE="${SUB2API_IMAGE:-sub2api-local:latest}"

docker build -t "${IMAGE}" \
    --build-arg GOPROXY=https://goproxy.cn,direct \
    --build-arg GOSUMDB=sum.golang.google.cn \
    -f "${REPO_ROOT}/Dockerfile" \
    "${REPO_ROOT}"

SHA="$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || true)"
if [ -n "${SHA}" ]; then
    TAG="${IMAGE%:*}:${SHA}"
    docker tag "${IMAGE}" "${TAG}"
    echo "Built ${IMAGE} (also tagged ${TAG})"
else
    echo "Built ${IMAGE}"
fi
