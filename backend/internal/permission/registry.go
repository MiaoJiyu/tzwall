package permission

import (
	"sort"

	"tzwall/backend/internal/model"
)

// 分组常量，用于后台权限矩阵展示分组。
const (
	GroupSystem     = "系统"
	GroupAuth       = "账号"
	GroupContent    = "内容"
	GroupSocial     = "互动"
	GroupMedia      = "存储"
	GroupGovernance = "治理"
)

// 角色键：public 表示未登录访问者的公开权限集。
const RolePublic = "public"

// Endpoint 端点目录项。目录是权限矩阵的事实源：
// seed 依据它初始化角色默认权限集，后台矩阵与前端页面清单均由它生成。
type Endpoint struct {
	Key          string
	Method       string
	Path         string
	Group        string
	Label        string
	Public       bool
	DefaultRoles []model.Role
}

// Page 前端页面清单项，由后端维护并下发前端守卫使用。
type Page struct {
	Key      string
	Path     string
	Label    string
	Public   bool
	Requires []string
}

func ep(method, path, group, label string, public bool, roles ...model.Role) Endpoint {
	return Endpoint{
		Key:          method + " " + path,
		Method:       method,
		Path:         path,
		Group:        group,
		Label:        label,
		Public:       public,
		DefaultRoles: roles,
	}
}

// Endpoints 全量端点目录。新增接口必须在此登记。
var Endpoints = []Endpoint{
	// 系统与元信息
	ep("GET", "/api/health", GroupSystem, "健康检查", true),
	ep("GET", "/api/meta/site", GroupSystem, "站点配置", true),
	ep("GET", "/api/meta/agreements", GroupSystem, "协议清单", true),
	ep("GET", "/api/meta/agreement", GroupSystem, "协议正文", true),
	ep("GET", "/api/meta/routes", GroupSystem, "页面权限清单", true),

	// 账号
	ep("POST", "/api/auth/register", GroupAuth, "注册", true),
	ep("POST", "/api/auth/login", GroupAuth, "登录", true),
	ep("GET", "/api/auth/verify-email", GroupAuth, "邮箱激活", true),
	ep("POST", "/api/auth/resend-verify", GroupAuth, "重发激活邮件", true),
	ep("POST", "/api/auth/forgot", GroupAuth, "找回密码", true),
	ep("POST", "/api/auth/reset", GroupAuth, "重置密码", true),
	ep("POST", "/api/auth/me", GroupAuth, "当前用户", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/auth/change-password", GroupAuth, "修改密码", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/auth/change-email", GroupAuth, "修改邮箱", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/auth/logout", GroupAuth, "退出登录", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/auth/delete-account", GroupAuth, "注销账号", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/auth/agreement-confirm", GroupAuth, "确认协议", false, model.RoleUser, model.RoleAdmin),

	// 内容
	ep("GET", "/api/submissions", GroupContent, "投稿列表", true),
	ep("GET", "/api/submissions/:id", GroupContent, "投稿详情", true),
	ep("GET", "/api/tags", GroupContent, "标签列表", true),
	ep("POST", "/api/submissions", GroupContent, "发布投稿", false, model.RoleUser, model.RoleAdmin),
	ep("PUT", "/api/submissions/:id", GroupContent, "编辑投稿", false, model.RoleUser, model.RoleAdmin),
	ep("DELETE", "/api/submissions/:id", GroupContent, "删除投稿", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/my/submissions", GroupContent, "我的投稿", false, model.RoleUser, model.RoleAdmin),

	// 互动
	ep("GET", "/api/submissions/:id/comments", GroupSocial, "评论列表", true),
	ep("POST", "/api/submissions/:id/comments", GroupSocial, "发表评论", false, model.RoleUser, model.RoleAdmin),
	ep("DELETE", "/api/comments/:id", GroupSocial, "删除评论", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/reactions", GroupSocial, "点赞点踩", false, model.RoleUser, model.RoleAdmin),

	// 存储
	ep("POST", "/api/images", GroupMedia, "上传图片", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/images/:id", GroupMedia, "图片代理", true),
	ep("POST", "/api/videos", GroupMedia, "上传视频", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/videos/:id", GroupMedia, "视频代理", true),

	// 治理：处罚
	ep("GET", "/api/punishments/status", GroupGovernance, "处罚状态", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/punishments/mine", GroupGovernance, "我的处罚记录", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/punishments/:id/ack", GroupGovernance, "确认警告", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/admin/punishments", GroupGovernance, "创建处罚", false, model.RoleAdmin),
	ep("GET", "/api/admin/punishments", GroupGovernance, "处罚列表", false, model.RoleAdmin),
	ep("POST", "/api/admin/punishments/:id/revoke", GroupGovernance, "撤销处罚", false, model.RoleAdmin),
	ep("POST", "/api/admin/ip-bans", GroupGovernance, "封禁 IP", false, model.RoleAdmin),
	ep("GET", "/api/admin/ip-bans", GroupGovernance, "IP 封禁列表", false, model.RoleAdmin),
	ep("POST", "/api/admin/ip-bans/:id/revoke", GroupGovernance, "撤销 IP 封禁", false, model.RoleAdmin),

	// 治理：举报
	ep("POST", "/api/reports", GroupGovernance, "提交举报", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/reports/mine", GroupGovernance, "我的举报", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/admin/reports", GroupGovernance, "举报列表", false, model.RoleAdmin),
	ep("POST", "/api/admin/reports/:id/resolve", GroupGovernance, "处理举报", false, model.RoleAdmin),
	ep("GET", "/api/admin/reports/:id/target", GroupGovernance, "查看举报内容", false, model.RoleAdmin),

	// 治理：权限矩阵（仅超级管理员）
	ep("GET", "/api/admin/permissions/endpoints", GroupGovernance, "端点目录", false, model.RoleSuperAdmin),
	ep("GET", "/api/admin/permissions/matrix", GroupGovernance, "权限矩阵", false, model.RoleSuperAdmin),
	ep("PUT", "/api/admin/permissions/matrix", GroupGovernance, "保存权限矩阵", false, model.RoleSuperAdmin),

	// 治理：系统配置（仅超级管理员）
	ep("GET", "/api/admin/configs", GroupGovernance, "系统配置", false, model.RoleSuperAdmin),
	ep("PUT", "/api/admin/configs", GroupGovernance, "保存系统配置", false, model.RoleSuperAdmin),

	// 治理：协议管理（仅超级管理员）
	ep("GET", "/api/admin/agreements", GroupGovernance, "协议与版本", false, model.RoleSuperAdmin),
	ep("POST", "/api/admin/agreements/:key/publish", GroupGovernance, "发布协议版本", false, model.RoleSuperAdmin),
	ep("POST", "/api/admin/agreement-versions/:id/rollback", GroupGovernance, "回滚协议版本", false, model.RoleSuperAdmin),

	// 治理：词表管理（仅超级管理员）
	ep("GET", "/api/admin/words", GroupGovernance, "词表列表", false, model.RoleSuperAdmin),
	ep("POST", "/api/admin/words", GroupGovernance, "新增词条", false, model.RoleSuperAdmin),
	ep("DELETE", "/api/admin/words/:id", GroupGovernance, "删除词条", false, model.RoleSuperAdmin),

	// 治理：日志查看（仅超级管理员）
	ep("GET", "/api/admin/logs/access", GroupGovernance, "访问日志", false, model.RoleSuperAdmin),
	ep("GET", "/api/admin/logs/audit", GroupGovernance, "审计日志", false, model.RoleSuperAdmin),

	// 治理：运维面板（仅超级管理员）
	ep("GET", "/api/admin/ops/overview", GroupGovernance, "运维概览", false, model.RoleSuperAdmin),

	// 后台管理
	ep("GET", "/api/admin/dashboard", GroupSystem, "数据概览", false, model.RoleAdmin),
	ep("GET", "/api/admin/users", GroupSystem, "用户列表", false, model.RoleAdmin),
	ep("POST", "/api/admin/users/:id/role", GroupSystem, "变更角色", false, model.RoleSuperAdmin),
	ep("POST", "/api/admin/users/:id/resend-verify", GroupSystem, "重发验证邮件", false, model.RoleAdmin),
	ep("POST", "/api/admin/users/:id/activate", GroupSystem, "直接激活用户", false, model.RoleSuperAdmin),
	ep("GET", "/api/admin/tags/pending", GroupContent, "待审标签", false, model.RoleAdmin),
	ep("POST", "/api/admin/tags/:id/approve", GroupContent, "审核标签", false, model.RoleAdmin),
	ep("POST", "/api/admin/tags/merge", GroupContent, "合并标签", false, model.RoleAdmin),
	ep("GET", "/api/admin/tags", GroupContent, "全部标签", false, model.RoleAdmin),
	ep("POST", "/api/admin/tags", GroupContent, "新增标签", false, model.RoleAdmin),
	ep("PUT", "/api/admin/tags/:id", GroupContent, "修改标签", false, model.RoleAdmin),
	ep("DELETE", "/api/admin/tags/:id", GroupContent, "删除标签", false, model.RoleAdmin),
	ep("GET", "/api/admin/submissions/pending", GroupContent, "待审投稿", false, model.RoleAdmin),
	ep("GET", "/api/admin/submissions/all", GroupContent, "全部稿件", false, model.RoleAdmin),
	ep("POST", "/api/admin/comments/:id/remove", GroupContent, "撤下评论", false, model.RoleAdmin),
	ep("POST", "/api/admin/comments/:id/restore", GroupContent, "恢复评论", false, model.RoleAdmin),
	ep("POST", "/api/admin/images/sync-private", GroupMedia, "同步图片私有ACL", false, model.RoleAdmin),
	ep("POST", "/api/admin/submissions/:id/review", GroupContent, "审核投稿", false, model.RoleAdmin),
	ep("POST", "/api/admin/submissions/:id/restore", GroupContent, "恢复投稿", false, model.RoleAdmin),

	// 点歌
	ep("GET", "/api/songs", GroupContent, "点歌列表", true),
	ep("GET", "/api/songs/quota", GroupContent, "点歌配额", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/songs", GroupContent, "提交点歌", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/admin/songs", GroupContent, "点歌管理列表", false, model.RoleAdmin),
	ep("POST", "/api/admin/songs/:id/status", GroupContent, "点歌状态流转", false, model.RoleAdmin),
	ep("DELETE", "/api/admin/songs/:id", GroupContent, "删除点歌", false, model.RoleAdmin),

	// 投票
	ep("GET", "/api/polls", GroupContent, "投票列表", true),
	ep("GET", "/api/polls/:id", GroupContent, "投票详情", true),
	ep("POST", "/api/polls", GroupContent, "发起投票", false, model.RoleUser, model.RoleAdmin),
	ep("POST", "/api/polls/:id/vote", GroupContent, "参与投票", false, model.RoleUser, model.RoleAdmin),
	ep("DELETE", "/api/polls/:id", GroupContent, "删除自己的投票", false, model.RoleUser, model.RoleAdmin),
	ep("GET", "/api/admin/polls", GroupContent, "投票管理列表", false, model.RoleAdmin),
	ep("POST", "/api/admin/polls", GroupContent, "发起置顶投票", false, model.RoleAdmin),
	ep("DELETE", "/api/admin/polls/:id", GroupContent, "强删投票", false, model.RoleAdmin),

	// 审核回调（腾讯云数据万象）
	ep("POST", "/api/moderation/callback", GroupMedia, "审核回调", true),
}

// Pages 前端页面清单。Requires 为空且 Public 为 true 表示完全公开。
var Pages = []Page{
	{Key: "page:home", Path: "/", Label: "首页", Public: true},
	{Key: "page:submission", Path: "/submission/:id", Label: "投稿详情", Public: true},
	{Key: "page:login", Path: "/login", Label: "登录", Public: true},
	{Key: "page:register", Path: "/register", Label: "注册", Public: true},
	{Key: "page:verify", Path: "/verify", Label: "邮箱激活", Public: true},
	{Key: "page:forgot", Path: "/forgot", Label: "找回密码", Public: true},
	{Key: "page:reset", Path: "/reset", Label: "重置密码", Public: true},
	{Key: "page:agreement", Path: "/agreement", Label: "协议确认", Requires: []string{"POST /api/auth/agreement-confirm"}},
	{Key: "page:publish", Path: "/publish", Label: "发布投稿", Requires: []string{"POST /api/submissions"}},
	{Key: "page:my", Path: "/my", Label: "我的投稿", Requires: []string{"GET /api/my/submissions"}},
	{Key: "page:profile", Path: "/profile", Label: "账号安全", Requires: []string{"POST /api/auth/me"}},
	{Key: "page:my.punishments", Path: "/my/punishments", Label: "我的权限变更记录", Requires: []string{"GET /api/punishments/mine"}},
	{Key: "page:songs", Path: "/songs", Label: "点歌", Public: true},
	{Key: "page:polls", Path: "/polls", Label: "投票", Public: true},
	{Key: "page:admin.dashboard", Path: "/admin/dashboard", Label: "数据概览", Requires: []string{"GET /api/admin/dashboard"}},
	{Key: "page:admin.users", Path: "/admin/users", Label: "用户管理", Requires: []string{"GET /api/admin/users"}},
	{Key: "page:admin.submissions", Path: "/admin/submissions", Label: "投稿审核", Requires: []string{"GET /api/admin/submissions/pending"}},
	{Key: "page:admin.allsubmissions", Path: "/admin/all-submissions", Label: "所有稿件", Requires: []string{"GET /api/admin/submissions/all"}},
	{Key: "page:admin.tags", Path: "/admin/tags", Label: "标签管理", Requires: []string{"GET /api/admin/tags/pending"}},
	{Key: "page:admin.songs", Path: "/admin/songs", Label: "点歌管理", Requires: []string{"GET /api/admin/songs"}},
	{Key: "page:admin.polls", Path: "/admin/polls", Label: "投票管理", Requires: []string{"GET /api/admin/polls"}},
	{Key: "page:admin.reports", Path: "/admin/reports", Label: "举报审核", Requires: []string{"GET /api/admin/reports"}},
	{Key: "page:admin.punishments", Path: "/admin/punishments", Label: "处罚管理", Requires: []string{"GET /api/admin/punishments"}},
	{Key: "page:admin.permissions", Path: "/admin/permissions", Label: "访问权限", Requires: []string{"GET /api/admin/permissions/matrix"}},
	{Key: "page:admin.configs", Path: "/admin/configs", Label: "系统配置", Requires: []string{"GET /api/admin/configs"}},
	{Key: "page:admin.logs.access", Path: "/admin/logs/access", Label: "访问日志", Requires: []string{"GET /api/admin/logs/access"}},
	{Key: "page:admin.logs.audit", Path: "/admin/logs/audit", Label: "审计日志", Requires: []string{"GET /api/admin/logs/audit"}},
	{Key: "page:admin.ops", Path: "/admin/ops", Label: "运维面板", Requires: []string{"GET /api/admin/ops/overview"}},
}

// Get 按端点键取目录项。
func Get(key string) (Endpoint, bool) {
	for _, e := range Endpoints {
		if e.Key == key {
			return e, true
		}
	}
	return Endpoint{}, false
}

// Groups 返回分组后的端点（顺序稳定）。
func Groups() map[string][]Endpoint {
	out := map[string][]Endpoint{}
	for _, e := range Endpoints {
		out[e.Group] = append(out[e.Group], e)
	}
	for k := range out {
		sort.Slice(out[k], func(i, j int) bool { return out[k][i].Key < out[k][j].Key })
	}
	return out
}

// Keys 全量端点键。
func Keys() []string {
	keys := make([]string, 0, len(Endpoints))
	for _, e := range Endpoints {
		keys = append(keys, e.Key)
	}
	sort.Strings(keys)
	return keys
}
