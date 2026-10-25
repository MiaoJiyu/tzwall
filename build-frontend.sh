#!/usr/bin/env bash
# 快捷前端构建脚本（限制 Node 堆内存，避免构建 OOM）
# 用法：
#   ./build-frontend.sh                 # 默认堆内存上限 512MB
#   FE_MAX_OLD_SPACE=768 ./build-frontend.sh   # 自定义上限（MB）
# 说明：构建会把 frontend/public/* 一并复制到 dist/，故新增 public 文件后跑一次即可生效。
# 提示：上线更新前建议先开启维护模式，避免用户看到构建中的页面：
#   /opt/tzwall/maintenance.sh on "正在更新，请稍候"   # 开启（全站显示维护页）
#   /opt/tzwall/maintenance.sh off                    # 更新完成后关闭
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FE="$ROOT/frontend"

# Node 堆内存上限（MB），默认 512，可用环境变量覆盖
MAX_OLD_SPACE="${FE_MAX_OLD_SPACE:-512}"
export NODE_OPTIONS="--max-old-space-size=${MAX_OLD_SPACE}"

cd "$FE"

echo "==> 前端构建开始 ($(date '+%H:%M:%S'))  堆内存上限=${MAX_OLD_SPACE}MB"
start=$(date +%s)

# 优先使用项目本地 vite，缺失时回退到 node 直接调用
if [ -x "node_modules/.bin/vite" ]; then
  node_modules/.bin/vite build
else
  node node_modules/vite/bin/vite.js build
fi

end=$(date +%s)
echo "==> 构建成功 ($(date '+%H:%M:%S'))  耗时 $((end - start))s"
echo "==> 产物目录: $FE/dist"
