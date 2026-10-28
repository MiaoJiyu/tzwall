#!/usr/bin/env bash
# 每日备份 PostgreSQL 到本地磁盘，保留 7 份（不额外购买备份存储）。
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/var/backups/tzwall}"
DB_NAME="${DB_NAME:-tzwall}"
DB_USER="${DB_USER:-tzwall}"
KEEP=7

mkdir -p "$BACKUP_DIR"
TS="$(date +%Y%m%d_%H%M%S)"

# 备份数据库（显式指定端口，PG 实际监听 5433 而非默认 5432）
PGPASSWORD="$DB_PASSWORD" pg_dump -U "$DB_USER" -h "${DB_HOST:-127.0.0.1}" -p "${DB_PORT:-5432}" "$DB_NAME" | gzip > "$BACKUP_DIR/db_$TS.sql.gz"

# 可选：本地图片目录纳入备份（默认不备份原图，见系统配置）
if [ "${BACKUP_IMAGES:-0}" = "1" ]; then
  tar -czf "$BACKUP_DIR/uploads_$TS.tar.gz" -C /opt/tzwall/backend/data uploads 2>/dev/null || true
fi

# 仅保留最近 KEEP 份
ls -tp "$BACKUP_DIR"/db_*.sql.gz 2>/dev/null | tail -n +$((KEEP + 1)) | xargs -r rm -f

echo "backup done: $BACKUP_DIR/db_$TS.sql.gz"
