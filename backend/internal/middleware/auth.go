package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/pkg/token"
	"tzwall/backend/internal/service"
)

// AuthMiddleware 鉴权与登录态校验。
type AuthMiddleware struct {
	cfg   *config.Config
	db    *gorm.DB
	store *service.ConfigStore
}

func NewAuth(cfg *config.Config, db *gorm.DB, store *service.ConfigStore) *AuthMiddleware {
	return &AuthMiddleware{cfg: cfg, db: db, store: store}
}

// CurrentUser 取上下文中的当前用户（可能为 nil）。
func CurrentUser(c *gin.Context) *model.User {
	if v, ok := c.Get("user"); ok {
		if u, ok := v.(*model.User); ok {
			return u
		}
	}
	return nil
}

func bearer(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}

// Required 必须登录，否则 401。校验 TokenVersion 与封禁状态。
func (a *AuthMiddleware) Required(c *gin.Context) {
	tok := bearer(c)
	if tok == "" {
		response.FailCode(c, errs.CodeUnauthorized, "")
		c.Abort()
		return
	}
	claims, err := token.Parse(a.cfg.JWTSecret, tok)
	if err != nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		c.Abort()
		return
	}
	var u model.User
	if err := a.db.First(&u, claims.UserID).Error; err != nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		c.Abort()
		return
	}
	if u.TokenVersion != claims.TokenVersion {
		response.Fail(c, http.StatusUnauthorized, errs.ErrTokenInvalid, "登录凭证已失效，请重新登录")
		c.Abort()
		return
	}
	if u.IsBanned() {
		response.Fail(c, http.StatusForbidden, errs.ErrBanned, "您已被封禁，请遵守社区规范")
		c.Abort()
		return
	}
	// 强制改密：标记并继续（由具体接口拦截敏感操作）
	if u.ForceResetPwd {
		c.Set("force_reset", true)
	}
	c.Set("user", &u)
	c.Next()
}

// Optional 尝试解析登录态，失败不阻断。
func (a *AuthMiddleware) Optional(c *gin.Context) {
	tok := bearer(c)
	if tok == "" {
		c.Next()
		return
	}
	claims, err := token.Parse(a.cfg.JWTSecret, tok)
	if err != nil {
		c.Next()
		return
	}
	var u model.User
	// 封禁状态由处罚中间件统一拦截，此处仅解析身份，保证访问日志与门禁可识别用户
	if err := a.db.First(&u, claims.UserID).Error; err == nil && u.TokenVersion == claims.TokenVersion {
		c.Set("user", &u)
		// 强制完善资料（仅网页端）：通过微信/QQ 注册或登录的账号，若未填写真实姓名/就读学校，
		// 除白名单端点外一律拦截。小程序不发送 X-Client: web 头，因此完全不受影响。
		if c.GetHeader("X-Client") == "web" && a.store != nil &&
			a.store.Get("social_profile_required") != "false" &&
			socialProfileIncomplete(&u) && !profileWhitelist(c.FullPath(), c.Request.Method) {
			response.Fail(c, http.StatusForbidden, errs.ErrProfileIncomplete, "")
			c.Abort()
			return
		}
	}
	c.Next()
}

// socialProfileIncomplete 判断社交账号是否未完善真实姓名/就读学校（管理员不强制）。
func socialProfileIncomplete(u *model.User) bool {
	if u.IsAdmin() {
		return false // 管理员不强制，避免影响后台管理
	}
	if u.WxOpenid == "" && u.QqOpenid == "" {
		return false // 邮箱注册用户不强制
	}
	return u.RealName == "" || u.School == ""
}

// profileWhitelist 资料完善前仍允许访问的端点（方法 + 路由模板精确匹配，或前缀匹配）。
func profileWhitelist(path, method string) bool {
	exact := map[string]bool{
		"POST /api/auth/me":                true,
		"POST /api/auth/profile":           true,
		"POST /api/auth/logout":            true,
		"POST /api/auth/agreement-confirm": true,
		"GET /api/meta/site":               true,
		"GET /api/meta/routes":             true,
		"GET /api/meta/agreements":         true,
		"GET /api/conversations/unread":    true, // 顶栏未读角标本轮询，放行避免打扰
	}
	if exact[method+" "+path] {
		return true
	}
	return strings.HasPrefix(path, "/api/meta/") || strings.HasPrefix(path, "/api/punishments/")
}
