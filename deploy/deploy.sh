#!/usr/bin/env bash
# 构建并发布站点（后端二进制 + 前端静态资源 + Nginx + 进程启停）
set -euo pipefail

ROOT="${ROOT:-/workspace/tzwall}"
NGINX_CONF="${NGINX_CONF:-/etc/nginx/sites-enabled/tzwall}"

echo "==> 构建后端"
cd "$ROOT/backend"
go build -o tzwall ./cmd/tzwall

echo "==> 构建前端"
cd "$ROOT/frontend"
npm run build

echo "==> 配置 Nginx"
mkdir -p /etc/nginx/sites-enabled
cp "$ROOT/deploy/nginx.conf.example" "$NGINX_CONF"
sed -i "s#/opt/tzwall/frontend/dist#$ROOT/frontend/dist#" "$NGINX_CONF"
rm -f /etc/nginx/sites-enabled/default
nginx -t
if nginx -s reload 2>/dev/null; then
  echo "Nginx 已重载"
else
  nginx
  echo "Nginx 已启动"
fi

echo "==> 重启后端"
"$ROOT/deploy/stop.sh" || true
"$ROOT/deploy/run.sh"

echo "==> 健康检查"
sleep 2
curl -s http://127.0.0.1/api/health || true
echo
echo "部署完成"
