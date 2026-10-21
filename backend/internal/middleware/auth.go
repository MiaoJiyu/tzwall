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
)

// AuthMiddleware 鉴权与登录态校验。
type AuthMiddleware struct {
	cfg *config.Config
	db  *gorm.DB
}

func NewAuth(cfg *config.Config, db *gorm.DB) *AuthMiddleware {
	return &AuthMiddleware{cfg: cfg, db: db}
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
	}
	c.Next()
}
