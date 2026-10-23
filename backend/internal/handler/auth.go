package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// AuthHandler 鉴权接口。
type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func userView(u *model.User) gin.H {
	return gin.H{
		"id":                u.ID,
		"username":          u.Username,
		"email":             u.Email,
		"role":              u.Role,
		"wechat":            u.Wechat,
		"qq":                u.QQ,
		"phone":             u.Phone,
		"nickname":          u.Nickname,
		"avatar":            u.Avatar,
		"title":             u.Title,
		"profile_public":    u.ProfilePublic,
		"public_fields":     u.PublicFields,
		"email_verified":    u.EmailVerified,
		"wechat_bound":      u.WxOpenid != "",
		"qq_bound":          u.QqOpenid != "",
		"status":            u.Status,
		"agreement_version": u.AgreementVersion,
		"created_at":        u.CreatedAt,
	}
}

// Register 注册。
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	u, err := h.svc.Register(req)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	// qq_pending：QQ 邮箱且仍需验证（未激活）→ 前端弹查收提示
	qqPending := !u.EmailVerified && service.IsQQMail(req.Email)
	response.OK(c, gin.H{"user": userView(u), "qq_pending": qqPending})
}

// Profile 修改个人资料与公开主页设置。
func (h *AuthHandler) Profile(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	var req service.ProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.UpdateProfile(u, req); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// Login 登录。
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	ip := middleware.ClientIP(c)
	tok, u, err := h.svc.Login(req.Identifier, req.Password, ip)
	if err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, gin.H{"token": tok, "user": userView(u)})
}

// VerifyEmail 激活。
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	tok := c.Query("token")
	if err := h.svc.VerifyEmail(tok); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	response.OK(c, nil)
}

// ResendVerify 重发激活邮件。
func (h *AuthHandler) ResendVerify(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	c.ShouldBindJSON(&req)
	if err := h.svc.Resend(req.Email); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	response.OK(c, nil)
}

// AdminResendVerify 管理员为指定用户重发验证邮件。
func (h *AuthHandler) AdminResendVerify(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.AdminResendVerify(actor, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// AdminActivate 超级管理员直接激活用户（跳过邮箱验证）。
func (h *AuthHandler) AdminActivate(c *gin.Context) {
	actor := adminUser(c)
	if actor == nil {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.AdminActivate(actor, uint(id)); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// Forgot 找回密码。
func (h *AuthHandler) Forgot(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	c.ShouldBindJSON(&req)
	if err := h.svc.Forgot(req.Email); err != nil {
		// 未配置邮件时提示不可用
		response.FailCode(c, errs.ErrValidation, "未配置邮件服务，找回密码不可用")
		return
	}
	response.OK(c, nil)
}

// Reset 重置密码。
func (h *AuthHandler) Reset(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.Reset(req.Token, req.Password); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	response.OK(c, nil)
}

// Me 当前用户。
func (h *AuthHandler) Me(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if u == nil {
		response.FailCode(c, errs.CodeUnauthorized, "")
		return
	}
	response.OK(c, userView(u))
}

// ChangePassword 修改密码。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var req struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.ChangePassword(u, req.Old, req.New); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// ChangeEmail 修改邮箱。
func (h *AuthHandler) ChangeEmail(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if err := h.svc.ChangeEmail(u, req.Email); err != nil {
		code, _ := errs.As(err)
		response.FailCode(c, code, err.Error())
		return
	}
	response.OK(c, nil)
}

// Logout 登出（前端清除 token；阶段一不实现单 token 黑名单）。
func (h *AuthHandler) Logout(c *gin.Context) {
	response.OK(c, nil)
}

// DeleteAccount 注销账号（软删+匿名化）。
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	u := middleware.CurrentUser(c)
	if err := h.svc.Anonymize(u); err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	response.OK(c, nil)
}
