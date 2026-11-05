#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

env_file="${SUBSCRIPTION_ENV_FILE:-.env.production}"
if [ ! -f "$env_file" ]; then
  echo "未找到环境文件：$env_file" >&2
  exit 1
fi

compose() {
  docker compose --env-file "$env_file" -f compose.production.yaml "$@"
}

echo "步骤 1/3：生成新的 SQLite 一致性快照"
compose exec -T api /app/backup

echo "步骤 2/3：将加密快照上传至 Restic 仓库"
compose --profile backup run --rm --no-deps restic backup /backups-data/backups --tag subscription-hub

echo "步骤 3/3：清理远端旧快照"
compose --profile backup run --rm --no-deps restic forget   --tag subscription-hub --keep-daily 7 --keep-weekly 4 --keep-monthly 6 --prune

echo "异地备份完成"
