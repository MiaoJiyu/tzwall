# 泰兴中学校园墙 · 系统技术文档

> 面向开发与维护人员的系统技术说明。  
> 覆盖：系统定位、功能模块、接口清单、整体架构、数据对象、运行环境、已知限制。

| 项目 | 说明 |
| --- | --- |
| 系统名称 | 泰兴中学校园墙（Taixing Middle School Campus Wall） |
| 代码仓库 | `tzwall`（`backend/` Go 服务 + `frontend/` Vue SPA + `miniprogram/` 微信小程序 + `deploy/` 部署脚本） |
| 文档版本 | v1.0 |
| 适用对象 | 后端/前端开发、运维、后续维护者 |
| 事实来源 | 以仓库当前代码为准（`internal/handler/router.go`、`internal/model/*`、`internal/config/*`、`deploy/*`） |

---

## 1. 系统定位与目标

泰兴中学校园墙是面向泰兴中学**师生**的校园社区网站，提供内容发布、浏览、互动与治理能力，并配套管理后台与微信小程序端。

设计目标：

- **安全可控**：内容先审后发（腾讯云内容安全 + AI 复审 + 人工兜底），全站违禁词拦截，处罚与举报闭环。
- **权限精细**：端点级「权限矩阵」+ 页面级访问清单，用户覆盖 > 角色默认 > 公开白名单 > 默认拒绝；端点支持四档速率限制。
- **可运维**：配置中心热更新、访问/审计日志、运维面板、备份脚本、维护模式。
- **可降级**：COS / 内容审核 / SMTP / Redis 任一未配置时自动降级，不影响核心阅读与发布能力。
- **轻量部署**：Nginx + systemd 裸机部署（不使用 Docker），适配边缘加速（EdgeOne）回源。

功能边界：校园场景的投稿、评论、点赞/点踩、投票、点歌、私聊、工单、公告、举报、处罚、用户主页、后台治理。

---

## 2. 技术栈与目录结构

### 2.1 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go 1.21、Gin 1.10、GORM 1.25、PostgreSQL（`pg_trgm`）、Redis（可选） |
| 前端 | Vue 3.4、Vite 5、TypeScript 5、Element Plus 2.7、Tailwind CSS 3.4、Pinia 2、Vue Router 4 |
| 小程序 | 微信小程序原生（`miniprogram/`） |
| 存储 | 腾讯云 COS（对象存储）+ 数据万象（内容审核 / 盲水印）；未配置时本地磁盘 |
| 部署 | Nginx 反向代理 + systemd；EdgeOne 边缘终结 SSL |

### 2.2 目录结构

```
tzwall/
├── backend/                 Go 后端
│   ├── cmd/tzwall/main.go   进程入口（仅调用 bootstrap.Run）
│   ├── internal/
│   │   ├── bootstrap/       依赖装配、迁移/种子、HTTP 启停、优雅退出
│   │   ├── config/          环境变量加载 + 配置中心目录（registry）
│   │   ├── database/        连接、迁移（AutoMigrate + 索引）、种子
│   │   ├── model/           数据模型（GORM）
│   │   ├── service/         业务逻辑层
│   │   ├── handler/         HTTP 处理器 + 路由（router.go）
│   │   ├── middleware/      真实IP/IP封禁/鉴权/处罚门禁/权限限流/ETag/访问日志
│   │   ├── permission/      端点目录 + 页面清单 + 权限解析（resolver）
│   │   ├── cache/           Redis / 内存缓存抽象
│   │   ├── storage/         本地/COS 存储抽象（含源站缓存）
│   │   ├── moderation/      内容审核抽象 + 回调/轮询 Worker
│   │   ├── ai/              OpenAI 兼容 AI 审核客户端
│   │   ├── mailer/          邮件发送抽象（动态配置）
│   │   ├── social/          微信/QQ 第三方登录
│   │   ├── watermark/       盲水印提取
│   │   └── pkg/             errs / response / token / crypto / iputil / logger / validate
│   └── data/                本地上传与媒体缓存目录（未配置 COS 时）
├── frontend/                Vue3 SPA
│   └── src/
│       ├── api/             axios 封装与各模块 API
│       ├── stores/          Pinia（auth / meta 等）
│       ├── router/          路由与页面级权限守卫
│       ├── layouts/         PublicLayout / AdminLayout
│       ├── views/           页面（含 admin/ 子目录）
│       ├── components/      复用组件（卡片、对话框等）
│       └── styles/          tokens.css 等
├── miniprogram/             微信小程序
├── deploy/                  systemd / Nginx 示例 / 备份 / 部署脚本
└── docs/                    运维与接入文档
```

---

## 3. 整体架构与设计思路

### 3.1 分层架构

```
浏览器 / 小程序
      │  HTTPS
      ▼
EdgeOne（边缘 SSL 终结）
      │  HTTP 回源 :10001
      ▼
Nginx :10001  ──静态资源──▶ frontend/dist（SPA）
      │  /api/ 反向代理
      ▼
Go 服务 :10002（Gin）
  ├─ 中间件链（RealIP → IPBan → Auth → Punishment → AccessLog → 端点权限/限流）
  ├─ handler（参数解析、鉴权、响应封装）
  ├─ service（业务规则、事务、跨表编排）
  ├─ 基础设施抽象（cache / storage / moderation / mailer / ai / social）
  └─ GORM → PostgreSQL ; Redis(可选) ; COS/本地
```

设计要点：

- **handler 薄、service 厚**：handler 只做绑定/校验/响应；业务规则与跨表操作集中在 `service/`。
- **接口化外部依赖**：`cache.Cache`、`storage.Storage`、`moderation.Moderation`、`mailer.Mailer` 均为接口，未配置即降级（内存缓存 / 本地磁盘 / 转人工审核 / 邮件不发送）。
- **权限即数据**：`permission/registry.go` 的端点目录与页面清单是事实源，seed 据此生成角色默认权限，后台矩阵与前端页面守卫都依赖它。
- **配置即数据**：`config/registry.go` 登记全部配置项，后台按目录渲染表单、校验类型/范围、敏感项 AES 加密。

### 3.2 请求链路与中间件顺序

`SetupRouter`（`internal/handler/router.go`）装配顺序：

1. `gin.Recovery()` 异常恢复
2. `middleware.RealIP(cfg)` 真实客户端 IP（信任代理）
3. `middleware.NewIPBan(punSvc).Gate()` IP 封禁拦截
4. `authMW.Optional` 身份解析（有 Token 则解析，无则游客）
5. `middleware.NewPunishment(punSvc).Gate()` 处罚门禁（禁言/强制改密/警告待确认等）
6. `middleware.NewAccessLogger(db, store).Log()` 访问日志（异步批量、脱敏、保留期清理）
7. 每个路由的 `perm.Public(key)` / `perm.Require(key)` 端点权限 + 四档限流

> 说明：`perm.Public` 仍需通过权限解析（可被矩阵关闭），`perm.Require` 额外要求登录。

### 3.3 权限解析优先级（`permission/resolver.go`）

1. 超级管理员（`super_admin`）→ 直接放行；
2. 用户级覆盖（`permission_grants.user_id = 当前用户`）；
3. 公开端点（目录 `Public=true`）→ 放行（仅取速率配置）；
4. 角色默认集（`permission_grants.role = 用户角色 AND user_id IS NULL`）；
5. 默认拒绝 → 返回 `41011`（前端提示"请求端点失败"）。

速率：端点可配 `Rps / Rpm / RpHour / RpDay`，0 表示不限，按 `端点+用户/匿名+IP+窗口` 计数（Redis；无 Redis 时跳过限流）。

### 3.4 部署拓扑

- 后端：`SERVER_HOST=127.0.0.1`、`SERVER_PORT=10002`。
- Nginx：监听 `10001`，静态托管 `frontend/dist`，`/api/` 代理到 `127.0.0.1:10002`，SPA 路由回退 `index.html`。
- EdgeOne 在边缘终结 SSL 后回源 `10001`；`X-Forwarded-For` 传递真实 IP。
- systemd 单元：`deploy/tzwall.service`（`User=tzwall`、`MemoryMax=512M`、`Restart=on-failure`）。

### 3.5 降级与容错

| 依赖 | 未配置/异常时的行为 |
| --- | --- |
| Redis | 降级为进程内内存缓存；限流跳过 |
| COS | 图片/视频存本地 `backend/data/uploads` |
| 腾讯云内容审核 | 审核失败/异常一律转人工（`review`/`failed`） |
| AI 审核 | 未启用/异常/超时/超限 → 转人工 |
| SMTP | 邮件不发送；注册免激活、找回密码入口关闭 |
| 维护模式 | `maintenance_enabled` 开启时 Nginx 直接返回维护页（`/api`、`/admin` 仍可访问） |

---

## 4. 主要功能模块及职责

### 4.1 后端 service（业务模块）

| 模块 | 文件 | 职责 |
| --- | --- | --- |
| 鉴权/账号 | `auth.go` / `auth_social.go` | 注册、登录、邮箱激活、找回/重置、改密、资料、注销、Token 版本、社交登录与绑定 |
| 投稿 | `submission.go` | 发布/编辑/撤回/删除、标签、图片视频、审核链路、置顶、列表排序、公开视图 |
| 评论 | `comment.go` | 多级评论、软删除、管理员移除/恢复、点赞 |
| 互动 | `interaction.go` | 投稿/评论的赞/踩（唯一态度） |
| 点歌 | `song.go` | 点歌提交、周配额、状态流转、后台管理 |
| 投票 | `poll.go` | 投票创建、选项、投票/改票、截止、与投稿关联 |
| 私聊 | `chat.go` | 一对一会话、消息收发、未读、已读 |
| 工单 | `ticket.go` | 工单创建/回复/关闭、状态机流转、附件 |
| 公告 | `announcement.go` | 公告 CRUD、必读确认、前台列表 |
| 举报 | `report.go` | 举报创建（唯一性/频率）、后台处理、结果可见 |
| 处罚 | `punishment.go` | 警告/禁言/封号/封 IP/强制改密、到期解除、撤销、门禁判定 |
| 权限 | `permission.go` | 矩阵读写、端点/页面清单下发、解析缓存失效 |
| 配置中心 | `configstore.go` / `configschema.go` | 配置读写、类型校验、密文加解密、热更新、审计 |
| 协议 | `agreement.go` | 协议发布/版本历史/回滚、版本变更触发重新确认 |
| 词表 | `wordfilter.go` | 违禁词拦截、常用词记录、缓存热加载 |
| 用户 | `user.go` | 公开主页、资料展示字段过滤 |
| 管理 | `admin.go` | 仪表盘、用户/标签/投稿列表与管理 |
| 审计/日志 | `audit.go` | 审计写入；访问日志查询与保留 |
| 运维 | `ops.go` | 磁盘/DB/COS 指标、备份列表、健康矩阵 |
| 维护 | `maintenance.go` | 维护模式标志文件与维护页 |

### 4.2 后端基础设施与中间件

- `middleware/`：`realip`、`ipban`、`auth`（Optional/Required）、`punishment`、`permission`（Public/Require + 限流）、`etag`、`accesslog`、`ratelimit`。
- `handler/`：按领域拆分（auth、submission、comment、poll、song、ticket、chat、announcement、image、video、report、punishment、permission、config、agreement、word、log、ops、watermark、meta、moderation）；`router.go` 统一装配。
- `pkg/`：`response`（统一响应体）、`errs`（业务码/文案）、`token`（JWT）、`crypto`（AES）、`iputil`、`logger`、`validate`。

### 4.3 前端

- `api/http.ts`：axios 实例（`baseURL=/api`，请求拦截注入 `Authorization: Bearer <token>`，统一错误处理）。
- `api/index.ts`：按模块导出 `authApi / submissionApi / pollApi / songApi / chatApi / ticketApi / adminApi ...`。
- `stores/`：`auth`（登录态、Token、用户信息）、`meta`（站点信息、页面权限清单、`can()` 守卫）。
- `router/index.ts`：`PublicLayout` + `AdminLayout`；`beforeEach` 做 `requireAuth`、`admin`、页面级 `meta.can(page)` 校验与模块开关（如 `song_enabled`）。
- `views/`：首页、投稿详情、发布、我的投稿、资料、消息、点歌、投票、工单、用户主页；`views/admin/` 为后台各管理页。

---

## 5. 关键数据对象及其依赖关系

### 5.1 实体清单（GORM 表）

| 实体 | 表 | 说明 |
| --- | --- | --- |
| User | `users` | 用户（角色/状态/社交身份/公开主页/称号/Token 版本） |
| Submission | `submissions` | 投稿主体（状态/计数/置顶/待审编辑） |
| Tag | `tags` | 标签（审核状态/合并目标） |
| SubmissionImage / Video | `submission_images` / `submission_videos` | 图片/视频元数据与审核状态 |
| SubmissionRevision | `submission_revisions` | 修改历史 |
| Comment | `comments` | 多级评论（软删除/审核状态） |
| Reaction | `reactions` | 赞/踩（用户+目标唯一） |
| Punishment | `punishments` | 处罚记录（可叠加/撤销/确认） |
| IPBan | `ip_bans` | IP 封禁（CIDR） |
| Report | `reports` | 举报（唯一性约束） |
| PermissionGrant | `permission_grants` | 端点权限（角色默认 / 用户覆盖）+ 速率 |
| Poll / PollOption / PollVote | `polls` / `poll_options` / `poll_votes` | 投票、选项、投票记录 |
| SubmissionPoll | `submission_polls` | 投稿↔投票关联（投票唯一关联） |
| Song | `songs` | 点歌记录 |
| Ticket / TicketReply | `tickets` / `ticket_replies` | 工单与处理记录 |
| Conversation / Message | `conversations` / `messages` | 私聊会话与消息 |
| Announcement / AnnouncementConfirm | `announcements` / `announcement_confirms` | 公告与确认记录 |
| Agreement / AgreementVersion | `agreements` / `agreement_versions` | 协议与历史版本 |
| WordFilter | `word_filters` | 违禁/常用词 |
| SystemConfig | `system_configs` | 键值配置（热更新） |
| AccessLog / AuditLog | `access_logs` / `audit_logs` | 访问日志 / 审计日志 |
| ModerationTask | `moderation_tasks` | 内容审核任务（回调/轮询追溯） |

### 5.2 主要关系

```
User 1─N Submission        Submission N─N Tag (submission_tags)
User 1─N Comment           Submission 1─N SubmissionImage / SubmissionVideo
User 1─N Reaction          Submission 1─N Comment / Reaction
User 1─N Song / Poll / Ticket
User 1─N Punishment / Report / Message
Submission 1─N SubmissionRevision
Submission 1─N SubmissionPoll N─1 Poll
Poll 1─N PollOption / PollVote
Ticket 1─N TicketReply
Conversation 1─N Message
Announcement 1─N AnnouncementConfirm
Agreement 1─N AgreementVersion
PermissionGrant：role(+可选 user_id) × endpoint_key → allow + 四档速率
```

要点：

- **Reaction** 以 `(user_id, target_type, target_id)` 唯一，`target_type ∈ {submission, comment}`，`attitude ∈ {1, -1}`。
- **SubmissionPoll** 的 `poll_id` 唯一，保证一个投票最多关联一个投稿；单投稿可关联多个投票（上限由 `poll_max_per_submission` 控制）。
- **Conversation** 以 `(user_low, user_high)` 规范排序后唯一，保证两人仅一个会话。
- **Punishment** 撤销仅置 `revoked`，不物理删除；到期自动失效（按 `expires_at`）。
- **权限/配置** 均以目录（registry）为事实源，数据库存储实际值与用户覆盖。

### 5.3 关键状态机

- 投稿：`pending`（审核中）→ `approved` / `rejected` / `need_edit`；作者可 `withdrawn`。已通过帖子编辑后进入"待审编辑"（原文继续展示，新内容暂存 `pending_*`，通过后覆盖）。
- 工单：`pending` → `processing` → `resolved` / `rejected` → `closed`（见 `TicketTransition`）。
- 点歌：`pending` → `played` / `canceled`。
- 审核：`none / pending / pass / block / review / failed`（回调 + 轮询双通道）。

---

## 6. 接口清单

### 6.0 统一约定

- **Base URL**：`/api`（前端 axios `baseURL=/api`）。
- **响应体**（`pkg/response`）：
  ```json
  { "code": 0, "message": "ok", "data": {}, "request_id": "...", "endpoint": "可选，无权时返回" }
  ```
  `code=0` 成功；非 0 为业务码，HTTP 状态按码映射（401/403/404/429/200）。
- **鉴权**：登录返回 `token`；后续请求头 `Authorization: Bearer <token>`。
- **分页**：列表接口通常返回 `data: { items: [], total, page, page_size }`。
- **权限列**：`公开`=无需登录；`登录`=需登录；`管理员`=普通管理员+超管；`超管`=仅超级管理员。

### 6.1 元信息

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `GET /api/health` | 健康检查 | — | `{status:"ok"}` | 公开 |
| `GET /api/meta/site` | 站点信息/功能开关/features | — | 站点名、开关、依赖状态 | 公开（ETag 60s） |
| `GET /api/meta/agreements` | 协议列表 | — | 协议键/版本/标题 | 公开（ETag 300s） |
| `GET /api/meta/agreement?key=` | 单个协议内容 | `key` | 协议正文/版本 | 公开 |
| `GET /api/meta/routes` | 当前用户可访问页面/端点清单 | — | 页面键列表、端点 | 公开（ETag 60s） |
| `POST /api/moderation/callback` | 腾讯云审核回调 | 回调报文 | 200/校验结果 | 公开（Token 校验） |

### 6.2 鉴权与账号

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `POST /api/auth/register` | 注册 | username、email、password、real_name、school、agree | 用户/需激活提示 | 公开 |
| `POST /api/auth/login` | 登录 | identifier、password | token、user | 公开 |
| `GET /api/auth/verify-email?token=` | 邮箱激活 | token | 激活结果 | 公开 |
| `POST /api/auth/resend-verify` | 重发激活邮件 | 邮箱/用户名 | 结果 | 公开 |
| `POST /api/auth/forgot` | 找回密码 | 邮箱 | 结果 | 公开 |
| `POST /api/auth/reset` | 重置密码 | token、新密码 | 结果 | 公开 |
| `POST /api/auth/me` | 当前用户信息 | — | user（含角色/资料/公开设置） | 登录 |
| `POST /api/auth/profile` | 更新资料（部分字段） | nickname/avatar/wechat/qq/phone/real_name/school/profile_public/public_fields | 结果 | 登录 |
| `POST /api/auth/change-password` | 修改密码 | 旧/新密码 | 结果 | 登录 |
| `POST /api/auth/change-email` | 修改邮箱 | 新邮箱/密码 | 结果 | 登录 |
| `POST /api/auth/logout` | 退出登录 | — | 结果（Token 版本+1） | 登录 |
| `POST /api/auth/delete-account` | 注销账号（匿名化） | — | 结果 | 登录 |
| `POST /api/auth/agreement-confirm` | 确认协议 | — | 结果 | 登录 |

### 6.3 社交登录 / 绑定（微信小程序、QQ、网页微信）

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `POST /api/auth/wechat/login` | 小程序微信登录 | 公开 |
| `POST /api/auth/wechat/register` | 小程序微信注册 | 公开 |
| `POST /api/auth/web-login/session` / `qrcode` / `confirm` / `poll` | 网页扫码登录（方案 B） | 公开 |
| `POST /api/auth/wechat/bind-code` | 生成绑定授权码 | 登录 |
| `POST /api/auth/wechat/bind-code/confirm` | 小程序码确认绑定 | 公开 |
| `POST /api/auth/wechat/bind/confirm` | 微信绑定确认 | 公开 |
| `GET /api/auth/qq/url` / `GET /api/auth/qq/callback` | QQ 授权与回调 | 公开 |
| `GET /api/auth/qq/bind/url` | QQ 绑定授权 | 登录 |
| `POST /api/auth/social/exchange` | 社交登录票据交换 | 公开 |
| `POST /api/auth/bind` / `POST /api/auth/unbind` | 绑定/解绑第三方 | 登录 |
| `GET /api/auth/wechat/web/url` / `callback` | 网页微信授权/回调 | 公开 |
| `GET /api/auth/wechat/web/bind/url` | 网页微信绑定授权 | 登录 |

### 6.4 投稿 / 标签 / 评论

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `GET /api/submissions` | 投稿列表 | `page`、`page_size`、`tag`、`keyword`、`sort`(latest/hot) | items/total（置顶优先） | 公开 |
| `GET /api/submissions/:id` | 投稿详情 | id | 投稿视图（含计数/联系/审核状态） | 公开 |
| `GET /api/tags` | 可用标签 | — | 标签列表 | 公开 |
| `GET /api/submissions/:id/comments` | 评论树 | id | 评论列表（含 children） | 公开 |
| `POST /api/submissions` | 发布投稿 | title、content、tags、images、videos、contact、contact_visible、anonymous | 投稿/审核状态 | 登录 |
| `PUT /api/submissions/:id` | 编辑投稿 | 同发布 | 结果 | 登录（作者） |
| `POST /api/submissions/:id/withdraw` | 撤回投稿 | — | 结果 | 登录（作者） |
| `DELETE /api/submissions/:id` | 删除投稿 | — | 结果 | 登录（作者/管理员） |
| `GET /api/my/submissions` | 我的投稿 | — | 列表（含各状态） | 登录 |
| `POST /api/submissions/:id/comments` | 发表评论 | content、parent_id | 评论 | 登录 |
| `DELETE /api/comments/:id` | 删除评论 | — | 结果 | 登录（作者/管理员） |
| `POST /api/comments/:id/like` | 评论点赞 | — | 计数 | 登录 |
| `GET /api/admin/comments` | 评论管理列表 | 分页/筛选 | 列表 | 管理员 |
| `POST /api/admin/comments/:id/remove` / `restore` | 移除/恢复评论 | — | 结果 | 管理员 |

### 6.5 互动

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `POST /api/reactions` | 点赞/点踩 | target_type、target_id、attitude(1/-1) | 计数/结果 | 登录 |

### 6.6 点歌

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `GET /api/songs` | 点歌列表 | 分页 | 列表 | 公开 |
| `GET /api/songs/quota` | 本周剩余额度 | — | 额度/已用 | 登录 |
| `POST /api/songs` | 提交点歌 | title、artist、note | 记录 | 登录 |
| `GET /api/admin/songs` | 后台列表 | 分页/状态 | 列表 | 管理员 |
| `POST /api/admin/songs/:id/status` | 设置状态 | status | 结果 | 管理员 |
| `DELETE /api/admin/songs/:id` | 删除 | — | 结果 | 管理员 |

### 6.7 投票

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `GET /api/polls` | 投票列表 | 分页 | 列表（置顶优先） | 公开 |
| `GET /api/polls/:id` | 投票详情 | id | 选项/票数/我的选择 | 公开 |
| `GET /api/polls/mine` | 我发起的投票 | — | 列表 | 登录 |
| `POST /api/polls` | 发起投票 | title、description、options、multiple、deadline | 投票 | 登录 |
| `POST /api/polls/:id/vote` | 投票/改票 | option_ids | 结果/票数 | 登录 |
| `DELETE /api/polls/:id` | 删除自己的投票 | — | 结果 | 登录 |
| `GET /api/submissions/:id/polls` | 投稿关联投票 | id | 投票列表 | 公开 |
| `POST /api/submissions/:id/polls` | 设置关联投票 | poll_ids | 结果 | 登录（作者/管理员） |
| `GET /api/admin/polls` / `POST` / `DELETE /:id` | 后台投票管理 | 分页/字段 | 结果 | 管理员 |

### 6.8 私聊

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `POST /api/conversations` | 打开/创建会话 | 登录 |
| `GET /api/conversations` | 会话列表 | 登录 |
| `GET /api/conversations/unread` | 未读总数 | 登录 |
| `GET /api/conversations/:id/messages` | 消息列表 | 登录 |
| `POST /api/conversations/:id/messages` | 发送消息 | 登录 |
| `POST /api/conversations/:id/read` | 标记已读 | 登录 |

### 6.9 工单

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `POST /api/tickets` | 创建工单 | 登录 |
| `GET /api/tickets/mine` | 我的工单 | 登录 |
| `GET /api/tickets/:id` | 工单详情 | 登录（本人/管理员） |
| `POST /api/tickets/:id/close` | 关闭工单 | 登录 |
| `GET /api/admin/tickets` / `GET /:id` / `POST /:id/reply` / `POST /:id/status` | 后台处理 | 管理员 |

### 6.10 公告

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /api/announcements` | 公告列表（含 confirmed） | 公开（no-cache） |
| `POST /api/announcements/:id/confirm` | 确认公告 | 登录 |
| `GET/POST/PUT/DELETE /api/admin/announcements[/:id]` | 公告管理 | 管理员 |

### 6.11 图片 / 视频

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `POST /api/images` | 上传图片 | multipart file | 图片 id/尺寸/审核状态 | 登录 |
| `GET /api/images/:id` | 图片代理（含盲水印/COS 私有读） | id、可选访客令牌 | 图片字节 | 公开 |
| `POST /api/videos` | 上传视频 | multipart file | 视频 id/审核状态 | 登录 |
| `GET /api/videos/:id` | 视频代理（支持 Range） | id、Range | 视频流 | 公开 |
| `POST /api/admin/images/sync-private` | 同步图片私有属性 | — | 结果 | 管理员 |

### 6.12 用户公开主页

| 方法 · 路径 | 用途 | 主要输入 | 主要输出 | 权限 |
| --- | --- | --- | --- | --- |
| `GET /api/users/:id/public` | 用户公开主页（按公开设置展示字段） | id | 用户名/昵称/称号/公开字段/近期投稿 | 公开 |

### 6.13 举报

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `POST /api/reports` | 举报投稿/评论 | 登录 |
| `GET /api/reports/mine` | 我的举报 | 登录 |
| `GET /api/admin/reports` | 后台举报列表 | 管理员 |
| `POST /api/admin/reports/:id/resolve` | 处理举报 | 管理员 |
| `GET /api/admin/reports/:id/target` | 举报对象详情 | 管理员 |

### 6.14 处罚

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /api/punishments/status` | 当前处罚状态（禁言/警告/强制改密） | 登录 |
| `GET /api/punishments/mine` | 我的处罚记录 | 登录 |
| `POST /api/punishments/:id/ack` | 确认警告 | 登录 |
| `POST /api/admin/punishments` | 创建处罚 | 管理员 |
| `GET /api/admin/punishments` | 处罚列表 | 管理员 |
| `POST /api/admin/punishments/:id/revoke` | 撤销处罚 | 管理员 |
| `POST/GET /api/admin/ip-bans` | 创建/列出 IP 封禁 | 管理员 |
| `POST /api/admin/ip-bans/:id/revoke` | 撤销 IP 封禁 | 管理员 |

### 6.15 后台：用户 / 标签 / 投稿

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /api/admin/dashboard` | 仪表盘统计 | 管理员 |
| `GET /api/admin/users` / `GET /:id` | 用户列表/详情 | 管理员 |
| `POST /api/admin/users/:id/role` / `title` | 设置角色/称号 | 管理员（角色受限于超管规则） |
| `POST /api/admin/users/:id/resend-verify` / `activate` | 重发激活/激活 | 管理员 |
| `GET /api/admin/tags/pending` | 待审标签 | 管理员 |
| `POST /api/admin/tags/:id/approve` / `merge` | 通过/合并标签 | 管理员 |
| `GET/POST/PUT/DELETE /api/admin/tags[/:id]` | 标签管理 | 管理员 |
| `GET /api/admin/submissions/pending` | 待审投稿 | 管理员 |
| `GET /api/admin/submissions/all` | 全部投稿（筛选/分页） | 管理员 |
| `POST /api/admin/submissions/:id/review` | 审核（通过/驳回/需修改） | 管理员 |
| `POST /api/admin/submissions/:id/restore` | 恢复投稿 | 管理员 |
| `POST /api/admin/submissions/:id/pin` | 置顶（可带 order/expires_at） | 管理员 |
| `POST /api/admin/submissions/:id/unpin` | 取消置顶 | 管理员 |

### 6.16 后台：权限矩阵（超管）

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /api/admin/permissions/endpoints` | 端点目录 | 超管 |
| `GET /api/admin/permissions/matrix` | 权限矩阵 | 超管 |
| `PUT /api/admin/permissions/matrix` | 保存矩阵（含速率） | 超管 |

### 6.17 后台：配置中心（超管）

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /api/admin/configs` | 配置项与当前值（密文掩码） | 超管 |
| `PUT /api/admin/configs` | 保存配置（类型/范围校验、AES 加密、热更新、审计） | 超管 |

### 6.18 后台：协议 / 词表（超管）

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /api/admin/agreements` | 协议与版本列表 | 超管 |
| `POST /api/admin/agreements/:key/publish` | 发布新版本 | 超管 |
| `POST /api/admin/agreement-versions/:id/rollback` | 回滚版本 | 超管 |
| `GET/POST/DELETE /api/admin/words[/:id]` | 词表管理 | 超管 |

### 6.19 后台：日志 / 运维 / 水印（超管）

| 方法 · 路径 | 用途 | 权限 |
| --- | --- | --- |
| `GET /api/admin/logs/access` | 访问日志（筛选/分页/详情） | 超管 |
| `GET /api/admin/logs/audit` | 审计日志 | 超管 |
| `GET /api/admin/ops/overview` | 运维面板（磁盘/DB/COS/健康矩阵） | 超管 |
| `POST /api/admin/watermark/extract` | 盲水印提取 | 超管 |

### 6.20 内部接口（Go interface，供模块解耦与降级）

| 接口 | 位置 | 职责 |
| --- | --- | --- |
| `cache.Cache` | `internal/cache` | `Get/Set/Incr/Del/Close`，Redis 或内存实现 |
| `storage.Storage` | `internal/storage` | 上传/读取/删除/签名 URL，本地或 COS 实现 |
| `moderation.Moderation` | `internal/moderation` | 文本/图片审核提交与结果解析 |
| `mailer.Mailer` | `internal/mailer` | 发信（`Dynamic` 按配置快照重建） |
| `ai.Client` | `internal/ai` | OpenAI 兼容审核调用（配置热生效） |
| `permission.Resolver` | `internal/permission` | 端点/页面权限解析与缓存失效 |

---

## 7. 运行环境与约束条件

### 7.1 运行依赖与版本

- Go 1.21（后端编译）；Node.js（前端 `npm run build`）。
- PostgreSQL（必需，启用 `pg_trgm` 用于模糊搜索）。
- Redis（可选，未配置降级内存缓存）。
- Nginx、systemd；`fonts-noto-cjk`（仅在需要服务端水印字体时）。
- 外部可选：腾讯云 COS + 数据万象、SMTP、OpenAI 兼容 AI、微信/QQ 开放平台。

### 7.2 关键环境变量（`.env`，`internal/config/config.go`）

| 变量 | 说明 | 默认 |
| --- | --- | --- |
| `SERVER_HOST` / `SERVER_PORT` | 监听地址/端口 | 空 / `80`（生产 `127.0.0.1` / `10002`） |
| `DB_HOST/PORT/USER/PASSWORD/NAME/SSLMODE` | PostgreSQL 连接 | `127.0.0.1/5432/tzwall//tzwall/disable` |
| `REDIS_ADDR` / `REDIS_PASSWORD` / `REDIS_DB` | Redis（留空降级） | 空 |
| `JWT_SECRET` | JWT 签名密钥 | 开发缺省（生产必须改） |
| `AES_KEY_B64` | 敏感配置加密密钥（32 字节 base64） | 开发缺省 |
| `COS_SECRET_ID/KEY/BUCKET/REGION/BASE_URL` | COS 存储与审核 | 空（本地存储） |
| `COS_AUDIT_BIZ_TYPE` / `COS_CALLBACK_TOKEN` | 审核策略模板 / 回调校验 | 空 |
| `SMTP_HOST/PORT/USER/PASS/FROM` | 邮件 | 空（不发送） |
| `WECHAT_MP_APPID/SECRET`、`QQ_OAUTH_*`、`WECHAT_WEB_*` | 第三方登录（后台配置优先） | 空 |
| `SITE_NAME` | 站点名 | 泰兴中学校园墙 |
| `TRUSTED_PROXIES` | 可信代理 IP 列表 | `127.0.0.1` |
| `ADMIN_USERNAME/PASSWORD/EMAIL/REAL_NAME/SCHOOL` | 种子超管（仅库空时写入） | `admin` 等 |
| `MAINTENANCE_FLAG_PATH` / `MAINTENANCE_HTML_PATH` | 维护标志/维护页 | `/opt/tzwall/run/...` |
| `MEDIA_CACHE_DIR` / `MEDIA_CACHE_MAX_GB` | 源站媒体缓存 | `./data/media_cache` / `10` |

### 7.3 配置中心（后台 `系统配置`，`config/registry.go`）

- 分 9 组：站点、注册与登录、发帖与评论、图片、审核与 AI、举报、日志、运维、页脚备案，共 40+ 项。
- 类型：`int / bool / string / secret / text`；`int` 带 `Min/Max` 校验；`secret` 以 AES 加密存储且不可回显；`Restart=true` 项（媒体缓存目录/上限）需重启生效。
- 保存即热更新（多数项），并写审计日志。

### 7.4 端口、资源与网络

- 后端：`127.0.0.1:10002`（仅本机，配合 Nginx）。
- Nginx：`10001`；静态 `frontend/dist`；`client_max_body_size 512m`。
- systemd：`MemoryMax=512M`、`Restart=on-failure`、`NoNewPrivileges=true`。
- 部署链：浏览器 → EdgeOne（SSL）→ Nginx(10001) → 后端(10002)。

### 7.5 数据库与索引约束

- `Migrate()` 幂等：`AutoMigrate` 全模型 + 创建 `pg_trgm` 扩展与 GIN 搜索索引、列表/举报/处罚/协议等复合索引。
- 唯一约束：`users.username/email`、`reactions(user_id,target_type,target_id)`、`polls` 投票唯一、`conversations(user_low,user_high)`、`reports(user_id,target_type,target_id)` 等。
- 软删除：`User`、`Submission`、`Poll` 使用 `gorm.DeletedAt`。

---

## 8. 构建、部署与运维

### 8.1 构建

```bash
# 后端
cd backend && go build -o tzwall ./cmd/tzwall

# 前端
cd frontend && npm install && npm run build   # 产物 frontend/dist
```

### 8.2 部署（Nginx + systemd）

```bash
sudo cp backend/tzwall /opt/tzwall/backend/
sudo cp backend/.env  /opt/tzwall/backend/.env
sudo cp -r frontend/dist /opt/tzwall/frontend/dist
sudo useradd -r -s /usr/sbin/nologin tzwall
sudo chown -R tzwall:tzwall /opt/tzwall && sudo chmod 600 /opt/tzwall/backend/.env
sudo cp deploy/tzwall.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now tzwall
sudo cp deploy/nginx.conf.example /etc/nginx/sites-available/tzwall
sudo ln -s /etc/nginx/sites-available/tzwall /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

首次启动自动迁移表结构并写入种子（超管、系统配置、预选标签、协议占位）。

### 8.3 运维

- 日志：`journalctl -u tzwall`；Nginx 日志见 `nginx.conf.example`。
- 健康：`GET /api/health`。
- 备份：`deploy/backup.service` + `backup.timer`（每日，保留 7 份）。
- 维护：开启 `maintenance_enabled` 后 Nginx 直接返回维护页（`/api`、`/admin` 仍可用）。
- 运维面板：`GET /api/admin/ops/overview`（磁盘/DB/COS 用量、备份列表、健康矩阵、阈值告警）。

---

## 9. 已知限制与待改进

1. **限流依赖 Redis**：未配置 Redis 时端点限流整体跳过（仅内存缓存），高并发防护减弱。
2. **审核外部依赖强**：内容安全/审核回调不可用时统一转人工，批量场景下人工压力大；审核策略模板需在云侧维护。
3. **AI 审核为单模型串行**：无重试退避与多模型兜底，超时/超限即转人工；提示词为单一 JSON 约定，健壮性依赖模型输出。
4. **置顶排序为查询期计算**：过期置顶在查询层视为普通帖，但 `pinned` 字段不会自动回写为 `false`（历史数据可能残留 `pinned=true` + 过期时间），维护时需留意统计口径。
5. **搜索性能**：依赖 `pg_trgm` GIN 索引；扩展缺失时降级为前缀匹配，长文本模糊搜索性能下降。
6. **媒体源站缓存需重启**：`media_cache_dir` / `media_cache_max_gb` 修改需重启（`Restart=true`）；缓存为 LRU，容量需结合磁盘监控。
7. **单机部署**：systemd 单实例 + 内存缓存/限流计数，未做多实例共享会话与分布式限流；水平扩展需外置 Redis 与共享存储。
8. **前端包体较大**：`index` / `ContentMarkdown` 等 chunk 超过 500KB（构建告警），可进一步按路由/依赖拆包。
9. **日志体量与隐私**：访问日志接近全量（Header/请求体摘要，敏感字段脱敏），需关注保留期与磁盘占用；IP 仅超管可见。
10. **第三方登录配置分散**：凭证可来自后台配置或 `.env`（后台优先），排查时需同时核对两处。
11. **协议/词表/配置变更热更新**：多数即时生效，但个别项（媒体缓存）需重启；变更应有审计与回滚预案。
12. **小程序与服务端契约**：`miniprogram/` 与后端接口需同步演进，缺少自动化契约测试。

---

## 10. 附录

### 10.1 业务错误码（`pkg/errs`）

| Code | 含义 |
| --- | --- |
| `0` | 成功 |
| `40000` | 请求参数错误 |
| `40100` | 未登录或登录失效 |
| `40300` | 权限不足 |
| `40400` | 资源不存在 |
| `42900` | 请求过于频繁 |
| `50000` | 服务器内部错误 |
| `41001`–`41027` | 业务细分（用户名/邮箱/封禁/禁言/未激活/凭证/锁定/Token/协议/端点无权/违禁词/标签待审/不可见/评论删除/态度冲突/重复举报/图片过大/标签超限/校验/强制改密/警告待确认/IP 封禁/第三方不可用/身份冲突/未关联/资料不完整） |

### 10.2 常用命令

```bash
# 健康检查
curl -s http://127.0.0.1:10002/api/health
# 服务状态 / 日志
systemctl status tzwall && journalctl -u tzwall -f
# 数据库
sudo -u postgres psql -d tzwall
# 前端本地开发（代理 /api 到 8080）
cd frontend && npm run dev
```

### 10.3 相关文档

- `docs/CONFIG.md`：环境变量、部署、后台配置、图片签名/盲水印、邮件、权限、备份、上线清单。
- `docs/MODERATION_CALLBACK.md`：内容审核回调配置与排障。
- `docs/图片盲水印.md`、`docs/微信小程序改造指南.md`、`docs/点歌接口接入指南.md`、`docs/网络优化.md`、`docs/DEMO.md`。

---

> 维护约定：新增端点须在 `permission/registry.go` 登记（方法+路径+分组+默认角色），新增配置项须在 `config/registry.go` 登记；否则权限矩阵/配置中心与前端表单无法感知。
