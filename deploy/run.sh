#!/usr/bin/env bash
# 启动后端服务（无 systemd 环境下的兜底方式）
set -euo pipefail

ROOT="${ROOT:-/workspace/tzwall}"
PIDFILE="${PIDFILE:-/var/run/tzwall.pid}"
LOG="${LOG:-/var/log/tzwall.log}"

if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "tzwall 已在运行 (pid $(cat "$PIDFILE"))"
  exit 0
fi

cd "$ROOT/backend"
setsid ./tzwall >> "$LOG" 2>&1 &
echo $! > "$PIDFILE"
sleep 2

if kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "已启动 tzwall (pid $(cat "$PIDFILE"))，日志：$LOG"
else
  echo "启动失败，请查看日志：$LOG"
  tail -n 20 "$LOG"
  exit 1
fi
