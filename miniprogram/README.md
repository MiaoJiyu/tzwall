# MeshPoint小助手 · 微信小程序

MeshPoint 账号体系的配套小程序，定位为**账号登录状态查询与个人登录信息记录工具**。

- 小程序名称：MeshPoint小助手
- 小程序简称：MP助手
- 服务类目：工具 > 信息查
- 生产环境后端：`https://tzwall.miaojiyu.top`

## 登录模型（重要）

后端已改为「**小程序首次登录必须填写用户名+密码注册**」，不再按 openid 自动建号：

| 场景 | 后端返回 | 小程序行为 |
| --- | --- | --- |
| 该微信已绑定账号 | `{token, user, is_new:false}` | 直接进首页 |
| 该微信首次登录（未绑定） | `{need_register:true, ticket}` | 跳注册页，填用户名+密码 |
| 已有账号想绑定微信 | 网页生成 8 位绑定码 | 小程序输入绑定码完成绑定 |

注册票据 `ticket` 有效期 10 分钟、一次性；绑定码有效期 10 分钟、一次性。

## 功能定位

| 页面 | 功能 |
| --- | --- |
| 协议授权页 | **首次启动必现**：列出协议清单，复选框默认不勾选，须用户主动勾选后才能继续；点「不同意」退出小程序 |
| 协议详情页 | 从后端拉取协议正文（Markdown 转块级渲染），协议入口全部可点击打开 |
| 首页 | 无需登录即可浏览：服务说明、登录状态查询；登录按钮由用户主动点击触发 |
| 我的 | 个人登录信息记录：本机登录记录（最多 20 条）、个人资料编辑、微信/QQ 绑定状态、协议入口、清空记录、退出登录 |
| 登录页 | 进入不自动发起授权，点击「微信登录」后才调用 `wx.login` |
| 注册页 | 首次登录设置账号名与密码（**本站账号，非微信账号**），须勾选协议才能提交 |
| 绑定页 | 输入网页生成的 8 位绑定码，把微信绑定到已有账号 |
| 网页登录确认 | 复用同一确认页，按 `scene` 前缀分流：网页登录票号自动确认；`bind:<token>` 扫码即绑微信 |

## 文件结构

本工程已直接部署在仓库根目录（即微信开发者工具的项目目录）：

```
.
├── app.js                  # 启动时静默校验/自动登录
├── app.json                # 页面路由 + tabBar
├── app.wxss                # 全局样式
├── sitemap.json
├── project.config.json     # 小程序工程配置（appid / 本地不校验域名）
├── config/index.js         # BASE_URL 配置 + 相对图片地址补全
├── utils/
│   ├── request.js          # 统一请求：BASE_URL、Bearer、错误透传 code、401 自动重登
│   ├── auth.js             # 登录/注册/绑定流程 + 本机登录记录 + 静默会话校验
│   └── agreement.js        # 协议同意状态管理（拉取清单/正文、记录同意行为）
└── pages/
    ├── index/              # 首页：无需登录即可浏览 + 登录状态查询
    ├── login/              # 登录（用户点击后才发起授权）
    ├── register/           # 首次注册（本站账号名+密码，须勾选协议）
    ├── bind/               # 绑定码绑定微信
    ├── web-login/          # 网页授权确认（网页登录扫码即登 / 扫码即绑微信 / 手动票号）
    ├── agreement/          # 协议授权页 + 协议详情页
    └── profile/            # 我的：个人登录信息记录
```

> 小程序代码完全自包含：所有 `require()` 均在项目内解析（`utils/`、`config/`），无任何对后端目录的文件依赖。
> 可上传的独立副本位于仓库根目录的 `mp-upload/`（不含 `tzwall/`、`tzwall1/`）。

## 一、配置 AppID

`project.config.json` 中的 `appid` 必须与后台「系统配置 → 注册与登录 → 微信小程序 AppID」一致（否则 `jscode2session` 失败）：

```json
{ "appid": "wx8780ee57ab9db24a" }
```

> 当前为占位值，请替换为 MeshPoint小助手 的正式 AppID。

## 二、后端地址

`config/index.js` 已配置为生产环境地址：

```js
const BACKEND_URL = 'https://tzwall.miaojiyu.top';          // 生产/真机
const LOCAL_URL   = 'http://118.195.194.223:10001';         // 内网直连调试备用
```

`DEV_BASE` 与 `PROD_BASE` 均指向 HTTPS 域名，开发者工具、真机、正式版行为一致。
需要直连内网后端时，把 `DEV_BASE` 改为 `LOCAL_URL`。

## 三、接口契约

统一响应体 `{code, message, data}`，成功 `code=0`。鉴权头 `Authorization: Bearer <token>`。

| 接口 | 方法 | body | 返回 data |
| --- | --- | --- | --- |
| `/api/auth/wechat/login` | POST | `{code}` | `{token,user,is_new:false}` 或 `{need_register:true,ticket}` |
| `/api/auth/wechat/register` | POST | `{ticket,username,password}` | `{token,user,is_new:true}` |
| `/api/auth/wechat/bind-code/confirm` | POST | `{bind_code,code}` | `null`（成功即 code=0） |
| `/api/auth/wechat/bind-code` | POST | —（需 token） | `{code,expires_min:10}` |
| `/api/auth/web-login/confirm` | POST | `{ticket,code}` | `null`（网页扫码即登，见下） |
| `/api/auth/wechat/bind/confirm` | POST | `{bind_token,code}` | `null`（扫码即绑微信，见下） |
| `/api/auth/me` | POST | —（需 token） | `user`（含 `wechat_bound`/`qq_bound`） |
| `/api/auth/profile` | POST | `{nickname,avatar,wechat,qq,phone}` | `null` |
| `/api/meta/agreements` | GET | — | `{items:[{key,title,version,effective_at}]}` |
| `/api/meta/agreement` | GET | `?key=user\|privacy\|data` | `{key,title,version,content}` |
| `/api/images` | POST | multipart，字段 `file` | `{id,content_type,url}` |

### 错误码与处理

| code | 含义 | 小程序处理 |
| --- | --- | --- |
| 0 | 成功 | — |
| 40100 | 登录失效（HTTP 401） | 清 token → 自动重新登录 → 重放原请求；若该微信未绑定则回登录页 |
| 41009 | 票据/绑定码/登录会话无效或过期 | 注册：回登录页重新取票；绑定：提示重新生成；网页票号：提示在网页刷新 |
| 41001 | 用户名不符合规则（6-12 位） | toast 提示 |
| 41002 | 用户名已被注册 | toast 提示（引导改用绑定码） |
| 41020 | 参数校验失败 | toast 透传 message（如「该微信尚未注册账号，请先在小程序完成注册」「该登录会话已被使用」） |
| 41024 | 第三方登录未配置或不可用 | toast 透传 message |
| 41025 | 该微信已绑定其他账号 | toast 透传 message |
| 42900 | 请求过于频繁 | toast 透传 message |
| 其他非 0 | 业务错误 | toast 透传 `message` |

> `utils/request.js` 只把 **HTTP 401 / 业务码 40100** 当作登录失效触发重登。业务码 41009 在注册/绑定/网页登录接口上表示票据过期，必须透传给页面，否则会误触发重登循环。

### 解绑后的状态同步（重要）

网页「解绑微信/QQ」会使该账号**既有 token 全部失效**（后端 `token_version + 1`）。小程序持有的旧 token 下一次请求会收到 `40100`：

1. `request.js` 自动清 token → 重新 `wx.login` → `/auth/wechat/login`；
2. 此时该微信已解绑，接口返回 `need_register`（而非 token），于是回登录页走注册流程；
3. 因此**不会**再显示「微信登录成功」——绑定状态一律以 `/auth/me` 的 `wechat_bound` 为准，不用本地成功标记。

## 四、本地联调

1. 微信开发者工具直接打开本目录（已部署在根目录）。
2. 「详情 → 本地设置 → 不校验合法域名」保持勾选。
3. 编译运行：进入首页会自动 `wx.login` → `POST /api/auth/wechat/login`。
4. 首次登录会跳注册页；已有账号请在网页端「个人中心 → 绑定微信」生成绑定码，再到小程序绑定页输入。

## 五、真机预览 / 上线

需在小程序后台「开发管理 → 服务器域名 → request 合法域名」添加：

```
https://tzwall.miaojiyu.top
```

证书为 Let's Encrypt，**2026-12-24 到期**，注意续期。

## 六、上传打包配置（重要）

本仓库根目录同时存放后端代码（`tzwall/`、`tzwall1/`，共约 9.6 MB）。若直接在根项目上传，必须用 `packOptions.ignore` 排除，否则超出小程序主包 2 MB 上限。

**注意 `ignore` 是对象数组，元素必须是 `{type, value}`，写字符串会报 `字段需为 object`。**

```json
"packOptions": {
  "ignore": [
    { "type": "folder", "value": "tzwall" },
    { "type": "folder", "value": "tzwall1" },
    { "type": "folder", "value": "mp-upload" },
    { "type": "suffix", "value": ".md" }
  ],
  "include": []
}
```

`type` 可选值：`folder`、`file`、`suffix`、`prefix`、`regexp`、`glob`；路径以 `miniprogramRoot` 为根目录；`include` 优先级高于 `ignore`。

另一种更稳妥的方式：使用仓库根目录下已生成好的独立工程 `mp-upload/`（仅 43 KB，只含小程序代码），用开发者工具直接打开该目录上传，与后端目录彻底隔离。

## 七、安全约束

- 小程序代码中**严禁出现 AppSecret**；openid 只能由后端 `jscode2session` 获取，不信任客户端传入。
- `code` 一次性、5 分钟内有效；注册票据与绑定码 10 分钟有效且一次性。
- 登录记录仅存于本机 `Storage`，不上传服务器。

## 七、验收清单

- [ ] 首次登录 → 跳注册页 → 填用户名+密码 → 进首页，账号已绑定微信
- [ ] 再次登录 → 直接进首页（不再要求注册）
- [ ] 网页拿绑定码 → 小程序输入 → 绑定成功；网页刷新显示「已绑定」
- [ ] 无效/过期绑定码 → 友好提示（41009）
- [ ] 用户名已存在 → 友好提示（41002）
- [ ] token 失效（40100）→ 自动重新登录，无需绑定则直接恢复
- [ ] 网页「微信登录 → 小程序登录」→ 扫码 → 自动确认 → 网页自动登录
- [ ] 手动进入网页登录确认页输入票号 → 同样确认成功
- [ ] 网页「绑定微信」小程序码（scene=bind:<token>）扫码进入 → 自动绑定，无需手动输入授权码
- [ ] 绑定令牌过期/已使用 → 提示「绑定已失效，请在网页刷新后重试」（41009）
- [ ] 未注册的微信确认 → 提示「请先在小程序完成注册」（41020）
- [ ] 网页解绑微信 → 小程序旧 token 失效 → 重新登录走注册流程，不再显示已绑定
- [ ] 真机预览可完成上述流程
- [ ] 首页仅展示账号登录状态与登录记录，不含任何第三方站点内容

## 九、隐私合规整改（针对审核驳回）

微信审核驳回的三类问题与对应整改：

| 驳回原因 | 整改措施 |
| --- | --- |
| 默认自动同意《用户协议》及《隐私政策》 | 新增 `pages/agreement/agreement`：首次启动必现，列出协议清单，**复选框默认不勾选**，必须用户主动勾选才能点「同意并继续」；点「不同意」弹窗确认后 `wx.exitMiniProgram()` 退出，不进入任何功能、不发起任何数据请求。同意行为（协议清单+版本+时间）写入本机存储 |
| 一进入首页即要求授权手机号码、头像、昵称登录 | **三层整改**：① 首页改为**免登录内容浏览页**（公告 + 最新投稿），用户可先体验功能服务；② 登录入口移至页面底部次要按钮，由用户主动点击才调用 `wx.login`；③ 移除全部微信用户信息授权控件（详见下表） |
| 【隐私政策】点击无响应 | 新增 `pages/agreement/doc`：从 `GET /api/meta/agreement?key=` 拉正文，Markdown 转块级结构渲染；登录页、注册页、协议授权页、我的页的协议入口均已接入且可打开 |

### 微信用户信息授权控件：已全部移除

| 原控件 | 触发的问题 | 整改后 |
| --- | --- | --- |
| `<button open-type="chooseAvatar">` | 点击弹出「使用微信头像」授权 | 改为 `<view bindtap="onPickAvatar">` + `wx.chooseMedia` 从相册/相机选普通图片 |
| `<input type="nickname">` | 聚焦弹出「使用微信昵称」授权 | 改为普通 `<input>`，由用户自行输入 |
| `<input type="number">` 手机 | 被判定为收集手机号 | **整项移除**，小程序内不再出现手机号输入；已存在的手机号仅原样回传不修改 |
| 首页 `微信登录` 主按钮 | 一进入即诱导登录 | 移至页面底部，`btn-mini` 次要样式，文案改为「登录」 |

### 逐条对应平台运营规范 3.4

| 条款 | 落实情况 |
| --- | --- |
| 3.4.1 收集数据须明确告知用途并经同意 | 协议授权页明确告知；首页/我的页声明「不收集手机号、地理位置、通讯录」 |
| 3.4.2 不得请求或诱导输入微信用户名密码 | 注册页显著说明填写的是「MeshPoint 本站账号」，与微信账号/密码无关 |
| 3.4.3 不得加入搜索小程序功能 | 未实现 |
| 3.4.4 不得出售/转交/泄露用户数据 | 未接入任何第三方 SDK，无数据外传 |
| 3.4.5 未经授权不得显示头像昵称 | 头像由用户从相册主动选择、昵称由用户主动输入，且仅在本人账号内展示 |
| 3.4.6 不得要求降低系统安全性 | 未要求 |
| 3.4.7 地理位置追踪须可退出 | 未获取任何位置信息 |

### 未使用的敏感接口（已核查）

`getPhoneNumber`、`getUserProfile`、`getUserInfo`、`chooseAvatar`、`type="nickname"`、`chooseAddress`、`getLocation`、`readContact` **均未使用**。整个小程序只调用 `wx.login`（静默、无弹窗）与 `wx.chooseMedia`（用户主动点击头像后触发）。

> 注意：`utils/request.js` 新增了 `noRetry` 选项，`utils/auth.js` 的 `checkSession()` 用它实现「只校验、不自动重登」，这是「不强制登录」整改的关键，请勿移除。

> 注意：`utils/request.js` 新增了 `noRetry` 选项，`utils/auth.js` 的 `checkSession()` 用它实现「只校验、不自动重登」，这是「不强制登录」整改的关键，请勿移除。

## 十、后端改动

**本次未修改任何后端代码**，全部沿用后端仓库（`tzwall2/`）中已实现的接口。

新增页面 `pages/web-login/web-login` 依赖后端四个公开端点（已在 `router.go` 核实）：

```
POST /api/auth/web-login/session   → {ticket, expires_in:300}
POST /api/auth/web-login/qrcode    → {image_base64}        （带 scene 的小程序码）
POST /api/auth/web-login/confirm   {ticket, code} → code=0
POST /api/auth/web-login/poll      {ticket} → {status, exchange_code}
```

协议内容取自后端 `seed.go` 中预置的三份协议（后台可编辑）：`user` 用户协议、`privacy` 隐私协议、`data` 数据保护协议。**正式上线前请先在后台把占位文本替换为正式协议内容**，否则协议详情页展示的是占位文案。

> 小程序侧只需调用 `confirm`；`session`/`qrcode`/`poll` 由网页端使用。
> 小程序码的跳转页面取后台配置「小程序登录确认页路径」，默认 `pages/web-login/web-login`，
> **必须与 `app.json` 注册的页面路径一致**（已一致）。

### 扫码即绑微信（与网页登录复用同一确认页）

网页「个人中心 → 绑定微信」生成的小程序码，`scene` 编码为 `bind:<token>`（token 16 位 hex，见后端 `AuthService.CreateWechatBindMpCode`，10 分钟过期、消费即删）。确认页 `onLoad` 按 `scene` 前缀分流：

- `scene` 以 `bind:` 开头 → 调 `bindWechat(token)` → `wx.login()` 拿 `code` → `POST /api/auth/wechat/bind/confirm {bind_token, code}`，**全程无需用户手动输入任何授权码**；
- 其他情况视为网页登录票号 → 走原 `confirm()` 流程。

`/api/auth/wechat/bind/confirm` 为**公开接口**（后端 `perm.Public`），仅需 `bind_token + wx.login code`，无需小程序登录态，安全性由一次性令牌保证。与既有的 `pages/bind/bind`（手动 8 位授权码）并存，互为兜底。

### 摄像头扫码（两页共用）

`pages/web-login/web-login` 与 `pages/bind/bind` 均提供「扫二维码自动填入」按钮，调用 `utils/scan.js` 封装的 `wx.scanCode`：

- 小程序运行于微信环境，`wx.scanCode` 原生调起摄像头，无需 H5 的 `getUserMedia`，在微信内置浏览器与主流移动端均原生支持；
- 扫描界面为微信原生全屏样式（无法自定义），风格一致性体现在按钮（复用 `.btn-plain`）与提示文案上；
- 结果解析：`web-login` 页抽取 12 位票号（`extractTicket`），`bind` 页抽取 8 位绑定码（`extractBindCode`），同时兼容「纯码 / URL 带 `?code=` 参数 / 混杂文本中的码段」三种形式；识别到 `bind:<token>` 前缀则直接走扫码即绑；
- 识别成功自动填入并触发对应流程；失败/无码给出友好 toast；**用户拒绝摄像头授权**时弹窗引导前往 `wx.openSetting` 开启（用户主动取消不提示）。
- 摄像头权限为系统级，`wx.scanCode` 不依赖任何 request 合法域名，无需后端配置。
