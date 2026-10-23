package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tzwall/backend/internal/ai"
	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/config"
	"tzwall/backend/internal/mailer"
	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/moderation"
	"tzwall/backend/internal/permission"
	"tzwall/backend/internal/service"
	"tzwall/backend/internal/storage"
)

// Deps 路由装配依赖。
type Deps struct {
	Cfg        *config.Config
	DB         *gorm.DB
	Cache      cache.Cache
	Storage    storage.Storage
	Moderation moderation.Moderation
	Mailer     mailer.Mailer
	Store      *service.ConfigStore
	AI         *ai.Client
}

// SetupRouter 装配中间件与全部路由。
func SetupRouter(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RealIP(d.Cfg))


	resolver := permission.NewResolver(d.DB)
	perm := middleware.NewPermission(resolver, d.Cache)

	authMW := middleware.NewAuth(d.Cfg, d.DB)
	// 处罚服务同时服务于中间件与后台：统一实例，注入词表用于新用户名校验
	punSvc := service.NewPunishment(d.DB, d.Cfg, d.Cache, d.Store, d.Mailer)
	words := service.NewWordFilter(d.DB)
	punSvc.WithWordFilter(words)

	// 中间件顺序：真实 IP → IP 封禁 → 身份解析 → 处罚门禁 → 访问日志
	r.Use(middleware.NewIPBan(punSvc).Gate())
	r.Use(authMW.Optional)
	r.Use(middleware.NewPunishment(punSvc).Gate())
	r.Use(middleware.NewAccessLogger(d.DB, d.Store).Log())

	authSvc := service.NewAuth(d.DB, d.Cfg, d.Cache, d.Mailer, d.Store).WithWordFilter(words)
	subSvc := service.NewSubmission(d.DB, d.Cfg, d.Storage, d.Moderation, d.Store, d.Mailer, d.Cache).
		WithGovernance(words, d.AI)
	commentSvc := service.NewComment(d.DB, d.Store, d.Moderation).WithWordFilter(words)
	songSvc := service.NewSong(d.DB, d.Store).WithWordFilter(words)
	pollSvc := service.NewPoll(d.DB, d.Store).WithWordFilter(words)
	ticketSvc := service.NewTicket(d.DB, d.Store).WithWordFilter(words)
	userSvc := service.NewUser(d.DB)
	chatSvc := service.NewChat(d.DB)
	interSvc := service.NewInteraction(d.DB)
	adminSvc := service.NewAdmin(d.DB, d.Store)
	permSvc := service.NewPermission(d.DB, resolver)

	authH := NewAuthHandler(authSvc)
	subH := NewSubmissionHandler(subSvc)
	commentH := NewCommentHandler(commentSvc)
	songH := NewSongHandler(songSvc)
	pollH := NewPollHandler(pollSvc)
	ticketH := NewTicketHandler(ticketSvc)
	userH := NewUserHandler(userSvc)
	chatH := NewChatHandler(chatSvc)
	annH := NewAnnouncementHandler(service.NewAnnouncement(d.DB))
	interH := NewInteractionHandler(interSvc)
	imgH := NewImageHandler(d.DB, d.Storage, d.Cfg, d.Cache, d.Store, d.Moderation)
	initImageLinks(d.Cfg.JWTSecret, d.Store) // 图片访客令牌签名密钥（盲水印写入用户名用）
	vidH := NewVideoHandler(d.DB, d.Storage, d.Store)
	metaH := NewMetaHandler(d.DB, d.Cfg, d.Store, resolver)
	adminH := NewAdminHandler(adminSvc, subSvc)
	permH := NewPermissionHandler(permSvc, resolver)
	punH := NewPunishmentHandler(punSvc)
	reportSvc := service.NewReport(d.DB, d.Cache, d.Store, punSvc)
	reportH := NewReportHandler(reportSvc, subSvc, commentSvc)
	modH := NewModerationHandler(d.DB, d.Cfg, d.Storage)
	cfgH := NewConfigHandler(d.Store, d.Cfg)
	agrH := NewAgreementHandler(service.NewAgreement(d.DB))
	wordH := NewWordHandler(words)
	logH := NewLogHandler(d.DB, d.Store)
	wmH := NewWatermarkHandler(d.Storage)
	opsH := NewOpsHandler(service.NewOps(d.DB, d.Cfg, d.Store, d.Cache, d.Moderation, d.Storage, func() bool {
		return d.AI != nil && d.AI.Configured()
	}))

	r.GET("/api/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	// 元信息（公开）
	r.GET("/api/meta/site", perm.Public("GET /api/meta/site"), metaH.Site)
	r.GET("/api/meta/agreements", perm.Public("GET /api/meta/agreements"), metaH.Agreements)
	r.GET("/api/meta/agreement", perm.Public("GET /api/meta/agreement"), metaH.Agreement)
	r.GET("/api/meta/routes", perm.Public("GET /api/meta/routes"), metaH.Routes)

	// 内容审核回调（腾讯云数据万象）
	r.POST("/api/moderation/callback", perm.Public("POST /api/moderation/callback"), modH.Callback)

	// 鉴权（公开）
	r.POST("/api/auth/register", perm.Public("POST /api/auth/register"), authH.Register)
	r.POST("/api/auth/login", perm.Public("POST /api/auth/login"), authH.Login)
	r.GET("/api/auth/verify-email", perm.Public("GET /api/auth/verify-email"), authH.VerifyEmail)
	r.POST("/api/auth/resend-verify", perm.Public("POST /api/auth/resend-verify"), authH.ResendVerify)
	r.POST("/api/auth/forgot", perm.Public("POST /api/auth/forgot"), authH.Forgot)
	r.POST("/api/auth/reset", perm.Public("POST /api/auth/reset"), authH.Reset)

	// 第三方登录（微信小程序 / QQ OAuth）
	r.POST("/api/auth/wechat/login", perm.Public("POST /api/auth/wechat/login"), authH.WechatLogin)
	r.POST("/api/auth/wechat/register", perm.Public("POST /api/auth/wechat/register"), authH.WechatRegister)
	r.POST("/api/auth/wechat/bind-code", perm.Require("POST /api/auth/wechat/bind-code"), authH.WechatBindCode)
	r.POST("/api/auth/wechat/bind-code/confirm", perm.Public("POST /api/auth/wechat/bind-code/confirm"), authH.WechatBindByCode)
	r.GET("/api/auth/qq/url", perm.Public("GET /api/auth/qq/url"), authH.QQAuthorize)
	r.GET("/api/auth/qq/callback", perm.Public("GET /api/auth/qq/callback"), authH.QQCallback)
	r.POST("/api/auth/social/exchange", perm.Public("POST /api/auth/social/exchange"), authH.SocialExchange)
	r.POST("/api/auth/bind", perm.Require("POST /api/auth/bind"), authH.BindProvider)
	r.POST("/api/auth/unbind", perm.Require("POST /api/auth/unbind"), authH.UnbindProvider)
	r.GET("/api/auth/qq/bind/url", perm.Require("GET /api/auth/qq/bind/url"), authH.QQBindAuthorize)
	r.GET("/api/auth/wechat/web/url", perm.Public("GET /api/auth/wechat/web/url"), authH.WechatWebAuthorize)
	r.GET("/api/auth/wechat/web/bind/url", perm.Require("GET /api/auth/wechat/web/bind/url"), authH.WechatWebBindAuthorize)
	r.GET("/api/auth/wechat/web/callback", perm.Public("GET /api/auth/wechat/web/callback"), authH.WechatWebCallback)

	// 鉴权（私有）
	r.POST("/api/auth/me", perm.Require("POST /api/auth/me"), authH.Me)
	r.POST("/api/auth/change-password", perm.Require("POST /api/auth/change-password"), authH.ChangePassword)
	r.POST("/api/auth/profile", perm.Require("POST /api/auth/profile"), authH.Profile)
	r.POST("/api/auth/change-email", perm.Require("POST /api/auth/change-email"), authH.ChangeEmail)
	r.POST("/api/auth/logout", perm.Require("POST /api/auth/logout"), authH.Logout)
	r.POST("/api/auth/delete-account", perm.Require("POST /api/auth/delete-account"), authH.DeleteAccount)
	r.POST("/api/auth/agreement-confirm", perm.Require("POST /api/auth/agreement-confirm"), metaH.ConfirmAgreement)

	// 投稿/标签/评论（公开）
	r.GET("/api/submissions", perm.Public("GET /api/submissions"), subH.List)
	r.GET("/api/submissions/:id", perm.Public("GET /api/submissions/:id"), subH.Get)
	r.GET("/api/tags", perm.Public("GET /api/tags"), subH.Tags)
	r.GET("/api/submissions/:id/comments", perm.Public("GET /api/submissions/:id/comments"), commentH.List)
	// 点歌/投票（公开浏览）
	r.GET("/api/songs", perm.Public("GET /api/songs"), songH.List)
	r.GET("/api/polls", perm.Public("GET /api/polls"), pollH.List)
	r.GET("/api/polls/:id", perm.Public("GET /api/polls/:id"), pollH.Detail)
	r.GET("/api/polls/mine", perm.Require("GET /api/polls/mine"), pollH.MyPolls)
	r.GET("/api/submissions/:id/polls", perm.Public("GET /api/submissions/:id/polls"), pollH.SubmissionPolls)
	r.POST("/api/submissions/:id/polls", perm.Require("POST /api/submissions/:id/polls"), pollH.SetSubmissionPolls)
	// 用户公开主页（公开）
	r.GET("/api/users/:id/public", perm.Public("GET /api/users/:id/public"), userH.Public)

	// 投稿/评论（私有）
	r.POST("/api/submissions", perm.Require("POST /api/submissions"), subH.Create)
	r.PUT("/api/submissions/:id", perm.Require("PUT /api/submissions/:id"), subH.Update)
	r.POST("/api/submissions/:id/withdraw", perm.Require("POST /api/submissions/:id/withdraw"), subH.Withdraw)
	r.DELETE("/api/submissions/:id", perm.Require("DELETE /api/submissions/:id"), subH.Delete)
	r.GET("/api/my/submissions", perm.Require("GET /api/my/submissions"), subH.ListMine)

	// 工单（用户端，需登录）
	r.POST("/api/tickets", perm.Require("POST /api/tickets"), ticketH.Create)
	r.GET("/api/tickets/mine", perm.Require("GET /api/tickets/mine"), ticketH.ListMine)
	r.GET("/api/tickets/:id", perm.Require("GET /api/tickets/:id"), ticketH.Get)
	r.POST("/api/tickets/:id/close", perm.Require("POST /api/tickets/:id/close"), ticketH.Close)
	r.POST("/api/submissions/:id/comments", perm.Require("POST /api/submissions/:id/comments"), commentH.Create)
	r.DELETE("/api/comments/:id", perm.Require("DELETE /api/comments/:id"), commentH.Delete)
	r.POST("/api/admin/comments/:id/remove", perm.Require("POST /api/admin/comments/:id/remove"), commentH.AdminRemove)
	r.POST("/api/admin/comments/:id/restore", perm.Require("POST /api/admin/comments/:id/restore"), commentH.AdminRestore)

	// 互动（私有）
	r.POST("/api/reactions", perm.Require("POST /api/reactions"), interH.React)

	// 点歌/投票（私有）
	r.GET("/api/songs/quota", perm.Require("GET /api/songs/quota"), songH.Quota)
	r.POST("/api/songs", perm.Require("POST /api/songs"), songH.Create)
	r.POST("/api/polls", perm.Require("POST /api/polls"), pollH.Create)
	r.POST("/api/polls/:id/vote", perm.Require("POST /api/polls/:id/vote"), pollH.Vote)
	r.DELETE("/api/polls/:id", perm.Require("DELETE /api/polls/:id"), pollH.DeleteOwn)

	// 私聊（需登录）
	r.POST("/api/conversations", perm.Require("POST /api/conversations"), chatH.Open)
	r.GET("/api/conversations", perm.Require("GET /api/conversations"), chatH.List)
	r.GET("/api/conversations/unread", perm.Require("GET /api/conversations/unread"), chatH.Unread)
	r.GET("/api/conversations/:id/messages", perm.Require("GET /api/conversations/:id/messages"), chatH.Messages)
	r.POST("/api/conversations/:id/messages", perm.Require("POST /api/conversations/:id/messages"), chatH.Send)
	r.POST("/api/conversations/:id/read", perm.Require("POST /api/conversations/:id/read"), chatH.Read)

	// 公告（公开列表 + 确认）
	r.GET("/api/announcements", perm.Public("GET /api/announcements"), annH.List)
	r.POST("/api/announcements/:id/confirm", perm.Require("POST /api/announcements/:id/confirm"), annH.Confirm)
	// 公告管理（管理员）
	r.GET("/api/admin/announcements", perm.Require("GET /api/admin/announcements"), annH.AdminList)
	r.POST("/api/admin/announcements", perm.Require("POST /api/admin/announcements"), annH.Create)
	r.PUT("/api/admin/announcements/:id", perm.Require("PUT /api/admin/announcements/:id"), annH.Update)
	r.DELETE("/api/admin/announcements/:id", perm.Require("DELETE /api/admin/announcements/:id"), annH.Delete)

	// 图片（私有上传 / 公开代理）
	r.POST("/api/images", perm.Require("POST /api/images"), imgH.Upload)
	r.GET("/api/images/:id", perm.Public("GET /api/images/:id"), imgH.Proxy)

	// 视频（私有上传 / 公开代理，支持 Range）
	r.POST("/api/videos", perm.Require("POST /api/videos"), vidH.Upload)
	r.GET("/api/videos/:id", perm.Public("GET /api/videos/:id"), vidH.Proxy)

	// 后台（私有，角色在 handler 内校验）
	r.GET("/api/admin/dashboard", perm.Require("GET /api/admin/dashboard"), adminH.Dashboard)
	r.GET("/api/admin/users", perm.Require("GET /api/admin/users"), adminH.Users)
	r.GET("/api/admin/users/:id", perm.Require("GET /api/admin/users/:id"), adminH.UserDetail)
	r.POST("/api/admin/users/:id/role", perm.Require("POST /api/admin/users/:id/role"), adminH.SetRole)
	r.POST("/api/admin/users/:id/title", perm.Require("POST /api/admin/users/:id/title"), adminH.SetTitle)
	r.POST("/api/admin/users/:id/resend-verify", perm.Require("POST /api/admin/users/:id/resend-verify"), authH.AdminResendVerify)
	r.POST("/api/admin/users/:id/activate", perm.Require("POST /api/admin/users/:id/activate"), authH.AdminActivate)
	r.GET("/api/admin/tags/pending", perm.Require("GET /api/admin/tags/pending"), adminH.PendingTags)
	r.POST("/api/admin/tags/:id/approve", perm.Require("POST /api/admin/tags/:id/approve"), adminH.ApproveTag)
	r.POST("/api/admin/tags/merge", perm.Require("POST /api/admin/tags/merge"), adminH.MergeTag)
	r.GET("/api/admin/tags", perm.Require("GET /api/admin/tags"), adminH.AllTags)
	r.POST("/api/admin/tags", perm.Require("POST /api/admin/tags"), adminH.CreateTag)
	r.PUT("/api/admin/tags/:id", perm.Require("PUT /api/admin/tags/:id"), adminH.UpdateTag)
	r.DELETE("/api/admin/tags/:id", perm.Require("DELETE /api/admin/tags/:id"), adminH.DeleteTag)
	r.GET("/api/admin/submissions/pending", perm.Require("GET /api/admin/submissions/pending"), adminH.PendingSubmissions)
	r.GET("/api/admin/submissions/all", perm.Require("GET /api/admin/submissions/all"), adminH.AllSubmissions)

	// 工单（后台管理：普通管理员与超级管理员均可处理）
	r.GET("/api/admin/tickets", perm.Require("GET /api/admin/tickets"), ticketH.AdminList)
	r.GET("/api/admin/tickets/:id", perm.Require("GET /api/admin/tickets/:id"), ticketH.AdminGet)
	r.POST("/api/admin/tickets/:id/reply", perm.Require("POST /api/admin/tickets/:id/reply"), ticketH.Reply)
	r.POST("/api/admin/tickets/:id/status", perm.Require("POST /api/admin/tickets/:id/status"), ticketH.SetStatus)
	r.POST("/api/admin/submissions/:id/review", perm.Require("POST /api/admin/submissions/:id/review"), adminH.ReviewSubmission)
	r.POST("/api/admin/submissions/:id/restore", perm.Require("POST /api/admin/submissions/:id/restore"), adminH.RestoreSubmission)
	r.POST("/api/admin/images/sync-private", perm.Require("POST /api/admin/images/sync-private"), imgH.SyncPrivate)

	// 处罚（私有）
	r.GET("/api/punishments/status", perm.Require("GET /api/punishments/status"), punH.Status)
	r.GET("/api/punishments/mine", perm.Require("GET /api/punishments/mine"), punH.Mine)
	r.POST("/api/punishments/:id/ack", perm.Require("POST /api/punishments/:id/ack"), punH.Ack)

	// 举报（私有）
	r.POST("/api/reports", perm.Require("POST /api/reports"), reportH.Create)
	r.GET("/api/reports/mine", perm.Require("GET /api/reports/mine"), reportH.Mine)

	// 举报审核（管理员）
	r.GET("/api/admin/reports", perm.Require("GET /api/admin/reports"), reportH.AdminList)
	r.POST("/api/admin/reports/:id/resolve", perm.Require("POST /api/admin/reports/:id/resolve"), reportH.Resolve)
	r.GET("/api/admin/reports/:id/target", perm.Require("GET /api/admin/reports/:id/target"), reportH.TargetDetail)

	// 点歌/投票管理（管理员）
	r.GET("/api/admin/songs", perm.Require("GET /api/admin/songs"), songH.AdminList)
	r.POST("/api/admin/songs/:id/status", perm.Require("POST /api/admin/songs/:id/status"), songH.SetStatus)
	r.DELETE("/api/admin/songs/:id", perm.Require("DELETE /api/admin/songs/:id"), songH.AdminDelete)
	r.GET("/api/admin/polls", perm.Require("GET /api/admin/polls"), pollH.AdminList)
	r.POST("/api/admin/polls", perm.Require("POST /api/admin/polls"), pollH.AdminCreate)
	r.DELETE("/api/admin/polls/:id", perm.Require("DELETE /api/admin/polls/:id"), pollH.AdminDelete)

	// 处罚管理（管理员）
	r.POST("/api/admin/punishments", perm.Require("POST /api/admin/punishments"), punH.AdminCreate)
	r.GET("/api/admin/punishments", perm.Require("GET /api/admin/punishments"), punH.AdminList)
	r.POST("/api/admin/punishments/:id/revoke", perm.Require("POST /api/admin/punishments/:id/revoke"), punH.AdminRevoke)
	r.POST("/api/admin/ip-bans", perm.Require("POST /api/admin/ip-bans"), punH.IPBanCreate)
	r.GET("/api/admin/ip-bans", perm.Require("GET /api/admin/ip-bans"), punH.IPBanList)
	r.POST("/api/admin/ip-bans/:id/revoke", perm.Require("POST /api/admin/ip-bans/:id/revoke"), punH.IPBanRevoke)

	// 权限矩阵（仅超级管理员）
	r.GET("/api/admin/permissions/endpoints", perm.Require("GET /api/admin/permissions/endpoints"), permH.Endpoints)
	r.GET("/api/admin/permissions/matrix", perm.Require("GET /api/admin/permissions/matrix"), permH.Matrix)
	r.PUT("/api/admin/permissions/matrix", perm.Require("PUT /api/admin/permissions/matrix"), permH.Save)

	// 系统配置中心（仅超级管理员）
	r.GET("/api/admin/configs", perm.Require("GET /api/admin/configs"), cfgH.List)
	r.PUT("/api/admin/configs", perm.Require("PUT /api/admin/configs"), cfgH.Save)

	// 协议管理（仅超级管理员）
	r.GET("/api/admin/agreements", perm.Require("GET /api/admin/agreements"), agrH.List)
	r.POST("/api/admin/agreements/:key/publish", perm.Require("POST /api/admin/agreements/:key/publish"), agrH.Publish)
	r.POST("/api/admin/agreement-versions/:id/rollback", perm.Require("POST /api/admin/agreement-versions/:id/rollback"), agrH.Rollback)

	// 词表管理（仅超级管理员）
	r.GET("/api/admin/words", perm.Require("GET /api/admin/words"), wordH.List)
	r.POST("/api/admin/words", perm.Require("POST /api/admin/words"), wordH.Add)
	r.DELETE("/api/admin/words/:id", perm.Require("DELETE /api/admin/words/:id"), wordH.Remove)

	// 日志查看（仅超级管理员）
	r.GET("/api/admin/logs/access", perm.Require("GET /api/admin/logs/access"), logH.AccessLogs)
	r.GET("/api/admin/logs/audit", perm.Require("GET /api/admin/logs/audit"), logH.AuditLogs)

	// 运维面板（仅超级管理员）
	r.GET("/api/admin/ops/overview", perm.Require("GET /api/admin/ops/overview"), opsH.Overview)

	// 盲水印识别（仅超级管理员）：上传外泄图片，还原其中嵌入的访客水印
	r.POST("/api/admin/watermark/extract", perm.Require("POST /api/admin/watermark/extract"), wmH.Extract)

	return r
}
