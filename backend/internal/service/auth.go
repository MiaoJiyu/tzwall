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
	return fmt.Sprintf(
		`<div style="font-family:-apple-system,Segoe UI,Roboto,'PingFang SC',sans-serif;font-size:14px;color:#222;line-height:1.7">`+
			`<p style="margin:0 0 12px">%s</p>`+
			`<p style="margin:0 0 12px"><a href="%s" style="color:#6b2a73;text-decoration:underline">%s</a></p>`+
			`<p style="margin:0;color:#888;font-size:12px">如链接无法点击，请复制以下地址到浏览器打开：<br>%s</p>`+
			`</div>`, prefix, link, link, link)
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

	// 是否需要邮箱验证：默认随邮件服务是否配置；QQ 邮箱可在后台开启“免验证直接注册”。
	needVerify := s.mailer.Configured()
	if needVerify && IsQQMail(req.Email) && s.store.Bool("qq_mail_skip_verify") {
		needVerify = false
	}
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
			_ = s.mailer.Send(context.Background(), u.Email, "【泰兴中学校园墙】邮箱验证",
				linkHTML("请点击以下链接完成邮箱验证：", s.verifyLink(u.EmailVerifyToken)))
		}()
	}
	return u, nil
}

// IsQQMail 判断是否为 QQ 邮箱：取“@”后域名，与 "qq.com" 不区分大小写精确匹配；
// 子域名（如 vip.qq.com）或其他变体均不算。
func IsQQMail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(email[at+1:]), "qq.com")
}

// ProfileReq 个人资料修改（头像/昵称/联系方式 + 真实姓名/就读学校 + 公开主页设置）。
// 头像/昵称/联系方式/公开字段列表使用指针：仅当请求「实际提供」该字段才更新，
// 避免局部更新（例如只保存公开设置开关）时把其它已填写的资料覆盖为空串。
type ProfileReq struct {
	Nickname      *string `json:"nickname"`
	Avatar        *string `json:"avatar"`
	Wechat        *string `json:"wechat"`
	QQ            *string `json:"qq"`
	Phone         *string `json:"phone"`
	RealName      string  `json:"real_name"` // 社交账号强制完善项（仅网页端）；空值不更新，避免误清空
	School        string  `json:"school"`    // 社交账号强制完善项（仅网页端）；空值不更新，避免误清空
	ProfilePublic *bool   `json:"profile_public"`
	PublicFields  *string `json:"public_fields"`
}

// UpdateProfile 修改个人资料与公开主页设置。
// 仅更新请求中实际提供的字段（指针非 nil），防止局部保存（如只切换公开开关、只改昵称）
// 把已填的联系方式 / 公开设置覆盖为空串，避免资料被意外清空。
func (s *AuthService) UpdateProfile(user *model.User, req ProfileReq) error {
	updates := map[string]any{}
	// 昵称
	if req.Nickname != nil {
		n := strings.TrimSpace(*req.Nickname)
		if len([]rune(n)) > 64 {
			return errs.New(errs.ErrValidation, fmt.Errorf("昵称长度超出限制"))
		}
		if s.words != nil && n != "" {
			if err := s.words.CheckContent(n); err != nil {
				return err
			}
		}
		updates["nickname"] = n
	}
	// 头像
	if req.Avatar != nil {
		a := strings.TrimSpace(*req.Avatar)
		if len([]rune(a)) > 512 {
			return errs.New(errs.ErrValidation, fmt.Errorf("头像地址过长"))
		}
		updates["avatar"] = a
	}
	// 微信
	if req.Wechat != nil {
		w := strings.TrimSpace(*req.Wechat)
		if len([]rune(w)) > 64 {
			return errs.New(errs.ErrValidation, fmt.Errorf("微信长度超出限制"))
		}
		updates["wechat"] = w
	}
	// QQ
	if req.QQ != nil {
		q := strings.TrimSpace(*req.QQ)
		if len([]rune(q)) > 32 {
			return errs.New(errs.ErrValidation, fmt.Errorf("QQ 长度超出限制"))
		}
		updates["qq"] = q
	}
	// 手机
	if req.Phone != nil {
		p := strings.TrimSpace(*req.Phone)
		if len([]rune(p)) > 32 {
			return errs.New(errs.ErrValidation, fmt.Errorf("手机号长度超出限制"))
		}
		updates["phone"] = p
	}
	// 真实姓名 / 就读学校：仅当传入非空时更新（不允许经此接口置空），避免误清空
	realName := strings.TrimSpace(req.RealName)
	school := strings.TrimSpace(req.School)
	if realName != "" && (len([]rune(realName)) < 2 || len([]rune(realName)) > 32) {
		return errs.New(errs.ErrValidation, fmt.Errorf("真实姓名长度需 2-32 字"))
	}
	if school != "" && (len([]rune(school)) < 2 || len([]rune(school)) > 64) {
		return errs.New(errs.ErrValidation, fmt.Errorf("就读学校长度需 2-64 字"))
	}
	if s.words != nil {
		if err := s.words.CheckContent(realName + " " + school); err != nil {
			return err
		}
	}
	if realName != "" {
		updates["real_name"] = realName
	}
	if school != "" {
		updates["school"] = school
	}
	// 公开字段白名单：仅当请求提供 public_fields 时才更新（避免只存资料时清空公开设置）
	if req.PublicFields != nil {
		allowed := map[string]bool{"wechat": true, "qq": true, "phone": true, "school": true, "real_name": true}
		clean := make([]string, 0, 5)
		for _, f := range strings.Split(*req.PublicFields, ",") {
			f = strings.TrimSpace(f)
			if allowed[f] {
				clean = append(clean, f)
			}
		}
		updates["public_fields"] = strings.Join(clean, ",")
	}
	if req.ProfilePublic != nil {
		updates["profile_public"] = *req.ProfilePublic
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.Model(&model.User{}).Where("id = ?", user.ID).Updates(updates).Error
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
	_ = s.mailer.Send(context.Background(), u.Email, "【泰兴中学校园墙】邮箱验证",
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
		_ = s.mailer.Send(context.Background(), email, "【泰兴中学校园墙】邮箱验证",
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
	_ = s.mailer.Send(context.Background(), u.Email, "【泰兴中学校园墙】密码重置",
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
