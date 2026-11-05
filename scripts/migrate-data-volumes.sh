#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$root"

mode="${1:-}"
case "$mode" in
  production)
    destination="$root/data"
    old_volume="${OLD_VOLUME:-subscription-hub-prod_subscription_data}"
    ;;
  development)
    destination="$root/data-dev"
    old_volume="${OLD_VOLUME:-${COMPOSE_PROJECT_NAME:-subscription-hub}_subscription_data}"
    ;;
  *)
    echo "用法：sh scripts/migrate-data-volumes.sh production|development" >&2
    exit 2
    ;;
esac

if [ -L "$destination" ] || { [ -e "$destination" ] && [ ! -d "$destination" ]; }; then
  echo "目标路径不是普通目录：$destination" >&2
  exit 1
fi

if [ -d "$destination" ]; then
  # 容器可能以 UID 10001 拥有目录；通过 Docker 检查，任何检查失败都视为不安全。
  if ! docker run --rm \
    --mount "type=bind,source=$destination,target=/check,readonly" \
    alpine:3.22 sh -eu -c 'first="$(find /check -mindepth 1 -print -quit)"; test -z "$first"'; then
    echo "目标目录非空或无法读取，禁止覆盖：$destination" >&2
    echo "请先确认现有数据、备份和旧命名卷的内容，勿直接删除任何数据库。" >&2
    exit 1
  fi
fi

if docker volume inspect "$old_volume" >/dev/null 2>&1; then
  echo "发现旧命名卷：$old_volume"

  if [ "$mode" = production ]; then
    container_id="$(sh scripts/compose-prod.sh ps -q api)"
  else
    container_id="$(docker compose -f compose.yaml ps -q backend)"
  fi

  if [ -n "$container_id" ] && [ "$(docker inspect -f '{{.State.Running}}' "$container_id")" = "true" ]; then
    echo "创建一致性 SQLite 快照..."
    docker exec "$container_id" /app/backup
  fi

  echo "停止旧应用的数据库写入..."
  if [ "$mode" = production ]; then
    sh scripts/compose-prod.sh stop api
  else
    docker compose -f compose.yaml stop backend
  fi

  if [ -n "$(docker ps -q --filter "volume=$old_volume")" ]; then
    echo "旧数据卷仍被运行中的容器挂载，迁移已中止：$old_volume" >&2
    exit 1
  fi
fi

mkdir -p "$destination"
if docker volume inspect "$old_volume" >/dev/null 2>&1; then
  echo "从旧命名卷复制数据到 $destination ..."
  docker run --rm \
    --mount "type=volume,source=$old_volume,target=/from,readonly" \
    --mount "type=bind,source=$destination,target=/to" \
    alpine:3.22 sh -eu -c '
      cp -a /from/. /to/
      chown -R 10001:10001 /to
      chmod 750 /to
    '
  echo "复制完成。旧命名卷保持原样，验证成功前请勿删除。"
else
  docker run --rm \
    --mount "type=bind,source=$destination,target=/to" \
    alpine:3.22 sh -eu -c 'chown 10001:10001 /to && chmod 750 /to'
  echo "未找到旧命名卷，已初始化空数据目录：$destination"
fi

echo "数据目录准备完成。请核对数据后再启动对应的 Docker Compose。"
