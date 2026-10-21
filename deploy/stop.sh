#!/usr/bin/env bash
# 停止后端服务
set -euo pipefail

PIDFILE="${PIDFILE:-/var/run/tzwall.pid}"

if [ ! -f "$PIDFILE" ]; then
  echo "未找到 PID 文件，服务可能未运行"
  exit 0
fi

PID="$(cat "$PIDFILE")"
if kill -0 "$PID" 2>/dev/null; then
  kill "$PID"
  echo "已停止 tzwall (pid $PID)"
else
  echo "进程 $PID 不存在"
fi
rm -f "$PIDFILE"
