#!/usr/bin/env bash
# 环境初始化（幂等）：安装 PostgreSQL / Redis / Nginx / 中文字体，并建库建用户
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

DB_NAME="${DB_NAME:-tzwall}"
DB_USER="${DB_USER:-tzwall}"
DB_PASSWORD="${DB_PASSWORD:-tzwall_dev_pwd}"

echo "==> 安装依赖"
apt-get update -qq
apt-get install -y -qq postgresql redis-server nginx fonts-noto-cjk

echo "==> 启动 PostgreSQL"
pg_ctlcluster 16 main start 2>/dev/null || service postgresql start || true
sleep 3

echo "==> 创建数据库与用户（已存在则跳过）"
su postgres -c "psql -tAc \"SELECT 1 FROM pg_roles WHERE rolname='$DB_USER'\"" | grep -q 1 ||
  su postgres -c "psql -c \"CREATE USER $DB_USER WITH PASSWORD '$DB_PASSWORD';\""
su postgres -c "psql -tAc \"SELECT 1 FROM pg_database WHERE datname='$DB_NAME'\"" | grep -q 1 ||
  su postgres -c "psql -c \"CREATE DATABASE $DB_NAME OWNER $DB_USER;\""

echo "==> 启动 Redis"
redis-server --daemonize yes --save '' --appendonly no 2>/dev/null || service redis-server start || true

echo "==> 完成"
pg_lsclusters | tail -2
