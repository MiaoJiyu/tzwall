# 泰兴中学校园墙 — 项目配置与部署指南

> 面向下一次全新部署：从依赖安装、环境变量、部署、后台配置到图片/水印/审核/邮件等关键能力，按顺序照做即可。
> 违规图片/文本审核回调的 COS 侧配置见 [`MODERATION_CALLBACK.md`](./MODERATION_CALLBACK.md)。

---

## 一、技术栈与目录

- 后端：Go 1.21 + Gin + GORM + PostgreSQL + Redis（可选）+ 腾讯云 COS / 数据万象
- 前端：Vue 3 + Vite + TypeScript + Element Plus + Tailwind + Pinia
- 部署：Ubuntu 裸机 + Nginx（80）+ 后端二进制（systemd 或脚本）；EdgeOne 可在边缘终结 SSL

```
backend/        Go 后端
frontend/       Vue3 前端
deploy/         provision.sh / deploy.sh / run.sh / stop.sh / nginx.conf.example / systemd 单元
docs/           文档
```

---

## 二、首次部署步骤

### 1. 环境初始化

```bash
sudo bash deploy/provision.sh
# 安装 PostgreSQL / Redis / Nginx / 中文字体，并创建数据库与用户（幂等）
```

手动等价命令：

```bash
sudo apt-get update
sudo apt-get install -y postgresql redis-server nginx fonts-noto-cjk
sudo -u postgres psql -c "CREATE USER tzwall WITH PASSWORD '你的强密码';"
sudo -u postgres psql -c "CREATE DATABASE tzwall OWNER tzwall;"
```

> `pg_trgm` 扩展会在迁移时自动创建（用于搜索）。

### 2. 配置环境变量

```bash
cd backend
cp .env.example .env
# 编辑 .env（见第三节）
```

### 3. 构建与发布

```bash
# 一键：构建后端二进制 + 构建前端 dist + 写 Nginx 配置 + 重启后端
sudo AND 环境就绪后在仓库根目录执行：
bash deploy/deploy.sh
```

`deploy.sh` 会：
- `go build -o tzwall ./cmd/tzwall`
- `npm run build`（输出 `frontend/dist`）
- 复制 `deploy/nginx.conf.example` 到 `/etc/nginx/sites-enabled/tzwall`，并把 root 替换为实际的 `frontend/dist` 路径
- 停止旧进程并 `deploy/run.sh` 启动，最后做 `/api/health` 健康检查

### 4. 仅重启前后端（已构建过）

```bash
bash deploy/stop.sh
bash deploy/run.sh      # 日志：/var/log/tzwall.log
# 若改了前端：cd frontend && npm run build && sudo nginx -s reload
```

### 5. systemd 方式（可选）

`deploy/tzwall.service`、`deploy/backup.service`、`deploy/backup.timer` 可复制到 `/etc/systemd/system/` 后 `systemctl enable --now tzwall`。

---

## 三、环境变量（backend/.env）

> **敏感项不要入库**：`.env` 已在 `.gitignore` 中忽略，仅保留 `.env.example` 模板。

| 变量 | 说明 | 示例 |
|---|---|---|
| `SERVER_PORT` | 后端监听端口（Nginx 反代目标） | `8080` |
| `DB_HOST` / `DB_PORT` | PostgreSQL 地址 | `127.0.0.1` / `5432` |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | 数据库账号/密码/库名 | `tzwall` / `******` / `tzwall` |
| `DB_SSLMODE` | 数据库 SSL 模式 | `disable` |
| `REDIS_ADDR` | Redis 地址；留空则用内存缓存 | `127.0.0.1:6379` |
| `REDIS_PASSWORD` / `REDIS_DB` | Redis 密码 / 库号 | `******` / `0` |
| `JWT_SECRET` | 登录令牌签名密钥（**务必更换为长随机串**） | 64 位随机 |
| `AES_KEY_B64` | 32 字节 AES 密钥的 Base64（用于加密敏感配置；图片链接签名密钥也由它派生） | `base64(32字节)` |
| `COS_SECRET_ID` / `COS_SECRET_KEY` | 腾讯云 API 密钥 | `AKID...` / `******` |
| `COS_BUCKET` / `COS_REGION` | 存储桶名与地域 | `tzwall-1321423822` / `ap-nanjing` |
| `COS_BASE_URL` | 自定义访问域名（用于图片处理请求；需带 `https://` 才启用，否则回退源站域名） | `https://cos.example.com` |
| `COS_AUDIT_BIZ_TYPE` | 数据万象审核策略 BizType；留空使用默认策略 | 空 |
| `COS_CALLBACK_TOKEN` | 审核回调校验令牌；**使用 COS 内容审核控制台回调时必须留空**（该回调不带自定义头） | 空 |
| `SMTP_HOST` / `SMTP_PORT` | 邮件服务器（465 为隐式 TLS） | `smtp.163.com` / `465` |
| `SMTP_USER` / `SMTP_PASS` / `SMTP_FROM` | 发件账号/密码/发件人 | `xxx@163.com` / `******` / 同上 |
| `TRUSTED_PROXIES` | 可信代理（用于解析真实 IP，逗号分隔 CIDR/IP） | `127.0.0.1` |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` / `ADMIN_EMAIL` / `ADMIN_REAL_NAME` / `ADMIN_SCHOOL` | 首次启动创建的超级管理员 | `admin` / `******` … |

生成密钥示例：

```bash
# AES_KEY_B64（32 字节）
openssl rand -base64 32
# JWT_SECRET
openssl rand -hex 32
```

---

## 四、Nginx 反代

参考 `deploy/nginx.conf.example`：

- 前端静态：`root <项目>/frontend/dist; try_files $uri $uri/ /index.html;`
- API 反代：`location /api/ { proxy_pass http://127.0.0.1:8080; proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; proxy_read_timeout 60s; }`
- `client_max_body_size 20m;`（图片上传）
- 真实 IP 依赖 `X-Forwarded-For` + `TRUSTED_PROXIES`

---

## 五、后台配置中心（/admin/configs，仅超管）

大部分配置**改后即时生效，无需重启**（敏感项加密存储、留空表示不修改）。

### 站点
| 键 | 说明 | 默认 |
|---|---|---|
| `site_name` | 站点名称 | 泰兴中学校园墙 |
| `site_url` | 站点访问地址，用于**邮件中的激活/重置链接** | 空（回退 `http://localhost`） |
| `agreement_required` | 协议更新后强制已登录用户重新确认 | true |

### 注册与登录
`register_rph`、`login_rph`、`login_fail_user_limit`、`login_fail_admin_limit`、`login_lock_minutes`、`email_verify_ttl_hours`

### 发帖与评论
`comment_max_len`、`comment_max_depth`、`tag_max_count`

### 图片
| 键 | 说明 | 默认 |
|---|---|---|
| `image_max_size_mb` / `image_max_count` / `image_formats` | 上传大小/数量/格式 | 5 / 9 / jpg,png… |
| `watermark_cache_seconds` | 代理图片服务端缓存秒数（**开启盲水印时不使用共享缓存**） | 600 |
| `image_link_ttl_seconds` | 图片签名链接有效期（秒），过期需刷新页面重新签发 | 600 |
| `image_blind_watermark` | 图片盲水印开关（用户名/ID+IP+时间，经数据万象写入） | true |
| `image_process_max_width` | 展示图片先缩放到该宽度再嵌盲水印（0=不限制） | 1280 |

### 审核与 AI
`ai_enabled`、`ai_base_url`、`ai_api_key`（加密）、`ai_model`、`ai_prompt`、`ai_timeout_seconds`、`ai_daily_limit`

### 举报
`report_hour_limit`、`report_day_limit`、`report_reason_max_len`

### 日志 / 运维 / 页脚
`log_access_retention_days`、`log_audit_retention_days`、`upload_dir`、`ops_disk_warn_percent`、`ops_cos_alert_gb`、`backup_dir`、`footer_icp`、`footer_psba`、`footer_custom`

### 运维（维护模式）
| 键 | 说明 | 默认 |
|---|---|---|
| `maintenance_enabled` | 系统维护模式开关 | false |
| `maintenance_message` | 维护页展示的说明文案（支持多行） | 站点正在更新维护，请稍后再来。 |

**维护模式工作原理**：开启后由 **Nginx 依据标志文件**（`/opt/tzwall/run/MAINTENANCE`）直接返回静态维护页
（`/opt/tzwall/run/maintenance.html`，位于前端 `dist` 之外）。因此**前端构建或后端下线期间，访客仍能看到维护提示**，
页面每 20 秒自动重试，维护结束后自动恢复，无需用户手动刷新。

- 后台开启/关闭：系统配置 → 运维 → 系统维护模式（保存即生效，无需重启）。
- 命令行开关（后端不可用时同样可用）：
  ```bash
  /opt/tzwall/maintenance.sh on "正在更新，请稍候"   # 开启（可附自定义文案）
  /opt/tzwall/maintenance.sh off                    # 关闭
  /opt/tzwall/maintenance.sh msg "新的维护说明"       # 仅更新文案
  /opt/tzwall/maintenance.sh status                 # 查看状态
  ```
- 维护期间**仍放行**：`/api/*`（接口）、`/admin/*`（管理员后台，可登录并关闭维护）、`/assets/*` 与静态资源。
  其余站点页面统一返回维护页（HTTP 503 + `Cache-Control: no-store`，CDN 不缓存）。
- 推荐上线流程：`maintenance.sh on` → 更新前端/后端 → 验证 → `maintenance.sh off`。

> SMTP 也可在「系统配置 → 注册与登录」中填写（后台优先于 `.env`）。

---

## 六、图片方案（重点）

当前采用 **私有读写 + 后端签名链接 + 盲水印**：

1. **对象私有**：上传到 COS 时对象 ACL=`private`，浏览器不直接访问 COS。
2. **签名时效链接**：稿件接口返回的图片地址形如
   `/api/images/{id}?e={过期时间戳}&u={浏览者ID}&s={HMAC签名}`
   - 签名 `HMAC-SHA256`，密钥由 `AES_KEY_B64` 派生；`u` 也参与签名，防止篡改；
   - 有效期由 `image_link_ttl_seconds` 控制。
3. **代理取流**：`GET /api/images/{id}` 校验签名与时效、Referer 防盗链后，从私有 COS 内部取流返回。
4. **盲水印**：代理取流时附加数据万象规则
   `?watermark/3/type/3/text/<URL安全Base64>`，文本为 `u{用户ID36}i{IP压缩36}t{时间Unix36}`（文字盲水印仅支持字母数字，故用 base36/压缩）。
   - 关闭：后台 `image_blind_watermark=false`；需桶已开通数据万象图片处理。
   - 图片处理异常会自动回退原图。
5. **违规图自动下架**：见 [`MODERATION_CALLBACK.md`](./MODERATION_CALLBACK.md)。

**存量对象 ACL 校正**（把历史对象统一设为私有）：

```bash
# 以超管登录拿 token 后
curl -X POST http://127.0.0.1/api/admin/images/sync-private \
  -H "Authorization: Bearer <SUPER_ADMIN_TOKEN>"
```

---

## 七、邮件与激活

- SMTP 推荐 465（隐式 TLS，已内置支持）或 587（STARTTLS）。
- 邮件正文中的激活/重置链接为**绝对可点击链接**，基于 `site_url` 生成：
  `{site_url}/verify?token=...`、`{site_url}/reset?token=...`。
  上线务必把 `site_url` 配成真实域名（如 `https://wall.example.com`）。
- 注册发信为异步，不会阻塞注册请求。

---

## 八、权限与角色

- 角色：`user` / `admin` / `super_admin`；超管直通，不受限流。
- 后台「访问权限」按 **端点** 配置允许与四档限流；页面权限由端点权限推导。
- 新端点/页面在代码 `permission/registry.go` 登记后，重启会自动补齐默认授权（幂等，不覆盖已有配置）。
- 新增后台能力后，如管理员看不到入口，去「访问权限」勾选对应端点或重启让默认授权生效。

---

## 九、备份与运维

- `deploy/backup.sh` + `backup.timer`：定时 `pg_dump` 到 `backup_dir`。
- 运维面板 `/admin/ops`：磁盘/数据库/COS 用量、六项健康（db/redis/cos/moderation/mailer/ai）、备份列表、阈值告警。
- 访问/审计日志保留天数由配置控制。

---

## 十、上线检查清单

- [ ] `JWT_SECRET`、`AES_KEY_B64`、数据库密码均已更换为强随机值
- [ ] `site_url` 已配置为真实域名（HTTPS）
- [ ] `TRUSTED_PROXIES` 与 Nginx/EdgeOne 回源一致，真实 IP 正确
- [ ] COS 桶：私有读写（或按需私有写公有读），已开通数据万象图片处理
- [ ] 存量图片已执行 `sync-private`（如曾切换过 ACL）
- [ ] SMTP 可用，能收到激活邮件且链接可点击
- [ ] 审核回调已按 `MODERATION_CALLBACK.md` 配置并验证
- [ ] `curl http://127.0.0.1/api/health` 返回 `{"status":"ok"}`
- [ ] 前端域名访问正常，后台各页面可打开

---

## 十一、常见问题

| 现象 | 排查 |
|---|---|
| 图片 403 | 链接过期（刷新页面重新签发）／签名被改（`u/e/s`）／Referer 非同站／图片已被判定违规下架 |
| 图片 404 | 对象不存在或被内容审核转移至 `audit_freeze_backup/increment_audit` |
| 图片不显示且后台 `audit=block` | 已判定违规并下架；确认无违规后再处理 |
| 邮件发不出 | `.env`/后台 SMTP 未配置；465 需隐式 TLS（已支持）；检查发件账号授权码 |
| 登录 429 | 触发登录频率限制（`login_rph`），或 Redis 中 `rl:login:*` 计数 |
| 后台入口缺失 | 「访问权限」未授权该端点／页面 |
| 定时任务/备份不跑 | `systemctl status tzwall-backup.timer` |

---

## 七、网络与流量优化（公网上行）

目标：降低服务器公网上行（EdgeOne→用户方向）与 COS 外网下行，同时保留盲水印与完整可溯源能力。

### 1. 缓存分层
| 层 | 对象 | 键 | TTL | 说明 |
|---|---|---|---|---|
| 浏览器 | 媒体/静态资源 | URL | 图片 5min（水印）/ 7d（原图）；视频 1d | 由后端 `Cache-Control` 控制 |
| **Nginx 源站盾** | `/api/images/` `/api/videos/` | 图片 `$uri:$arg_u`（游客按 IP）；视频 `$uri$http_range` | 图片 5min；视频 1h | 重复访问不再回源后端，**键按查看者隔离，避免盲水印跨用户串用** |
| 后端字节缓存 | 无水印图片 | `img:<id>` | `watermark_cache_seconds` | 仅无水印场景 |
| 本地磁盘盾 | COS 对象 | key 哈希 | 长期（容量淘汰） | 减少 COS 取流 |

- 缓存区：`/var/cache/nginx/tzwall_media`（6G 上限、7 天未用剔除），配置在 `conf.d/tzwall-media-cache.conf`。
- **立即失效（下架/删图）**：`rm -rf /var/cache/nginx/tzwall_media/*`（后端已会返回 403，清缓存后即刻生效）。
- 响应头 `X-Cache-Status`：`HIT/MISS/BYPASS/UPDATING`，用于观测命中率。

### 2. 可观测与溯源
- **Nginx 访问日志**（`log_format tzwall`，定义于 `conf.d/tzwall-traffic-log.conf`）：含 `cache=`（缓存状态）、`$body_bytes_sent`（响应字节）、`rt/urt`（耗时）、`rid`（请求 ID）。
- **后端 `access_logs`** 新增字段：`bytes_sent`（响应字节，计费依据）、`cache_status`（`mem`=后端字节缓存命中 / `origin`=回源）、`resource`（如 `image:27`）、`watermark`（是否走盲水印）。
- 双向溯源：`rid` 关联 nginx 与后端日志；`resource` + `watermark` 关联到具体媒体与水印批次。
- **超时截断请求计数**：访问日志在**请求入口即建档**（`StatusCode=0` 表示处理中），handler 结束时补全状态/字节/耗时；
  客户端中途断开（如压测超时被截断）而 handler 未结束的请求，由看门狗标记 `truncated=true`。
  因此**被截断的请求同样计入 IP 访问统计与限流计数**，可在后台「访问日志」按「截断」标记排查刷流量行为。
  - 通道满时改为同步落库，不再丢记录；
  - 图片取流限时 30s，避免 COS 卡住或客户端已断连时无限占用连接/协程。

### 3. 静态资源瘦身
- `favicon.svg`（原 4.0MB，VTracer 逐像素追踪产物）→ `favicon.png` 256×256 调色板（**19.9KB**，降幅 99.5%）；原文件备份在 `backup/`。
- `dist` 体积 8.7MB → 4.9MB。
- 未读角标轮询：15s → 30s，且**页面隐藏时暂停、回前台立即刷新**。
- `meta/site`、`meta/routes`、`meta/agreements` 增加 **ETag + 304** 与 `private, max-age` 短缓存，避免重复传输相同 JSON。
- ⚠️ **按用户变化的响应禁止用 `max-age` 缓存**：`/api/announcements` 列表含 `confirmed` 字段（依用户与「确认」动作变化），若用 `max-age` 缓存，浏览器会在有效期内直接返回本地过期的 `confirmed:false`，导致「用户确认后刷新仍重复弹窗」。故该接口改用 `ETag(-1)` → `Cache-Control: private, no-cache`（每次校验，未变 304、确认后变 200），既修复弹窗 bug 又保留免重传优化。
- **后台模块开关实时生效**：`/api/meta/site` 现下发 `song_enabled` / `poll_enabled`（缺省视为启用，仅显式 `false` 才关闭，与 `service.SongService.enabled()` 语义一致）。前端据此：① 导航菜单（桌面/移动）按开关隐藏「点歌」入口；② 路由守卫 `page:songs` 在开关关闭时把 `/songs` 重定向回首页，杜绝直接 URL 进入；③ `Songs.vue` 在开关中途变化时实时清空列表并显示「点歌功能已关闭」。`App.vue` 每 60s 重新拉取 `meta/site`（ETag 未变则为 304，开销极小），保证后台改动在约 1 分钟内对在线用户生效；后台通过配置接口保存会同步内存缓存，服务端下一次 `meta/site` 即反映新值。
- **微信「小程序码自动绑定」开关 `wechat_mp_auto_bind`**（超级管理员可勾选，默认 `false`，位于账号配置分组）：开启后网页账号中心「绑定微信」生成携带绑定令牌的**动态小程序码**（`scene=bind:<token>`），用户扫码后小程序自动用 `wx.login` 完成绑定，无需手动输入授权码；关闭则沿用「静态小程序码 + 手动授权码」流程。需已配置微信小程序 AppID/Secret。后端 `POST /api/auth/wechat/bind-code` 据开关返回 `{auto, code, image_base64}`；小程序侧 `POST /api/auth/wechat/bind/confirm` 凭绑定令牌 + `wx.login` 的 `code` 完成绑定（与手动 `wechat/bind-code/confirm` 对称）。

### 4. COS 内网与盲水印修复
- `COS_BASE_URL` 指向**同地域内网域名**（`cos-internal.ap-nanjing.myqcloud.com`）：免 COS 公网下行、更低延迟。
- 修复：`ProcessedGet` 在自定义域名不可达时**自动回退标准 COS 域名**（此前 `cos.tzwall.miaojiyu.top` 服务器侧 TLS 不可达，导致盲水印长期静默失败、全部回退原图）。
