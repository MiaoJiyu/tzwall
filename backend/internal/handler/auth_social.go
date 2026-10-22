package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
)

// userSocialView 社交登录场景的用户视图（含社交昵称/头像）。
func userSocialView(u *model.User) gin.H {
	return gin.H{
		"id":             u.ID,
		"username":       u.Username,
		"email":          u.Email,
		"role":           u.Role,
		"wechat":         u.Wechat,
		"qq":             u.QQ,
		"phone":          u.Phone,
		"nickname":       u.Nickname,
		"avatar":         u.Avatar,
		"email_verified": u.EmailVerified,
		"status":         u.Status,
		"created_at":     u.CreatedAt,
	}
}

// WechatLogin 微信小程序登录：小程序端 wx.login() 拿 code 调用本接口。
func (h *AuthHandler) WechatLogin(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	tok, u, isNew, err := h.svc.WechatLogin(req.Code)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"token": tok, "user": userSocialView(u), "is_new": isNew})
}

// QQAuthorize 返回 QQ 授权地址（含防 CSRF 的 state）。
func (h *AuthHandler) QQAuthorize(c *gin.Context) {
	authorizeURL, state, err := h.svc.QQAuthorize()
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"url": authorizeURL, "state": state})
}

// QQCallback QQ 授权回调（由 QQ 重定向到本站）。校验 state 后回跳前端并携带一次性兑换码。
func (h *AuthHandler) QQCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if strings.TrimSpace(code) == "" {
		h.redirectSocial(c, "", "缺少授权 code")
		return
	}
	if !h.svc.QQStateValid(state) {
		h.redirectSocial(c, "", "state 校验失败，请重试")
		return
	}
	u, isNew, err := h.svc.QQCallback(code)
	if err != nil {
		h.redirectSocial(c, "", "QQ 登录失败："+err.Error())
		return
	}
	exchange, err := h.svc.SaveSocialCode(u.ID, isNew)
	if err != nil {
		h.redirectSocial(c, "", "登录失败，请重试")
		return
	}
	h.redirectSocial(c, exchange, "")
}

// redirectSocial 回跳前端登录页，携带一次性 code 或错误信息。
func (h *AuthHandler) redirectSocial(c *gin.Context, exchange, errMsg string) {
	base := h.svc.SiteBaseURL()
	target := base + "/login"
	if exchange != "" {
		target += "?social_code=" + url.QueryEscape(exchange)
	} else if errMsg != "" {
		target += "?social_error=" + url.QueryEscape(errMsg)
	}
	c.Redirect(http.StatusFound, target)
}

// SocialExchange 用一次性 code 兑换登录态（JWT）。
func (h *AuthHandler) SocialExchange(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	u, isNew, err := h.svc.RedeemSocialCode(req.Code)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	tok, err := h.svc.IssueToken(u)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, gin.H{"token": tok, "user": userSocialView(u), "is_new": isNew})
}

// BindProvider 将第三方账号绑定到当前已登录用户。
func (h *AuthHandler) BindProvider(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	var req struct {
		Provider string `json:"provider"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		(req.Provider != "wechat" && req.Provider != "qq") ||
		strings.TrimSpace(req.Code) == "" {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.BindProvider(u.ID, req.Provider, req.Code); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// UnbindProvider 解绑第三方账号。
func (h *AuthHandler) UnbindProvider(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	var req struct {
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Provider != "wechat" && req.Provider != "qq") {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.UnbindProvider(u.ID, req.Provider); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}
