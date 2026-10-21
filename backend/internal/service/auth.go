package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/config"
	"tzwall/backend/internal/mailer"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/token"
	"tzwall/backend/internal/pkg/validate"
)

// AuthService 账号与认证业务。
type AuthService struct {
	db     *gorm.DB
	cfg    *config.Config
	cache  cache.Cache
	mailer mailer.Mailer
	store  *ConfigStore
	words  *WordFilterService
}

func NewAuth(db *gorm.DB, cfg *config.Config, c cache.Cache, m mailer.Mailer, store *ConfigStore) *AuthService {
	return &AuthService{db: db, cfg: cfg, cache: c, mailer: m, store: store}
}

// WithWordFilter 注入词表服务（注册与改名时校验用户名）。
func (s *AuthService) WithWordFilter(w *WordFilterService) *AuthService {
	s.words = w
	return s
}

type RegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	RealName string `json:"real_name"`
	School   string `json:"school"`
	Wechat   string `json:"wechat"`
	QQ       string `json:"qq"`
	Phone    string `json:"phone"`
	Agree    bool   `json:"agree"`
}

// siteBaseURL 站点外部访问地址（用于生成邮件内可点击链接）。
func (s *AuthService) siteBaseURL() string {
	base := strings.TrimSpace(s.store.Get("site_url"))
	if base == "" {
		base = "http://localhost"
	}
	return strings.TrimRight(base, "/")
}

// verifyLink 邮箱激活绝对链接（前端 /verify 路由）。
func (s *AuthService) verifyLink(token string) string {
	return s.siteBaseURL() + "/verify?token=" + token
}

// resetLink 重置密码绝对链接（前端 /reset 路由）。
func (s *AuthService) resetLink(token string) string {
	return s.siteBaseURL() + "/reset?token=" + token
}

// linkHTML 生成含可点击绝对链接的邮件正文（同时展示链接文本便于复制）。
func linkHTML(prefix, link string) string {
	return prefix + "<br><a href=\"" + link + "\" target=\"_blank\" style=\"color:#6b2a73\">" + link + "</a><br>若无法点击，请复制上述链接到浏览器打开。"
}

// Register 注册。
func (s *AuthService) Register(req RegisterReq) (*model.User, error) {
	if !validate.ValidUsername(req.Username) {
		return nil, errs.New(errs.ErrInvalidUsername, nil)
	}
	if !validate.ValidEmail(req.Email) {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if !req.Agree {
		return nil, errs.New(errs.ErrAgreementRequired, nil)
	}
	// 违禁词（用户名）
	if s.words != nil {
		if err := s.words.CheckUsername(req.Username); err != nil {
			return nil, err
		}
	}
	var cnt int64
	s.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&cnt)
	if cnt > 0 {
		return nil, errs.New(errs.ErrUserExists, nil)
	}
	s.db.Model(&model.User{}).Where("email = ?", req.Email).Count(&cnt)
	if cnt > 0 {
		return nil, errs.New(errs.ErrEmailExists, nil)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}

	needVerify := s.mailer.Configured()
	u := &model.User{
		Username:         req.Username,
		Email:            req.Email,
		PasswordHash:     string(hash),
		RealName:         req.RealName,
		School:           req.School,
		Wechat:           req.Wechat,
		QQ:               req.QQ,
		Phone:            req.Phone,
		Role:             model.RoleUser,
		EmailVerified:    !needVerify,
		Status:           model.UserStatusActive,
		AgreementVersion: "1.0",
	}
	if needVerify {
		u.EmailVerifyToken = newToken()
		t := time.Now().Add(time.Duration(s.store.Int("email_verify_ttl_hours", 24)) * time.Hour)
		u.EmailVerifyExpiry = &t
	}
	if err := s.db.Create(u).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	if needVerify {
		// 异步发送激活邮件，避免 SMTP 不可达时阻塞注册响应（issue 10）
		go func() {
			_ = s.mailer.Send(context.Background(), u.Email, "激活您的校园墙账号",
				linkHTML("请点击以下链接完成邮箱验证：", s.verifyLink(u.EmailVerifyToken)))
		}()
	}
	return u, nil
}

// Login 登录，返回 token。
func (s *AuthService) Login(identifier, password, ip string) (string, *model.User, error) {
	var u model.User
	if err := s.db.Where("username = ? OR email = ?", identifier, identifier).First(&u).Error; err != nil {
		return "", nil, errs.New(errs.ErrInvalidCredentials, nil)
	}
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		return "", nil, errs.New(errs.ErrAccountLocked, nil)
	}
	if u.IsBanned() {
		return "", nil, errs.New(errs.ErrBanned, nil)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		limit := s.store.Int("login_fail_user_limit", 5)
		if u.IsAdmin() {
			limit = s.store.Int("login_fail_admin_limit", 10)
		}
		lockMin := s.store.Int("login_lock_minutes", 15)
		n := s.cache.Incr("lf:"+u.Username+":"+ip, time.Duration(lockMin)*time.Minute)
		if int(n) >= limit {
			t := time.Now().Add(time.Duration(lockMin) * time.Minute)
			s.db.Model(&u).Update("locked_until", &t)
		}
		return "", nil, errs.New(errs.ErrInvalidCredentials, nil)
	}
	if s.mailer.Configured() && !u.EmailVerified {
		return "", nil, errs.New(errs.ErrEmailNotVerified, nil)
	}
	s.cache.Del("lf:" + u.Username + ":" + ip)
	tok, err := token.Generate(s.cfg.JWTSecret, u.ID, string(u.Role), u.TokenVersion, 7*24*time.Hour)
	if err != nil {
		return "", nil, errs.New(errs.CodeInternal, err)
	}
	return tok, &u, nil
}

// Resend 未激活用户重发激活邮件（未配置 SMTP 时无效）。
func (s *AuthService) Resend(email string) error {
	if !s.mailer.Configured() {
		return errs.New(errs.ErrValidation, nil)
	}
	var u model.User
	if err := s.db.Where("email = ?", email).First(&u).Error; err != nil {
		return nil // 不泄露
	}
	if u.EmailVerified {
		return nil
	}
	u.EmailVerifyToken = newToken()
	t := time.Now().Add(time.Duration(s.store.Int("email_verify_ttl_hours", 24)) * time.Hour)
	s.db.Model(&u).Updates(map[string]any{"email_verify_token": u.EmailVerifyToken, "email_verify_expiry": &t})
	_ = s.mailer.Send(nil, u.Email, "激活您的校园墙账号",
		linkHTML("请点击以下链接完成邮箱验证：", s.verifyLink(u.EmailVerifyToken)))
	return nil
}

// AdminResendVerify 管理员为指定用户重发邮箱验证邮件。
func (s *AuthService) AdminResendVerify(actor *model.User, targetID uint) error {
	if actor == nil || !actor.IsAdmin() {
		return errs.New(errs.CodeForbidden, nil)
	}
	if !s.mailer.Configured() {
		return errs.New(errs.ErrValidation, fmt.Errorf("未配置邮件服务"))
	}
	var u model.User
	if err := s.db.First(&u, targetID).Error; err != nil {
		return errs.New(errs.CodeNotFound, err)
	}
	if u.EmailVerified {
		return errs.New(errs.ErrValidation, fmt.Errorf("该用户邮箱已验证，无需重发"))
	}
	u.EmailVerifyToken = newToken()
	exp := time.Now().Add(time.Duration(s.store.Int("email_verify_ttl_hours", 24)) * time.Hour)
	if err := s.db.Model(&u).Updates(map[string]any{"email_verify_token": u.EmailVerifyToken, "email_verify_expiry": &exp}).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	email, tok := u.Email, u.EmailVerifyToken
	go func() {
		_ = s.mailer.Send(context.Background(), email, "激活您的校园墙账号",
			linkHTML("请点击以下链接完成邮箱验证：", s.verifyLink(tok)))
	}()
	return nil
}

// AdminActivate 超级管理员直接激活用户（跳过邮箱验证）。
func (s *AuthService) AdminActivate(actor *model.User, targetID uint) error {
	if actor == nil || !actor.IsSuperAdmin() {
		return errs.New(errs.CodeForbidden, nil)
	}
	var u model.User
	if err := s.db.First(&u, targetID).Error; err != nil {
		return errs.New(errs.CodeNotFound, err)
	}
	if err := s.db.Model(&u).Updates(map[string]any{
		"email_verified":      true,
		"email_verify_token":  "",
		"email_verify_expiry": nil,
	}).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	return nil
}

// VerifyEmail 激活邮箱。
func (s *AuthService) VerifyEmail(t string) error {
	var u model.User
	if err := s.db.Where("email_verify_token = ?", t).First(&u).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if u.EmailVerifyExpiry != nil && u.EmailVerifyExpiry.Before(time.Now()) {
		return errs.New(errs.ErrValidation, nil)
	}
	s.db.Model(&u).Updates(map[string]any{"email_verified": true, "email_verify_token": ""})
	return nil
}

// Forgot 找回密码（未配置 SMTP 不可用）。
func (s *AuthService) Forgot(email string) error {
	if !s.mailer.Configured() {
		return errs.New(errs.ErrValidation, nil)
	}
	var u model.User
	if err := s.db.Where("email = ?", email).First(&u).Error; err != nil {
		return nil // 不泄露邮箱是否存在
	}
	u.EmailVerifyToken = newToken()
	t := time.Now().Add(30 * time.Minute)
	s.db.Model(&u).Updates(map[string]any{"email_verify_token": u.EmailVerifyToken, "email_verify_expiry": &t})
	_ = s.mailer.Send(nil, u.Email, "重置密码",
		linkHTML("请点击以下链接重置密码：", s.resetLink(u.EmailVerifyToken)))
	return nil
}

// Reset 重置密码并使旧 token 失效。
func (s *AuthService) Reset(t, pwd string) error {
	var u model.User
	if err := s.db.Where("email_verify_token = ?", t).First(&u).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if u.EmailVerifyExpiry != nil && u.EmailVerifyExpiry.Before(time.Now()) {
		return errs.New(errs.ErrValidation, nil)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	s.db.Model(&u).Updates(map[string]any{
		"password_hash":      string(hash),
		"email_verify_token": "",
		"token_version":      gorm.Expr("token_version + 1"),
	})
	return nil
}

// ChangePassword 修改密码。
func (s *AuthService) ChangePassword(u *model.User, old, new string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(old)); err != nil {
		return errs.New(errs.ErrInvalidCredentials, nil)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(new), bcrypt.DefaultCost)
	if err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	s.db.Model(u).Updates(map[string]any{
		"password_hash": string(hash),
		"token_version": gorm.Expr("token_version + 1"),
	})
	return nil
}

// ChangeEmail 修改邮箱（阶段一直接置已验证）。
func (s *AuthService) ChangeEmail(u *model.User, email string) error {
	if !validate.ValidEmail(email) {
		return errs.New(errs.ErrValidation, nil)
	}
	var cnt int64
	s.db.Model(&model.User{}).Where("email = ? AND id <> ?", email, u.ID).Count(&cnt)
	if cnt > 0 {
		return errs.New(errs.ErrEmailExists, nil)
	}
	s.db.Model(u).Update("email", email)
	return nil
}

// Anonymize 注销：软删除 + 匿名化用户名/联系方式，邮箱不可再注册。
func (s *AuthService) Anonymize(u *model.User) error {
	anon := "deleted_" + newToken()[:8]
	s.db.Model(u).Updates(map[string]any{
		"status":        model.UserStatusDeleted,
		"username":      anon,
		"real_name":     "",
		"school":        "",
		"wechat":        "",
		"qq":            "",
		"phone":         "",
		"token_version": gorm.Expr("token_version + 1"),
		"deleted_at":    time.Now(),
	})
	return nil
}

func newToken() string {
	return uuid.NewString()
}
