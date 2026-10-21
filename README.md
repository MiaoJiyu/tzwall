# 泰兴中学校园墙

面向泰兴中学师生的校园社区网站：投稿、浏览、评论、点赞/点踩、举报（举报在阶段二），前后端分离。
本仓库为**阶段一**交付：可运行骨架、数据库迁移与种子、注册登录、投稿/评论/点赞、图片代理水印、基础后台与部署文件。
阶段二（权限矩阵、处罚全流程、审核与举报、日志与配置中心）已预留数据模型与接口边界。

## 技术栈

- 后端：Go 1.21 + Gin + GORM + PostgreSQL + Redis（可选）
- 前端：Vue 3 + Vite + TypeScript + Element Plus + Tailwind CSS + Pinia + Vue Router
- 部署：Nginx + systemd（不使用 Docker）；Ubuntu amd64 裸机，EdgeOne 回源

## 目录结构

```
backend/         Go 后端（config / model / service / handler / middleware / storage / moderation / mailer / cache / watermark / bootstrap）
frontend/        Vue3 前端（views / layouts / components / stores / api）
deploy/          systemd unit、Nginx 示例、备份脚本
```

## 一、后端

### 1. 初始化数据库与依赖

```bash
sudo apt-get update
sudo apt-get install -y postgresql redis-server nginx fonts-noto-cjk
sudo -u postgres psql -c "CREATE USER tzwall WITH PASSWORD 'tzwall';"
sudo -u postgres psql -c "CREATE DATABASE tzwall OWNER tzwall;"
# pg_trgm 扩展会在迁移时自动创建（用于搜索）
```

### 2. 配置

```bash
cd backend
cp .env.example .env
# 编辑 .env：至少填写 DB_PASSWORD、JWT_SECRET、AES_KEY_B64
```

生成密钥示例：

```bash
# JWT 随机串
openssl rand -hex 32
# AES 32 字节 base64
openssl rand -base64 32
```

### 3. 构建与运行

```bash
cd backend
go build -o tzwall ./cmd/tzwall
./tzwall            # 首次启动自动迁移表结构并写入种子（超管/系统配置/预选标签/协议占位）
```

> 未配置 Redis 时自动使用内存缓存降级；未配置 COS 时图片存本地 `backend/data/uploads`；未配置 SMTP 时邮件不发送、找回密码入口关闭。

## 二、前端

```bash
cd frontend
npm install
npm run build       # 产物输出到 frontend/dist
# 本地预览
npm run preview
```

开发模式：`npm run dev`（默认代理 `/api` 到 `http://localhost:8080`，请确保后端以 `SERVER_PORT=8080` 运行）。

## 三、生产部署（Nginx + systemd）

```bash
# 1) 放置二进制与 .env
sudo mkdir -p /opt/tzwall/backend /opt/tzwall/frontend /opt/tzwall/deploy
sudo cp backend/tzwall /opt/tzwall/backend/
sudo cp backend/.env /opt/tzwall/backend/.env
sudo cp -r frontend/dist /opt/tzwall/frontend/dist

# 2) 专用用户与权限
sudo useradd -r -s /usr/sbin/nologin tzwall
sudo chown -R tzwall:tzwall /opt/tzwall
sudo chmod 600 /opt/tzwall/backend/.env

# 3) systemd
sudo cp deploy/tzwall.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now tzwall

# 4) Nginx
sudo cp deploy/nginx.conf.example /etc/nginx/sites-available/tzwall
sudo ln -s /etc/nginx/sites-available/tzwall /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

后端 `SERVER_PORT` 建议设为 `8080`，由 Nginx 监听 `80` 回源；EdgeOne 在边缘终结 SSL。

## 四、备份与运维

```bash
sudo cp deploy/backup.service deploy/backup.timer /etc/systemd/system/
sudo systemctl enable --now backup.timer   # 每日备份，保留 7 份
```

- 日志：`journalctl -u tzwall`
- 健康检查：`GET /api/health` 返回 `{"status":"ok"}`
- 水印字体：安装 `fonts-noto-cjk` 并在 `.env` 填写 `WATERMARK_FONT_PATH`；Go 字体库不支持 `.ttc` 集合，需使用单文件 TTF（如 Noto Sans SC），缺失时图片仍正常返回但不烧录水印
- 磁盘容量告警：监控 `backend/data/uploads` 目录

## 五、权限与降级（阶段一）

- 默认拒绝 + 角色默认权限集的最简版：公开白名单（首页/列表/详情/登录/注册/激活/找回/协议/静态资源/健康检查）未登录可访问；其余端点需登录。
- 超级管理员不受限；普通管理员可审核投稿、标签与用户管理。
- 外部依赖（COS / 数据万象 / SMTP）均以接口抽象，未配置即降级；后台 `/api/meta/site` 返回 `features` 状态供前端展示。
- 图片统一经 `/api/images/:id` 后端代理，浏览器不直连存储；代理动态烧录“用户名|时间”水印（游客显示“游客”），按“用户+IP+图片”短缓存。

## 六、初始账号

`.env` 中的 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 在库为空时写入为超级管理员；库非空后不再覆盖。登录后可在后台“用户管理”授予普通管理员（超级管理员身份仅手动变更）。

## 七、阶段二已完成（治理核心）

- 访问权限矩阵：端点目录 + 页面清单下发、用户覆盖 > 角色默认集 > 公开白名单 > 默认拒绝、端点级四档速率、超管直通
- 处罚全流程：警告确认、禁言、封禁用户名、IP 封禁（IPv4/IPv6/CIDR）、强制改密踢下线、临时到期自动解除、撤销仅标记
- 举报：投稿/评论举报、唯一性与频率限制、禁言禁止、处理结果对举报人可见
- 内容审核：COS 私有存储、数据万象文本/图片审核、回调 + 轮询双通道、审核任务落库；未配置/失败一律转人工
- 访问日志：接近全量记录（Header/请求体摘要）、敏感字段脱敏、批量异步写入、保留期自动清理
- 部署：`deploy/provision.sh`、`deploy/deploy.sh`、`deploy/run.sh`、`deploy/stop.sh`、`deploy/seed-demo.py`

运行示例见 [`docs/DEMO.md`](docs/DEMO.md)。

> COS 与数据万象已接入（Bucket `tzwall-1321423822`，区域 `ap-nanjing`）：图片上传后即审，投稿文本同步审核，回调与 60 秒轮询双通道落地结果；未配置审核策略时使用 Bucket 默认策略，可疑/违规/失败一律转人工。

## 八、阶段三已完成（配置中心与运维）

- 系统配置中心（仅超管）：40+ 配置项分组管理、类型与范围校验、敏感项 AES 加密与掩码、保存即热更新、变更写审计
- SMTP 后台录入：`mailer.Dynamic` 按配置快照重建，保存后下一次发信即生效
- 投稿审核链路升级：**腾讯云审核通过 → AI 复审 → AI 通过才直接放行**；腾讯云调用异常直接驳回；AI 异常/超时/超限转人工；违禁词提前拦截
- AI 审核客户端：OpenAI 兼容协议（`internal/ai`），Base URL / 密钥 / 模型 / 提示词 / 超时 / 每日上限均可在后台配置，密钥加密存储
- 词表治理：违禁词拦截（用户名、投稿标题正文、评论、强制改密新用户名），常用词仅记录；缓存热加载，支持批量导入
- 协议版本历史：发布与回滚，版本变化触发已登录用户重新确认
- 访问日志与审计日志页（仅超管）：筛选、分页、请求详情抽屉，敏感字段脱敏，保留天数可配
- 运维面板（仅超管）：磁盘 / 数据库 / COS 用量指标、备份文件列表、DB/Redis/COS/审核/AI/邮件健康矩阵、磁盘与 COS 阈值告警
- 图片不再烧录水印：代理直接返回原图，缓存键简化为图片 ID；全站水印由前端覆盖层承担

## 九、配置与回调文档

- 部署与配置指南：[`docs/CONFIG.md`](docs/CONFIG.md)（环境变量、部署步骤、后台配置项、图片签名链接与盲水印、邮件、权限、备份、上线清单）
- 违规审核回调配置指南：[`docs/MODERATION_CALLBACK.md`](docs/MODERATION_CALLBACK.md)（COS 内容审核/自定义风险库/自动冻结、回调地址与报文、验证与排障）
- 演示流程：[`docs/DEMO.md`](docs/DEMO.md)
