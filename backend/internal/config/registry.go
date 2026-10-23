package config

// 配置项类型
const (
	TypeInt    = "int"
	TypeBool   = "bool"
	TypeString = "string"
	TypeSecret = "secret"
	TypeText   = "text"
)

// 配置分组
const (
	GroupSite    = "站点"
	GroupAccount = "注册与登录"
	GroupPost    = "发帖与评论"
	GroupMedia   = "图片"
	GroupAI      = "审核与 AI"
	GroupSocial  = "举报"
	GroupLog     = "日志"
	GroupOps     = "运维"
	GroupFooter  = "页脚备案"
)

// ConfigItem 配置项元数据。目录是配置中心的事实源：
// 后端据此校验与渲染，前端据此生成表单；新增配置项必须在此登记。
type ConfigItem struct {
	Key     string
	Group   string
	Label   string
	Type    string // int/bool/string/secret/text
	Default string
	Min     int      // int 类型有效范围
	Max     int      // int 类型有效范围
	Options []string // 枚举候选（预留）
	Restart bool     // true 表示需重启生效
	Desc    string
}

// DefaultAIPrompt 默认 AI 审核提示词：要求严格的结构化输出。
const DefaultAIPrompt = `你是校园墙内容安全审核员。请判断投稿内容是否适合在中学校园社区公开展示。
判定标准：涉及违法违规、色情、暴力、毒品、赌博、政治敏感、人身攻击、霸凌、广告导流、隐私泄露或明显恶意的内容应判不通过。
只输出 JSON，不要输出其他文字：{"pass": true 或 false, "reason": "简要中文原因（20字内）"}`

// ConfigItems 全量配置目录（顺序即展示顺序）。
var ConfigItems = []ConfigItem{
	// 站点
	{Key: "site_name", Group: GroupSite, Label: "站点名称", Type: TypeString, Default: "泰兴中学校园墙", Desc: "显示在标题栏与登录页"},
	{Key: "site_url", Group: GroupSite, Label: "站点访问地址", Type: TypeString, Default: "", Desc: "用于生成邮件中的激活/重置链接，如 https://wall.example.com；留空则回退 http://localhost"},
	{Key: "agreement_required", Group: GroupSite, Label: "强制确认协议", Type: TypeBool, Default: "true", Desc: "协议更新后强制已登录用户重新确认"},

	// 注册与登录
	{Key: "register_rph", Group: GroupAccount, Label: "每小时注册上限（已迁移）", Type: TypeInt, Default: "20", Min: 0, Max: 10000, Desc: "已迁移至「访问权限」矩阵按端点配置，此处不再生效"},
	{Key: "login_rph", Group: GroupAccount, Label: "每小时登录上限（已迁移）", Type: TypeInt, Default: "30", Min: 0, Max: 10000, Desc: "已迁移至「访问权限」矩阵按端点配置，此处不再生效"},
	{Key: "login_fail_user_limit", Group: GroupAccount, Label: "用户登录失败次数", Type: TypeInt, Default: "5", Min: 1, Max: 100},
	{Key: "login_fail_admin_limit", Group: GroupAccount, Label: "管理员登录失败次数", Type: TypeInt, Default: "10", Min: 1, Max: 100},
	{Key: "login_lock_minutes", Group: GroupAccount, Label: "锁定时长（分钟）", Type: TypeInt, Default: "15", Min: 1, Max: 1440},
	{Key: "email_verify_ttl_hours", Group: GroupAccount, Label: "邮箱激活有效期（小时）", Type: TypeInt, Default: "24", Min: 1, Max: 720},

	// 第三方登录（凭证暂未申请，先预留占位；配置后由后台「系统配置」填写或经由 .env 注入）
	{Key: "wechat_mp_appid", Group: GroupAccount, Label: "微信小程序 AppID", Type: TypeString, Default: "", Desc: "微信小程序登录用 AppID；留空则禁用微信登录"},
	{Key: "wechat_mp_secret", Group: GroupAccount, Label: "微信小程序 AppSecret", Type: TypeSecret, Default: "", Desc: "微信小程序登录用 AppSecret；加密存储，留空表示不修改"},
	{Key: "qq_oauth_appid", Group: GroupAccount, Label: "QQ 互联 AppID", Type: TypeString, Default: "", Desc: "QQ OAuth 登录用 AppID；留空则禁用 QQ 登录"},
	{Key: "qq_oauth_secret", Group: GroupAccount, Label: "QQ 互联 AppKey", Type: TypeSecret, Default: "", Desc: "QQ OAuth 登录用 AppKey；加密存储，留空表示不修改"},
	{Key: "qq_oauth_redirect_uri", Group: GroupAccount, Label: "QQ 回调地址", Type: TypeString, Default: "", Desc: "QQ 授权完成后的回调地址，需与 QQ 互联后台配置一致"},
	{Key: "wechat_web_appid", Group: GroupAccount, Label: "微信网页应用 AppID", Type: TypeString, Default: "", Desc: "微信开放平台·网站应用 AppID，用于网页扫码登录/绑定；留空则禁用网页微信"},
	{Key: "wechat_web_secret", Group: GroupAccount, Label: "微信网页应用 AppSecret", Type: TypeSecret, Default: "", Desc: "微信网页应用 AppSecret；加密存储，留空表示不修改"},
	{Key: "wechat_web_redirect_uri", Group: GroupAccount, Label: "微信网页回调地址", Type: TypeString, Default: "", Desc: "微信网页授权回调地址，需与微信开放平台配置一致；留空则用站点地址 + /api/auth/wechat/web/callback"},
	{Key: "social_auto_create", Group: GroupAccount, Label: "社交登录自动建号（QQ/网页微信）", Type: TypeBool, Default: "true", Desc: "开启后，QQ/网页微信首次登录自动创建账号；小程序微信首次登录始终要求填写用户名与密码"},
	{Key: "miniprogram_name", Group: GroupAccount, Label: "小程序名称", Type: TypeString, Default: "泰兴中学校园墙", Desc: "展示在微信小程序码弹窗中"},
	{Key: "miniprogram_login_qrcode", Group: GroupAccount, Label: "登录小程序码图片地址", Type: TypeString, Default: "", Desc: "点击「微信登录/注册」时弹窗展示的小程序码图片 URL"},
	{Key: "miniprogram_login_desc", Group: GroupAccount, Label: "登录说明文案", Type: TypeText, Default: "请使用微信扫描小程序码，在小程序内完成登录/注册。", Desc: "登录弹窗中的说明文字"},
	{Key: "miniprogram_bind_qrcode", Group: GroupAccount, Label: "绑定小程序码图片地址", Type: TypeString, Default: "", Desc: "个人中心点击「绑定微信」时弹窗展示的小程序码图片 URL（与登录码区分）"},
	{Key: "miniprogram_bind_desc", Group: GroupAccount, Label: "绑定说明文案", Type: TypeText, Default: "请使用微信扫描小程序码，在小程序内完成微信绑定。", Desc: "绑定弹窗中的说明文字"},
	{Key: "qq_mail_skip_verify", Group: GroupAccount, Label: "如果是QQ邮箱，直接注册，不进行验证", Type: TypeBool, Default: "false", Desc: "开启后，QQ 邮箱（域名恰为 qq.com）注册跳过邮箱验证直接完成，且不弹提示；关闭则按常规验证流程并提示查收"},

	// 发帖与评论
	{Key: "comment_max_len", Group: GroupPost, Label: "评论字数上限", Type: TypeInt, Default: "500", Min: 10, Max: 5000},
	{Key: "comment_max_depth", Group: GroupPost, Label: "评论嵌套层数", Type: TypeInt, Default: "3", Min: 1, Max: 10},
	{Key: "tag_max_count", Group: GroupPost, Label: "投稿标签数量上限", Type: TypeInt, Default: "5", Min: 1, Max: 20},
	{Key: "song_enabled", Group: GroupPost, Label: "启用点歌", Type: TypeBool, Default: "true", Desc: "关闭后禁止提交点歌，前台隐藏入口"},
	{Key: "song_weekly_limit", Group: GroupPost, Label: "每人每周点歌上限", Type: TypeInt, Default: "1", Min: 0, Max: 100, Desc: "按自然周统计；0 表示不限"},
	{Key: "poll_enabled", Group: GroupPost, Label: "启用投票", Type: TypeBool, Default: "true", Desc: "关闭后禁止发起投票与投票"},
	{Key: "poll_max_options", Group: GroupPost, Label: "投票选项上限", Type: TypeInt, Default: "10", Min: 2, Max: 50},

	// 图片
	{Key: "image_max_size_mb", Group: GroupMedia, Label: "单张图片大小（MB）", Type: TypeInt, Default: "5", Min: 1, Max: 50},
	{Key: "image_max_count", Group: GroupMedia, Label: "投稿图片数量上限", Type: TypeInt, Default: "9", Min: 1, Max: 30},
	{Key: "image_formats", Group: GroupMedia, Label: "允许的图片格式", Type: TypeString, Default: "jpg,jpeg,png,gif,webp", Desc: "逗号分隔，不含点号"},
	{Key: "watermark_cache_seconds", Group: GroupMedia, Label: "图片缓存时长（秒）", Type: TypeInt, Default: "600", Min: 0, Max: 86400, Desc: "图片代理的 Cache-Control 时长；0 表示不缓存"},
	{Key: "image_link_ttl_seconds", Group: GroupMedia, Label: "图片链接有效期（秒）", Type: TypeInt, Default: "600", Min: 30, Max: 86400, Desc: "后端签发的图片链接有效期，过期需刷新页面重新获取"},
	{Key: "image_blind_watermark", Group: GroupMedia, Label: "图片盲水印", Type: TypeBool, Default: "true", Desc: "开启后每张图片经数据万象写入盲水印（用户名+IP+请求时间），需启用 COS 图片处理"},
	{Key: "image_cache_ttl_seconds", Group: GroupMedia, Label: "图片 CDN 缓存（秒）", Type: TypeInt, Default: "604800", Min: 0, Max: 31536000, Desc: "对外图片的 CDN 缓存时长；0 表示不缓存；内容不可变建议较长"},
	{Key: "video_cache_ttl_seconds", Group: GroupMedia, Label: "视频 CDN 缓存（秒）", Type: TypeInt, Default: "86400", Min: 0, Max: 31536000, Desc: "对外视频的 CDN 缓存时长；0 表示不缓存；内容不可变建议 1 天以上"},

	// 视频
	{Key: "video_max_size_mb", Group: GroupMedia, Label: "单个视频大小（MB）", Type: TypeInt, Default: "100", Min: 1, Max: 1024},
	{Key: "video_max_count", Group: GroupMedia, Label: "投稿视频数量上限", Type: TypeInt, Default: "3", Min: 1, Max: 10},
	{Key: "video_formats", Group: GroupMedia, Label: "允许的视频格式", Type: TypeString, Default: "mp4,mov,webm,m4v", Desc: "逗号分隔，不含点号"},

	// 邮件（后台配置优先于 .env）
	{Key: "smtp_host", Group: GroupAccount, Label: "SMTP 服务器", Type: TypeString, Default: "", Desc: "留空则使用 .env 中的配置；均未配置时注册免激活"},
	{Key: "smtp_port", Group: GroupAccount, Label: "SMTP 端口", Type: TypeInt, Default: "465", Min: 1, Max: 65535},
	{Key: "smtp_user", Group: GroupAccount, Label: "SMTP 账号", Type: TypeString, Default: ""},
	{Key: "smtp_pass", Group: GroupAccount, Label: "SMTP 密码", Type: TypeSecret, Default: "", Desc: "加密存储，保存后不可回显；留空表示不修改"},
	{Key: "smtp_from", Group: GroupAccount, Label: "发件人地址", Type: TypeString, Default: "", Desc: "留空默认使用 SMTP 账号"},

	// 审核与 AI
	{Key: "ai_enabled", Group: GroupAI, Label: "启用 AI 审核", Type: TypeBool, Default: "false", Desc: "腾讯云文本审核通过后交由 AI 复审；AI 通过才直接放行"},
	{Key: "ai_base_url", Group: GroupAI, Label: "AI 接口地址", Type: TypeString, Default: "", Desc: "OpenAI 兼容接口，如 https://api.openai.com/v1"},
	{Key: "ai_api_key", Group: GroupAI, Label: "AI 接口密钥", Type: TypeSecret, Default: "", Desc: "加密存储，保存后不可回显；留空表示不修改"},
	{Key: "ai_model", Group: GroupAI, Label: "模型名称", Type: TypeString, Default: "gpt-4o-mini"},
	{Key: "ai_prompt", Group: GroupAI, Label: "审核提示词", Type: TypeText, Default: DefaultAIPrompt, Desc: "要求模型仅输出 JSON：{\"pass\":布尔,\"reason\":\"原因\"}"},
	{Key: "ai_timeout_seconds", Group: GroupAI, Label: "AI 调用超时（秒）", Type: TypeInt, Default: "10", Min: 3, Max: 60, Desc: "超时按异常处理，转人工审核"},
	{Key: "ai_daily_limit", Group: GroupAI, Label: "AI 每日调用上限", Type: TypeInt, Default: "500", Min: 0, Max: 100000, Desc: "0 表示不限；超出按异常处理，转人工"},

	// 举报
	{Key: "report_hour_limit", Group: GroupSocial, Label: "每小时举报上限", Type: TypeInt, Default: "5", Min: 1, Max: 100},
	{Key: "report_day_limit", Group: GroupSocial, Label: "每日举报上限", Type: TypeInt, Default: "20", Min: 1, Max: 1000},
	{Key: "report_reason_max_len", Group: GroupSocial, Label: "举报理由字数", Type: TypeInt, Default: "500", Min: 10, Max: 2000},

	// 日志
	{Key: "log_access_retention_days", Group: GroupLog, Label: "访问日志保留天数", Type: TypeInt, Default: "90", Min: 1, Max: 3650},
	{Key: "log_audit_retention_days", Group: GroupLog, Label: "审计日志保留天数", Type: TypeInt, Default: "365", Min: 1, Max: 3650},

	// 运维
	{Key: "upload_dir", Group: GroupOps, Label: "本地图片目录", Type: TypeString, Default: "./data/uploads", Desc: "未配置 COS 时的图片落地目录，运维面板据此统计用量"},
	{Key: "ops_disk_warn_percent", Group: GroupOps, Label: "磁盘告警阈值（%）", Type: TypeInt, Default: "85", Min: 50, Max: 99},
	{Key: "ops_cos_alert_gb", Group: GroupOps, Label: "COS 用量告警（GB）", Type: TypeInt, Default: "50", Min: 1, Max: 100000, Desc: "按已上传图片体积估算"},
	{Key: "backup_dir", Group: GroupOps, Label: "备份目录", Type: TypeString, Default: "/var/backups/tzwall", Desc: "运维面板读取该目录展示备份文件"},
	{Key: "media_cache_dir", Group: GroupOps, Label: "源站缓存目录", Type: TypeString, Default: "./data/media_cache", Desc: "COS 对象本地副本目录，用于减少回源流量；修改需重启", Restart: true},
	{Key: "media_cache_max_gb", Group: GroupOps, Label: "源站缓存上限（GB）", Type: TypeInt, Default: "10", Min: 1, Max: 1000, Desc: "源站缓存最大占用，超出按 LRU 淘汰；修改需重启", Restart: true},

	// 页脚备案
	{Key: "footer_icp", Group: GroupFooter, Label: "ICP 备案号", Type: TypeString, Default: "", Desc: "留空则页脚不展示"},
	{Key: "footer_psba", Group: GroupFooter, Label: "公安备案号", Type: TypeString, Default: "", Desc: "留空则页脚不展示"},
	{Key: "footer_custom", Group: GroupFooter, Label: "页脚自定义内容", Type: TypeText, Default: "", Desc: "支持简单 HTML（仅超级管理员可配置），展示在备案号下方"},
}

// ConfigGroups 分组顺序与标签。
var ConfigGroups = []struct {
	Key   string
	Label string
}{
	{GroupSite, "站点"},
	{GroupAccount, "注册与登录"},
	{GroupPost, "发帖与评论"},
	{GroupMedia, "图片"},
	{GroupAI, "审核与 AI"},
	{GroupSocial, "举报"},
	{GroupLog, "日志"},
	{GroupOps, "运维"},
	{GroupFooter, "页脚备案"},
}

// ConfigItemMap 按键索引。
var ConfigItemMap = buildItemMap()

func buildItemMap() map[string]ConfigItem {
	m := make(map[string]ConfigItem, len(ConfigItems))
	for _, it := range ConfigItems {
		m[it.Key] = it
	}
	return m
}

// LookupConfig 按键取目录项。
func LookupConfig(key string) (ConfigItem, bool) {
	it, ok := ConfigItemMap[key]
	return it, ok
}

// ConfigGroupOrder 分组键顺序。
func ConfigGroupOrder() []string {
	out := make([]string, 0, len(ConfigGroups))
	for _, g := range ConfigGroups {
		out = append(out, g.Key)
	}
	return out
}
