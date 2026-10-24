#!/usr/bin/env bash
# 快捷删除用户脚本
# 删除指定用户及其全部关联数据（投稿及图片/视频/标签/评论/机审、投票、评论、表态、举报、
# 私聊会话与消息、工单与回复、公告、处罚、权限覆盖、访问日志），不影响其他用户。
#
# 用法：
#   ./delete-user.sh <用户名或用户ID>              # 交互确认后删除
#   ./delete-user.sh <用户名或用户ID> -y           # 跳过确认
#   ./delete-user.sh <用户名或用户ID> -n           # 仅预览 dry-run
#   ./delete-user.sh <用户名或用户ID> --force      # 允许删除管理员/超级管理员
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
FORCE=0
KEY=""
for a in "$@"; do
  case "$a" in
    -y|--yes) YES=1 ;;
    -n|--dry-run) DRY=1 ;;
    -f|--force) FORCE=1 ;;
    -h|--help)
      echo "用法: $0 <用户名或用户ID> [-y|--yes] [-n|--dry-run] [-f|--force]"
      exit 0 ;;
    -*)
      echo "✗ 未知参数: $a" >&2; exit 1 ;;
    *)
      if [ -n "$KEY" ]; then echo "✗ 一次只支持一个用户参数" >&2; exit 1; fi
      KEY="$a" ;;
  esac
done

if [ -z "$KEY" ]; then
  echo "用法: $0 <用户名或用户ID> [-y|--yes] [-n|--dry-run] [-f|--force]"
  exit 1
fi

# 解析目标用户（纯数字按 ID；否则按用户名，单引号翻倍转义）
if [[ "$KEY" =~ ^[0-9]+$ ]]; then
  WHERESQL="id = $KEY"
else
  ESC="${KEY//\'/\'\'}"
  WHERESQL="username = '$ESC'"
fi
INFO="$("${PSQL[@]}" -t -A -F '|' -c "SELECT id, username, role, coalesce(email,'') FROM users WHERE $WHERESQL;")"

if [ -z "$INFO" ]; then
  echo "✗ 未找到用户: $KEY"
  exit 1
fi

UID_="$(echo "$INFO" | head -1 | cut -d'|' -f1)"
UNAME="$(echo "$INFO" | head -1 | cut -d'|' -f2)"
UROLE="$(echo "$INFO" | head -1 | cut -d'|' -f3)"

if ! [[ "$UID_" =~ ^[0-9]+$ ]]; then
  echo "✗ 解析用户 ID 失败: $INFO" >&2
  exit 1
fi

echo "== 目标用户 =="
"${PSQL[@]}" -c "SELECT id, username, role, email FROM users WHERE id = $UID_;"

if { [ "$UROLE" = "super_admin" ] || [ "$UROLE" = "admin" ]; } && [ "$FORCE" != "1" ]; then
  echo "✗ 该用户是 $UROLE（管理员）。如确认删除，请追加 --force。"
  exit 1
fi

count() {
  "${PSQL[@]}" -t -A -c "$1" | tr -d ' '
}

if [ "$DRY" = "1" ]; then
  echo "-- 关联数据统计（dry-run，不删除）--"
  echo "  投稿        = $(count "SELECT count(*) FROM submissions WHERE user_id=$UID_;")"
  echo "  投票        = $(count "SELECT count(*) FROM polls WHERE user_id=$UID_;")"
  echo "  评论        = $(count "SELECT count(*) FROM comments WHERE user_id=$UID_;")"
  echo "  表态        = $(count "SELECT count(*) FROM reactions WHERE user_id=$UID_;")"
  echo "  举报        = $(count "SELECT count(*) FROM reports WHERE user_id=$UID_;")"
  echo "  私聊会话    = $(count "SELECT count(*) FROM conversations WHERE user_low=$UID_ OR user_high=$UID_;")"
  echo "  工单        = $(count "SELECT count(*) FROM tickets WHERE user_id=$UID_;")"
  echo "  公告        = $(count "SELECT count(*) FROM announcements WHERE created_by=$UID_;")"
  echo "  处罚        = $(count "SELECT count(*) FROM punishments WHERE user_id=$UID_;")"
  echo "  权限覆盖    = $(count "SELECT count(*) FROM permission_grants WHERE user_id=$UID_;")"
  echo "  访问日志    = $(count "SELECT count(*) FROM access_logs WHERE user_id=$UID_;")"
  exit 0
fi

if [ "$YES" != "1" ]; then
  read -r -p "确认删除用户「$UNAME」(id=$UID_) 及其全部关联数据？其他用户不受影响 [y/N] " ans
  case "$ans" in
    y|Y|yes|YES) ;;
    *) echo "已取消。"; exit 0 ;;
  esac
fi

echo "== 执行删除（事务）=="
"${PSQL[@]}" <<SQL
BEGIN;
DELETE FROM submission_polls WHERE submission_id IN (SELECT id FROM submissions WHERE user_id = $UID_) OR poll_id IN (SELECT id FROM polls WHERE user_id = $UID_);
DELETE FROM moderation_tasks WHERE biz_id IN (SELECT 'sub-'||id FROM submissions WHERE user_id = $UID_) OR object_key IN (SELECT storage_key FROM submission_images WHERE submission_id IN (SELECT id FROM submissions WHERE user_id = $UID_));
DELETE FROM submission_images WHERE submission_id IN (SELECT id FROM submissions WHERE user_id = $UID_);
DELETE FROM submission_videos WHERE submission_id IN (SELECT id FROM submissions WHERE user_id = $UID_);
DELETE FROM submission_revisions WHERE submission_id IN (SELECT id FROM submissions WHERE user_id = $UID_);
DELETE FROM submission_tags WHERE submission_id IN (SELECT id FROM submissions WHERE user_id = $UID_);
DELETE FROM comments WHERE user_id = $UID_ OR submission_id IN (SELECT id FROM submissions WHERE user_id = $UID_);
DELETE FROM reactions WHERE user_id = $UID_ OR (target_type='submission' AND target_id IN (SELECT id FROM submissions WHERE user_id = $UID_));
DELETE FROM reports WHERE user_id = $UID_ OR (target_type='submission' AND target_id IN (SELECT id FROM submissions WHERE user_id = $UID_));
DELETE FROM submissions WHERE user_id = $UID_;
DELETE FROM poll_votes WHERE user_id = $UID_ OR poll_id IN (SELECT id FROM polls WHERE user_id = $UID_);
DELETE FROM poll_options WHERE poll_id IN (SELECT id FROM polls WHERE user_id = $UID_);
DELETE FROM polls WHERE user_id = $UID_;
DELETE FROM messages WHERE sender_id = $UID_ OR conversation_id IN (SELECT id FROM conversations WHERE user_low = $UID_ OR user_high = $UID_);
DELETE FROM conversations WHERE user_low = $UID_ OR user_high = $UID_;
DELETE FROM ticket_replies WHERE user_id = $UID_ OR ticket_id IN (SELECT id FROM tickets WHERE user_id = $UID_);
DELETE FROM tickets WHERE user_id = $UID_;
DELETE FROM announcement_confirms WHERE user_id = $UID_ OR announcement_id IN (SELECT id FROM announcements WHERE created_by = $UID_);
DELETE FROM announcements WHERE created_by = $UID_;
DELETE FROM punishments WHERE user_id = $UID_;
DELETE FROM permission_grants WHERE user_id = $UID_;
DELETE FROM access_logs WHERE user_id = $UID_;
DELETE FROM users WHERE id = $UID_;
COMMIT;
SQL

LEFT="$(count "SELECT count(*) FROM users WHERE id=$UID_;")"
echo "== 完成：用户「$UNAME」剩余 ${LEFT} 条（其他用户未受影响）=="
