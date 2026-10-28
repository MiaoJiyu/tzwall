package handler

import (
	"encoding/base64"
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
		"wechat_bound":   u.WxOpenid != "",
		"qq_bound":       u.QqOpenid != "",
		"real_name_filled": u.RealName != "",
		"school_filled":    u.School != "",
		"status":         u.Status,
		"created_at":     u.CreatedAt,
	}
}

// WechatLogin 微信小程序登录：小程序端 wx.login() 拿 code 调用本接口。
// 已绑定账号返回 token；未绑定返回 need_register + ticket，要求用户填写用户名+密码后再注册。
func (h *AuthHandler) WechatLogin(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	tok, u, ticket, err := h.svc.WechatLogin(req.Code)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	if ticket != "" {
		response.OK(c, gin.H{"need_register": true, "ticket": ticket})
		return
	}
	response.OK(c, gin.H{"token": tok, "user": userSocialView(u), "is_new": false})
}

// WechatRegister 小程序首次登录完成注册（用户名+密码）。
func (h *AuthHandler) WechatRegister(c *gin.Context) {
	var req struct {
		Ticket   string `json:"ticket"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	tok, u, err := h.svc.WechatRegister(req.Ticket, req.Username, req.Password)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"token": tok, "user": userSocialView(u), "is_new": true})
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

// QQCallback QQ 授权回调（由 QQ 重定向到本站）。按 state 区分「登录」或「绑定当前账号」。
func (h *AuthHandler) QQCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if strings.TrimSpace(code) == "" {
		h.redirectSocial(c, "", "缺少授权 code")
		return
	}
	mode, uid, ok := h.svc.QQStateConsume(state)
	if !ok {
		h.redirectSocial(c, "", "state 校验失败，请重试")
		return
	}
	if mode == "bind" {
		if err := h.svc.BindProvider(uid, "qq", code); err != nil {
			h.redirectProfileBind(c, "qq", err.Error())
			return
		}
		h.redirectProfileBind(c, "qq", "")
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

// QQBindAuthorize 生成“绑定 QQ 到当前账号”的授权地址（需登录）。
func (h *AuthHandler) QQBindAuthorize(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	authorizeURL, _, err := h.svc.QQAuthorizeBind(u.ID)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"url": authorizeURL})
}

// redirectProfileBind 绑定完成后回跳账号中心并携带结果。
func (h *AuthHandler) redirectProfileBind(c *gin.Context, provider, errMsg string) {
	base := h.svc.SiteBaseURL()
	target := base + "/profile?bind=" + url.QueryEscape(provider)
	if errMsg != "" {
		target += "&bind_error=" + url.QueryEscape(errMsg)
	} else {
		target += "&bind_ok=1"
	}
	c.Redirect(http.StatusFound, target)
}

// WechatWebAuthorize 生成微信网页扫码登录授权地址（无需登录）。
func (h *AuthHandler) WechatWebAuthorize(c *gin.Context) {
	u, err := h.svc.WechatWebAuthorize("login", 0)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"url": u})
}

// WechatWebBindAuthorize 生成微信网页扫码绑定授权地址（需登录）。
func (h *AuthHandler) WechatWebBindAuthorize(c *gin.Context) {
	cu := middleware.CurrentUser(c)
	if cu == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	u, err := h.svc.WechatWebAuthorize("bind", cu.ID)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"url": u})
}

// WechatBindCode 发起微信绑定（需登录）。依后台开关返回自动绑定小程序码或手动授权码。
//   - auto=true：额外返回 image_base64（携带绑定令牌的小程序码），小程序扫码后自动完成绑定；
//   - auto=false：仅返回 code（手动授权码），与历史流程一致。
func (h *AuthHandler) WechatBindCode(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	auto, code, _, image, err := h.svc.WechatBindInit(u.ID)
	if err != nil {
		c2, _ := errs.As(err)
		response.FailCode(c, c2, err.Error())
		return
	}
	resp := gin.H{"auto": auto, "code": code, "expires_in": 600}
	if auto && len(image) > 0 {
		resp["image_base64"] = base64.StdEncoding.EncodeToString(image)
	}
	response.OK(c, resp)
}

// WechatBindConfirm 小程序侧用绑定令牌完成自动绑定（公开：凭 bind_token + wx.login 的 code）。
func (h *AuthHandler) WechatBindConfirm(c *gin.Context) {
	var req struct {
		BindToken string `json:"bind_token"`
		Code      string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.BindByToken(req.BindToken, req.Code); err != nil {
		c2, _ := errs.As(err)
		response.FailCode(c, c2, err.Error())
		return
	}
	response.OK(c, nil)
}

// WechatBindByCode 小程序内用绑定码完成绑定（公开：凭绑定码 + wx.login 的 code）。
func (h *AuthHandler) WechatBindByCode(c *gin.Context) {
	var req struct {
		BindCode string `json:"bind_code"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.BindByCode(req.BindCode, req.Code); err != nil {
		c2, _ := errs.As(err)
		response.FailCode(c, c2, err.Error())
		return
	}
	response.OK(c, nil)
}

// WechatWebCallback 微信网页授权回调：按 state 区分登录或绑定。
func (h *AuthHandler) WechatWebCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if strings.TrimSpace(code) == "" {
		h.redirectSocial(c, "", "缺少授权 code")
		return
	}
	mode, uid, ok := h.svc.WechatWebStateConsume(state)
	if !ok {
		h.redirectSocial(c, "", "state 校验失败，请重试")
		return
	}
	if mode == "bind" {
		if err := h.svc.WechatWebBind(uid, code); err != nil {
			h.redirectProfileBind(c, "wechat", err.Error())
			return
		}
		h.redirectProfileBind(c, "wechat", "")
		return
	}
	u, isNew, err := h.svc.WechatWebLogin(code)
	if err != nil {
		h.redirectSocial(c, "", "微信登录失败："+err.Error())
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

// ───────────────────────── 网页登录：小程序扫码即登（方案 B） ─────────────────────────

// WebLoginSession 创建网页登录会话（返回一次性票据）。
func (h *AuthHandler) WebLoginSession(c *gin.Context) {
	tk, err := h.svc.CreateWebLoginSession()
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"ticket": tk, "expires_in": 300})
}

// WebLoginQrcode 生成带 scene=票据 的小程序码（返回 base64 PNG）。
func (h *AuthHandler) WebLoginQrcode(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	img, err := h.svc.WebLoginQrcode(req.Ticket)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"image_base64": base64.StdEncoding.EncodeToString(img)})
}

// WebLoginConfirm 小程序侧确认网页登录（票据 + wx.login 的 code）。
func (h *AuthHandler) WebLoginConfirm(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket"`
		Code   string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.ConfirmWebLogin(req.Ticket, req.Code); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// WebLoginPoll 网页轮询登录会话：pending / confirmed(携带一次性兑换码) / expired。
func (h *AuthHandler) WebLoginPoll(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	status, exchange, err := h.svc.PollWebLogin(req.Ticket)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"status": status, "exchange_code": exchange})
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
	tok, err := h.svc.UnbindProvider(u.ID, req.Provider)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	// 返回新 token：当前网页端保持登录；其他端（如小程序）旧 token 已失效
	response.OK(c, gin.H{"token": tok})
}
