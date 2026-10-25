#!/usr/bin/env bash
# 维护模式快捷开关
# nginx 依据标志文件直接返回静态维护页（位于 dist 之外），因此后端/前端更新或下线时仍可见。
# 用法：
#   ./maintenance.sh on [文案]   开启维护（可选自定义文案）
#   ./maintenance.sh off         关闭维护
#   ./maintenance.sh msg <文案>  仅更新维护文案（维护中立即生效）
#   ./maintenance.sh status      查看当前状态
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN_DIR="$ROOT/run"
FLAG="$RUN_DIR/MAINTENANCE"
HTML="$RUN_DIR/maintenance.html"
MSG_DEFAULT="站点正在更新维护，请稍后再来。"

# 生成自包含维护页（无外部依赖，禁缓存，每 20 秒自动重试）
write_html() {
  local msg="$1"
  local esc
  esc="$(printf '%s' "$msg" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g')"
  mkdir -p "$RUN_DIR"
  cat > "$HTML" <<EOF
<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<meta name="robots" content="noindex" />
<title>系统维护中</title>
<style>
  * { box-sizing: border-box; }
  body { margin:0; min-height:100vh; display:flex; align-items:center; justify-content:center; padding:24px;
         font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;
         background:linear-gradient(135deg,#6d28d9 0%,#4c1d95 100%); color:#fff; }
  .card { width:100%; max-width:440px; background:rgba(255,255,255,.08); border:1px solid rgba(255,255,255,.18);
          border-radius:20px; padding:36px 28px; text-align:center; }
  .icon { width:64px; height:64px; margin:0 auto 18px; border-radius:50%; background:rgba(255,255,255,.16);
          display:flex; align-items:center; justify-content:center; font-size:30px; }
  h1 { margin:0 0 12px; font-size:22px; font-weight:700; letter-spacing:.5px; }
  p  { margin:0; font-size:15px; line-height:1.7; color:rgba(255,255,255,.88); }
  .tip{ margin-top:20px; font-size:12px; color:rgba(255,255,255,.6); }
  .bar{ margin-top:22px; height:4px; border-radius:999px; background:rgba(255,255,255,.18); overflow:hidden; }
  .bar i{ display:block; height:100%; width:40%; border-radius:999px; background:#fff; animation:slide 1.6s ease-in-out infinite; }
  @keyframes slide { 0%{transform:translateX(-100%)} 100%{transform:translateX(320%)} }
  @media (max-width:480px){ .card{padding:28px 20px} h1{font-size:19px} }
</style>
</head>
<body>
  <div class="card">
    <div class="icon">🛠</div>
    <h1>系统维护中</h1>
    <p>$esc</p>
    <div class="bar"><i></i></div>
    <div class="tip">页面每 20 秒自动刷新，维护完成后即可正常访问</div>
  </div>
  <script>
    setTimeout(function(){ location.reload(); }, 20000);
  </script>
</body>
</html>
EOF
}

# 尽力同步后台配置项（数据库可能未运行；失败不影响标志文件生效）
db_set() {
  local key="$1" val="$2"
  local env_file="$ROOT/backend/.env"
  [ -f "$env_file" ] || return 0
  local pw host port user name esc
  pw="$(grep -E '^DB_PASSWORD=' "$env_file" | head -1 | cut -d= -f2- | tr -d '\r')"
  host="$(grep -E '^DB_HOST=' "$env_file" | head -1 | cut -d= -f2- | tr -d '\r')"; host="${host:-127.0.0.1}"
  port="$(grep -E '^DB_PORT=' "$env_file" | head -1 | cut -d= -f2- | tr -d '\r')"; port="${port:-5433}"
  user="$(grep -E '^DB_USER=' "$env_file" | head -1 | cut -d= -f2- | tr -d '\r')"; user="${user:-tzwall}"
  name="$(grep -E '^DB_NAME=' "$env_file" | head -1 | cut -d= -f2- | tr -d '\r')"; name="${name:-tzwall}"
  esc="${val//\'/\'\'}"
  if ! PGPASSWORD="$pw" psql -h "$host" -p "$port" -U "$user" -d "$name" -v ON_ERROR_STOP=1 -c \
      "INSERT INTO system_configs (key, value, updated_at) VALUES ('$key', '$esc', now()) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now();" >/dev/null 2>&1; then
    echo "  (提示：后台配置项 $key 同步失败——数据库可能未运行；标志文件已生效)"
  fi
}

case "${1:-}" in
  on)
    MSG="${2:-$MSG_DEFAULT}"
    write_html "$MSG"
    printf '%s\n' "$(date '+%F %T')" > "$FLAG"
    db_set "maintenance_message" "$MSG"
    db_set "maintenance_enabled" "true"
    echo "✅ 已开启维护模式：全站（除 /api、/admin）返回维护页"
    echo "   文案：$MSG"
    echo "   关闭：$0 off"
    ;;
  off)
    rm -f "$FLAG"
    db_set "maintenance_enabled" "false"
    echo "✅ 已关闭维护模式，站点恢复正常访问"
    ;;
  msg)
    MSG="${2:-}"
    if [ -z "$MSG" ]; then echo "用法：$0 msg <文案>" >&2; exit 1; fi
    write_html "$MSG"
    db_set "maintenance_message" "$MSG"
    if [ -f "$FLAG" ]; then echo "✅ 维护文案已更新（维护中，立即生效）"; else echo "✅ 维护文案已保存（当前未开启维护）"; fi
    ;;
  status)
    if [ -f "$FLAG" ]; then echo "维护模式：开启（标志文件 $FLAG 存在）"; else echo "维护模式：关闭"; fi
    [ -f "$HTML" ] && echo "维护页：$HTML（$(wc -c < "$HTML") 字节）" || echo "维护页：缺失（开启时会自动生成）"
    ENVF="$ROOT/backend/.env"
    if [ -f "$ENVF" ]; then
      PW="$(grep -E '^DB_PASSWORD=' "$ENVF" | head -1 | cut -d= -f2- | tr -d '\r')"
      H="$(grep -E '^DB_HOST=' "$ENVF" | head -1 | cut -d= -f2- | tr -d '\r')"; H="${H:-127.0.0.1}"
      P="$(grep -E '^DB_PORT=' "$ENVF" | head -1 | cut -d= -f2- | tr -d '\r')"; P="${P:-5433}"
      U="$(grep -E '^DB_USER=' "$ENVF" | head -1 | cut -d= -f2- | tr -d '\r')"; U="${U:-tzwall}"
      N="$(grep -E '^DB_NAME=' "$ENVF" | head -1 | cut -d= -f2- | tr -d '\r')"; N="${N:-tzwall}"
      echo "后台配置：$(PGPASSWORD="$PW" psql -h "$H" -p "$P" -U "$U" -d "$N" -t -A -c "SELECT key||'='||value FROM system_configs WHERE key IN ('maintenance_enabled','maintenance_message') ORDER BY key;" 2>/dev/null | tr '\n' ' ' || echo '(数据库不可用)')"
    fi
    ;;
  *)
    echo "用法：$0 on [文案] | off | msg <文案> | status" >&2
    exit 1
    ;;
esac
