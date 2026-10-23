#!/usr/bin/env bash
# 快捷删除投稿脚本
# 仅删除指定投稿及其从属数据（图片/视频/修订/标签/评论/表态/举报/机审/关联投票），
# 不会删除用户、账号或其他投稿。
#
# 用法：
#   ./delete-submissions.sh 57 58 59        # 交互确认后删除
#   ./delete-submissions.sh -y 57 58 59     # 跳过确认
#   ./delete-submissions.sh -n 57           # 仅预览 dry-run
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$ROOT/backend/.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "✗ 未找到 $ENV_FILE" >&2
  exit 1
fi

get_env() {
  grep -E "^$1=" "$ENV_FILE" | head -1 | cut -d= -f2- | tr -d '\r'
}

DB_HOST="${DB_HOST:-$(get_env DB_HOST)}"; DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-$(get_env DB_PORT)}"; DB_PORT="${DB_PORT:-5433}"
DB_USER="${DB_USER:-$(get_env DB_USER)}"; DB_USER="${DB_USER:-tzwall}"
DB_NAME="${DB_NAME:-$(get_env DB_NAME)}"; DB_NAME="${DB_NAME:-tzwall}"
DB_PASSWORD="$(get_env DB_PASSWORD)"

if [ -z "$DB_PASSWORD" ]; then
  echo "✗ 未读取到 DB_PASSWORD（请检查 $ENV_FILE）" >&2
  exit 1
fi

export PGPASSWORD="$DB_PASSWORD"
PSQL=(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1)

YES=0
DRY=0
IDS=()
for a in "$@"; do
  case "$a" in
    -y|--yes) YES=1 ;;
    -n|--dry-run) DRY=1 ;;
    -h|--help)
      echo "用法: $0 [-y|--yes] [-n|--dry-run] <投稿ID...>"
      exit 0 ;;
    -*)
      echo "✗ 未知参数: $a" >&2; exit 1 ;;
    *)
      if ! [[ "$a" =~ ^[0-9]+$ ]]; then
        echo "✗ 无效投稿ID: $a" >&2; exit 1
      fi
      IDS+=("$a") ;;
  esac
done

if [ ${#IDS[@]} -eq 0 ]; then
  echo "用法: $0 [-y|--yes] [-n|--dry-run] <投稿ID...>"
  exit 1
fi

IDS_CSV="$(IFS=,; echo "${IDS[*]}")"

echo "== 目标投稿: $IDS_CSV =="
"${PSQL[@]}" -c "SELECT id, user_id, left(title,40) AS title, status FROM submissions WHERE id IN ($IDS_CSV) ORDER BY id;"

FOUND="$("${PSQL[@]}" -t -A -c "SELECT count(*) FROM submissions WHERE id IN ($IDS_CSV);" | tr -d ' ')"
if [ "${FOUND:-0}" -eq 0 ]; then
  echo "没有找到这些投稿，未做任何更改。"
  exit 0
fi

if [ "$DRY" = "1" ]; then
  echo "-- 关联数据统计（dry-run，不删除）--"
  for t in submission_images submission_videos submission_revisions submission_tags comments submission_polls; do
    echo "  $t = $("${PSQL[@]}" -t -A -c "SELECT count(*) FROM $t WHERE submission_id IN ($IDS_CSV);" | tr -d ' ')"
  done
  echo "  reactions = $("${PSQL[@]}" -t -A -c "SELECT count(*) FROM reactions WHERE target_type='submission' AND target_id IN ($IDS_CSV);" | tr -d ' ')"
  echo "  reports = $("${PSQL[@]}" -t -A -c "SELECT count(*) FROM reports WHERE target_type='submission' AND target_id IN ($IDS_CSV);" | tr -d ' ')"
  echo "  moderation_tasks = $("${PSQL[@]}" -t -A -c "SELECT count(*) FROM moderation_tasks WHERE biz_id IN (SELECT 'sub-'||id FROM submissions WHERE id IN ($IDS_CSV)) OR object_key IN (SELECT storage_key FROM submission_images WHERE submission_id IN ($IDS_CSV));" | tr -d ' ')"
  exit 0
fi

if [ "$YES" != "1" ]; then
  read -r -p "确认删除以上投稿及其从属数据？用户不会被删除 [y/N] " ans
  case "$ans" in
    y|Y|yes|YES) ;;
    *) echo "已取消。"; exit 0 ;;
  esac
fi

echo "== 执行删除（事务）=="
"${PSQL[@]}" <<SQL
BEGIN;
DELETE FROM moderation_tasks
 WHERE biz_id IN (SELECT 'sub-'||id FROM submissions WHERE id IN ($IDS_CSV))
    OR object_key IN (SELECT storage_key FROM submission_images WHERE submission_id IN ($IDS_CSV));
DELETE FROM submission_polls     WHERE submission_id IN ($IDS_CSV);
DELETE FROM submission_images    WHERE submission_id IN ($IDS_CSV);
DELETE FROM submission_videos    WHERE submission_id IN ($IDS_CSV);
DELETE FROM submission_revisions WHERE submission_id IN ($IDS_CSV);
DELETE FROM submission_tags      WHERE submission_id IN ($IDS_CSV);
DELETE FROM comments             WHERE submission_id IN ($IDS_CSV);
DELETE FROM reactions            WHERE target_type='submission' AND target_id IN ($IDS_CSV);
DELETE FROM reports              WHERE target_type='submission' AND target_id IN ($IDS_CSV);
DELETE FROM submissions          WHERE id IN ($IDS_CSV);
COMMIT;
SQL

LEFT="$("${PSQL[@]}" -t -A -c "SELECT count(*) FROM submissions WHERE id IN ($IDS_CSV);" | tr -d ' ')"
echo "== 完成：目标投稿剩余 ${LEFT:-0} 篇（用户未受影响）=="
