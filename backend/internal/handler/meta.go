package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/permission"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// MetaHandler 站点元信息与协议。
type MetaHandler struct {
	db    *gorm.DB
	cfg   *config.Config
	store *service.ConfigStore
	res   *permission.Resolver
}

func NewMetaHandler(db *gorm.DB, cfg *config.Config, store *service.ConfigStore, res *permission.Resolver) *MetaHandler {
	return &MetaHandler{db: db, cfg: cfg, store: store, res: res}
}

// Routes 下发页面清单与当前用户可访问页面（前端守卫使用）。
func (h *MetaHandler) Routes(c *gin.Context) {
	u := middleware.CurrentUser(c)
	allowed := []string{}
	if h.res != nil {
		allowed = h.res.AllowedPages(u)
	}
	response.OK(c, gin.H{"pages": permission.Pages, "allowed": allowed})
}

// Site 站点公开配置。
func (h *MetaHandler) Site(c *gin.Context) {
	siteName := h.store.Get("site_name")
	if siteName == "" {
		siteName = h.cfg.SiteName
	}
	response.OK(c, gin.H{
		"site_name":          siteName,
		"footer_icp":         h.store.Get("footer_icp"),
		"footer_psba":        h.store.Get("footer_psba"),
		"footer_custom":      h.store.Get("footer_custom"),
		"agreement_required": h.store.Bool("agreement_required"),
		"features": gin.H{
			"storage":    h.cfg.StorageConfigured,
			"mailer":     h.cfg.MailerConfigured,
			"moderation": h.cfg.ModerationConfigured,
			"social": gin.H{
				"wechat": (h.store.Get("wechat_mp_appid") != "" || h.cfg.WechatMPAppID != "") &&
					(h.store.SecretValue("wechat_mp_secret") != "" || h.cfg.WechatMPSecret != ""),
				"wechat_web": (h.store.Get("wechat_web_appid") != "" || h.cfg.WechatWebAppID != "") &&
					(h.store.SecretValue("wechat_web_secret") != "" || h.cfg.WechatWebSecret != ""),
				"qq": (h.store.Get("qq_oauth_appid") != "" || h.cfg.QQAppID != "") &&
					(h.store.SecretValue("qq_oauth_secret") != "" || h.cfg.QQSecret != "") &&
					(h.store.Get("qq_oauth_redirect_uri") != "" || h.cfg.QQRedirectURI != ""),
			},
		},
	})
}

// Agreements 协议清单（含最新版本）。
func (h *MetaHandler) Agreements(c *gin.Context) {
	var list []model.Agreement
	h.db.Order("created_at DESC").Find(&list)
	items := make([]gin.H, 0, len(list))
	for _, a := range list {
		items = append(items, gin.H{"key": a.Key, "title": a.Title, "version": a.Version, "effective_at": a.EffectiveAt})
	}
	response.OK(c, gin.H{"items": items})
}

// Agreement 单份协议正文。
func (h *MetaHandler) Agreement(c *gin.Context) {
	key := c.Query("key")
	var a model.Agreement
	if err := h.db.Where("key = ?", key).First(&a).Error; err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	response.OK(c, gin.H{"key": a.Key, "title": a.Title, "version": a.Version, "content": a.Content})
}

// ConfirmAgreement 已登录用户确认最新协议。
func (h *MetaHandler) ConfirmAgreement(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var latest model.Agreement
	h.db.Order("created_at DESC").First(&latest)
	h.db.Model(u).Update("agreement_version", latest.Version)
	response.OK(c, nil)
}
