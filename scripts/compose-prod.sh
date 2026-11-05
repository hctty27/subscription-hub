#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$root"

if [ "$#" -eq 0 ]; then
  echo "用法：sh scripts/compose-prod.sh [Docker Compose 参数和命令]" >&2
  exit 2
fi

env_file="${SUBSCRIPTION_ENV_FILE:-.env.production}"
if [ ! -f "$env_file" ]; then
  echo "未找到生产环境文件：$env_file" >&2
  exit 1
fi

if [ -f compose.production.local.yaml ]; then
  exec docker compose --env-file "$env_file" \
    -f compose.production.yaml \
    -f compose.production.local.yaml "$@"
fi

exec docker compose --env-file "$env_file" -f compose.production.yaml "$@"
