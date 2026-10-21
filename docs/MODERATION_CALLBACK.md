# 违规图片/文本审核回调配置指南

> 本文说明如何把 **腾讯云 COS「内容审核」**（含自定义风险库、自动冻结）与本站的审核回调打通，使违规图片/内容在判定后**自动停止展示**。
> 项目整体配置见 [`CONFIG.md`](./CONFIG.md)。

---

## 一、两条审核链路（先分清）

| 链路 | 触发方 | 关联方式 | 本站处理 |
|---|---|---|---|
| **A. 站点提交的 CI 审核** | 后台上传/投稿时调用数据万象 `POST /image/auditing`、`/text/auditing` | 返回 `JobId`，写入 `moderation_tasks` | 回调按 `job_id` 关联任务，联动 `submission_images.audit_status` / `submissions.audit_status` |
| **B. COS 控制台「内容审核」** | 存储桶**自动审核配置**（可带**自定义风险库**） | 回调携带对象 `Object`（对象 Key） | 按 `storage_key` 直接标记该图片违规私有，无需 job_id |

> 本指南重点是 **B**：即使站点没有主动调用审核，控制台自动审核命中后也能让网站下架图片。

**视频同理**：上传到 COS 的视频走链路 B 的「视频审核」自动审核配置，回调携带视频对象的 `Object`，本站按 `storage_key` 在 `submission_videos` 中标记违规私有并立即驳回所属投稿；未命中违规的视频进入人工审核（链路 A 不涉及视频）。回调地址、令牌留空等要求与图片完全一致。

---

## 二、回调地址

```
POST https://<你的站点域名>/api/moderation/callback
```

- 该端点为 **公开接口**（无需登录），用于接收腾讯云回调。
- **必须可被公网 HTTPS 访问**（COS 回调从腾讯云侧发起）。
- 若前面有 Nginx/EdgeOne：确保 `/api/` 正常反代到后端，`proxy_read_timeout` 足够。
- **`COS_CALLBACK_TOKEN` 必须留空**：COS 控制台的「内容审核」回调**不会**携带本站自定义头 `X-Cos-Callback-Token`；一旦配置了令牌，回调会被 401 拒绝。
  （该令牌仅用于链路 A 中由本站主动提交的 CI 任务回调场景。）

---

## 三、COS 控制台配置步骤

1. 进入 **对象存储控制台 → 选择存储桶 → 内容审核 → 自动审核配置**。
2. 选择 **图片审核**（必要时同时配置 **文本审核**），添加配置：
   - **审核范围/路径**：按需（如整个存储桶或 `images/` 前缀；本站对象为 UUID 文件名，建议整桶）。
   - **审核策略/风险库**：选择你要启用的策略，勾选涉黄/违法违规/广告等场景；如需自定义敏感词/图片，先创建**自定义风险库**并在策略中关联。
   - **冻结设置**（关键）：
     - 可选择 **将文件变为私有读**（冻结后访问返回 **403**）
     - 或 **将文件转移到备份目录**（移动到 `audit_freeze_backup/increment_audit`，访问返回 **404**）
     - 建议勾选「冻结后刷新 CDN」（如使用了 CDN）。
   - **回调设置**：
     - 回调地址：`https://<你的站点域名>/api/moderation/callback`
     - 回调版本：优先选 **详细回调（Detail）**（字段全，含 `Object`）；如只能选简洁回调也可用。

> 提示：DETAIL 回调需**提交任务时使用 `Object` 参数**才会返回 `Object`（对象 Key）；SIMPLE 回调只返回 `url`，本站会从 URL 中解析对象 Key。

---

## 四、回调报文格式与本站处理

本站 `/api/moderation/callback` 同时兼容 **DETAIL** 与 **SIMPLE** 两种结构（不依赖版本头）。

### 1. 详细回调（Detail）

```json
{
  "EventName": "ReviewImage",
  "JobsDetail": {
    "JobId": "job-xxxx",
    "State": "Success",
    "Object": "3da4bf84-ae10-4c5a-8619-92c9f99dba14.png",
    "Label": "Porn",
    "Result": 1,
    "Score": 96,
    "ForbidState": 1,
    "BucketId": "tzwall-1321423822",
    "Region": "ap-nanjing"
  }
}
```

- **对象 Key**：`JobsDetail.Object`
- **违规判定**：`Result = 1`（确认违规）或 `ForbidState ≥ 1`（1 已冻结 / 2 已转移）
- 本站动作：按 `storage_key = Object` 将该图片置为 `audit_status=block`、`is_private=true`，并**立即驳回该图片所属投稿**（`status=rejected`、`audit_status=block`，驳回原因写入 `admin_note`，对作者可见）。

### 2. 简洁回调（Simple）

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "event": "ReviewImage",
    "result": 1,
    "forbidden_status": 1,
    "url": "https://<bucket>.cos.<region>.myqcloud.com/3da4bf84-....png"
  }
}
```

- **对象 Key**：从 `data.url` 剥离域名与路径前缀得到。
- **违规判定**：`data.result = 1` 或 `data.forbidden_status ≥ 1`。

### 3. 本站处理流程

```
收到回调
  ├─ 若是我方提交的 CI 任务（有 JobId 且命中 moderation_tasks）→ 更新任务状态，命中违规时同样驳回投稿
  └─ 否则按「内容审核回调」解析 Object
        ├─ 命中违规 → submission_images(audit_status=block, is_private=true)
        │             + submissions(status=rejected, audit_status=block, admin_note=驳回原因)
        └─ 未命中 → 仅确认接收
```

> 时序注意：图片审核回调可能**早于投稿创建**（此时图片尚未关联投稿，无法直接驳回投稿）。因此创建投稿关联图片时会再次校验图片状态：只要存在 `audit_status=block` 的图片，投稿立即置为 `rejected`，避免漏判。

标记 `block/private` 后：
- 所属投稿立即被驳回（`status=rejected`），从公开列表/详情中消失；
- 稿件接口不再返回该图（查询统一过滤 `audit_status<>'block' AND is_private=false`）；
- 图片代理 `/api/images/:id` 直接返回 **403**；
- 前端 `<img>` 加载失败自动隐藏。

---

## 五、兜底：访问探测（即使没配回调也能下架）

为避免"没配回调 / 回调丢失"导致违规图继续展示，图片代理内置**访问探测**：

- 每个图片最多每 **60 秒**做一次 `HeadObject`；
- 若返回 **403**（私有读冻结）或 **404**（转移备份目录）→ 判定违规，标记 `block/private`、驳回所属投稿并 403，前端立即不再显示。

> 该兜底与回调互补：回调负责"及时、精准"；探测负责"兜底、无需配置"。

---

## 六、验证（联调步骤）

> 上线后再按此验证；本地可用模拟回调验证解析逻辑。

### 1. 模拟 DETAIL 回调

```bash
KEY=$(psql -h 127.0.0.1 -U tzwall -d tzwall -tAc \
  "select storage_key from submission_images order by id desc limit 1")
curl -s -X POST http://127.0.0.1/api/moderation/callback \
  -H 'Content-Type: application/json' \
  -d "{\"EventName\":\"ReviewImage\",\"JobsDetail\":{\"Object\":\"$KEY\",\"Result\":1,\"ForbidState\":1}}"
# 预期 {"code":0,...}
psql -h 127.0.0.1 -U tzwall -d tzwall -tAc \
  "select audit_status,is_private from submission_images where storage_key='$KEY'"
# 预期 block|t
```

### 2. 模拟 SIMPLE 回调

```bash
curl -s -X POST http://127.0.0.1/api/moderation/callback \
  -H 'Content-Type: application/json' \
  -d "{\"code\":0,\"data\":{\"result\":1,\"forbidden_status\":1,\"url\":\"https://<bucket>.cos.<region>.myqcloud.com/$KEY\"}}"
```

### 3. 校验图片接口与列表

```bash
# 图片代理应 403（签名链接本身需有效；此处直接验证已标记后的下架行为）
curl -s -o /dev/null -w "%{http_code}\n" "http://127.0.0.1/api/images/<id>?...&e=...&u=...&s=..."
# 稿件列表不再包含该图
```

### 4. 真实回调

在 COS 控制台「内容审核」里查看审核结果与回调记录；或上传一张命中**自定义风险库**的测试图，观察：
- 对象被冻结（私有读/转移备份目录）；
- 站点数据库该图 `audit_status=block`；
- 前台/后台该图不再显示。

---

## 七、存量对象 ACL 校正

如果曾把对象设为公有读、或不确定对象 ACL，可用管理接口统一设为私有（幂等）：

```bash
curl -X POST http://127.0.0.1/api/admin/images/sync-private \
  -H "Authorization: Bearer <SUPER_ADMIN_TOKEN>"
# 返回 {"total":N,"private":N}
```

---

## 八、常见问题

| 现象 | 排查 |
|---|---|
| 回调 401 | 配置了 `COS_CALLBACK_TOKEN`；使用内容审核控制台回调时须**留空** |
| 回调无效果 | 地址不可公网访问 / HTTPS 证书问题 / Nginx 未反代 `/api/` |
| 命中违规但未下架 | 回调 `Object` 为空（提交任务用了 URL 参数）；改用 `Object` 提交或用站点探测兜底 |
| 图片仍显示 | 浏览器缓存了旧图；刷新页面（重新签发链接）；确认数据库 `audit_status=block` |
| 使用 CDN | 冻结后需刷新 CDN（控制台勾选"冻结后刷新 CDN"），否则边缘缓存仍可访问旧图 |
| 探测未下架 | 冻结方式为"私有读"但后端密钥是该桶所有者仍可读取；请**配置回调**为主 |
| 跨桶/多地域 | DETAIL 回调含 `BucketId`/`Region`，多桶部署需确保本站回调只处理本桶对象 |

---

## 九、与图片方案的关系（当前生产形态）

- 对象：**私有读写**；浏览器不直连 COS。
- 图片地址：后端签发**签名时效链接** `/api/images/{id}?e=&u=&s=`，含防盗链与有效期。
- 展示：代理取流时写入**盲水印**（用户ID+IP+时间，数据万象 `watermark/3/type/3/text/<base64>`）。
- 违规处置：**回调 + 访问探测**双保险 → 标记 `block/private` → 列表剔除 + 403 + 前端隐藏。
