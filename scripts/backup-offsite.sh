#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

compose() {
  sh scripts/compose-prod.sh "$@"
}

echo "步骤 1/3：生成新的 SQLite 一致性快照"
compose exec -T api /app/backup

echo "步骤 2/3：将加密快照上传至 Restic 仓库"
compose --profile backup run --rm --no-deps restic backup /backups-data/backups --tag subscription-hub

echo "步骤 3/3：清理远端旧快照"
compose --profile backup run --rm --no-deps restic forget   --tag subscription-hub --keep-daily 7 --keep-weekly 4 --keep-monthly 6 --prune

echo "异地备份完成"
