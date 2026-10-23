package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/token"
	"tzwall/backend/internal/pkg/validate"
)

// SocialCreds 第三方登录凭证。后台配置中心优先，回退到 .env；二者皆空表示对应方式未启用。
type SocialCreds struct {
	WechatMPAppID        string
	WechatMPSecret       string
	QQAppID              string
	QQSecret             string
	QQRedirectURI        string
	WechatWebAppID       string
	WechatWebSecret      string
	WechatWebRedirectURI string
	WechatEnabled        bool
	WechatWebEnabled     bool
	QQEnabled            bool
}

// resolveSocial 解析第三方登录凭证：后台配置优先于 .env。
func (s *AuthService) resolveSocial() SocialCreds {
	get := func(k, d string) string {
		if v := strings.TrimSpace(s.store.Get(k)); v != "" {
			return v
		}
		return d
	}
	// 敏感项在库中以 enc:v1: 密文保存，必须用 SecretValue 解密后再使用，否则会把密文当密钥发给第三方（如微信 40125）
	secret := func(k, d string) string {
		if v := strings.TrimSpace(s.store.SecretValue(k)); v != "" {
			return v
		}
		return d
	}
	c := SocialCreds{
		WechatMPAppID:        get("wechat_mp_appid", s.cfg.WechatMPAppID),
		WechatMPSecret:       secret("wechat_mp_secret", s.cfg.WechatMPSecret),
		QQAppID:              get("qq_oauth_appid", s.cfg.QQAppID),
		QQSecret:             secret("qq_oauth_secret", s.cfg.QQSecret),
		QQRedirectURI:        get("qq_oauth_redirect_uri", s.cfg.QQRedirectURI),
		WechatWebAppID:       get("wechat_web_appid", s.cfg.WechatWebAppID),
		WechatWebSecret:      secret("wechat_web_secret", s.cfg.WechatWebSecret),
		WechatWebRedirectURI: get("wechat_web_redirect_uri", s.cfg.WechatWebRedirectURI),
	}
	c.WechatEnabled = c.WechatMPAppID != "" && c.WechatMPSecret != ""
	c.WechatWebEnabled = c.WechatWebAppID != "" && c.WechatWebSecret != ""
	c.QQEnabled = c.QQAppID != "" && c.QQSecret != "" && c.QQRedirectURI != ""
	return c
}

// httpGet 发起带超时与状态码校验的 GET 请求，返回响应体字节。
func httpGet(u string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// randState 生成随机防 CSRF / 一次性兑换码。
func randState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ───────────────────────── 微信小程序 ─────────────────────────

type wechatSessionResp struct {
	Openid     string `json:"openid"`
	Unionid    string `json:"unionid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// wechatCode2Session 用 wx.login 的 code 换取 openid/unionid。
// TODO: 凭证到位后无需改动；当前若未配置会返回 ErrThirdPartyUnavailable。
func (s *AuthService) wechatCode2Session(code string) (*wechatSessionResp, error) {
	c := s.resolveSocial()
	if !c.WechatEnabled {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, nil)
	}
	u := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		url.QueryEscape(c.WechatMPAppID), url.QueryEscape(c.WechatMPSecret), url.QueryEscape(code),
	)
	data, err := httpGet(u, 10*time.Second)
	if err != nil {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	var r wechatSessionResp
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	if r.ErrCode != 0 {
		// TODO: 凭证到位后按 errcode 细化（40029 无效 code / 45011 频率限制 等）
		return nil, errs.New(errs.ErrThirdPartyUnavailable, fmt.Errorf("wechat code2session errcode=%d %s", r.ErrCode, r.ErrMsg))
	}
	return &r, nil
}

// WechatLogin 微信小程序登录：已绑定账号直接登录；未绑定则返回注册票据，要求用户填写用户名+密码后完成注册
// （不再按 openid 自动生成账号）。
func (s *AuthService) WechatLogin(code string) (token string, user *model.User, ticket string, err error) {
	if strings.TrimSpace(code) == "" {
		return "", nil, "", errs.New(errs.ErrValidation, nil)
	}
	sess, err := s.wechatCode2Session(code)
	if err != nil {
		return "", nil, "", err
	}
	u, err := s.findExistingSocial("wechat", sess.Openid, sess.Unionid)
	if err != nil {
		return "", nil, "", err
	}
	if u != nil {
		tok, e := s.IssueToken(u)
		if e != nil {
			return "", nil, "", e
		}
		return tok, u, "", nil
	}
	// 未绑定：签发一次性注册票据（携带已由微信校验的 openid/unionid），待用户填写用户名+密码
	tk := randState()
	s.cache.Set("wx_reg_ticket:"+tk, sess.Openid+"|"+sess.Unionid, 10*time.Minute)
	return "", nil, tk, nil
}

// WechatRegister 小程序首次登录完成注册：凭票据创建账号，要求用户自定用户名与密码。
func (s *AuthService) WechatRegister(ticket, username, password string) (string, *model.User, error) {
	v, ok := s.cache.Get("wx_reg_ticket:" + ticket)
	if !ok {
		return "", nil, errs.New(errs.ErrTokenInvalid, nil)
	}
	parts := strings.SplitN(v, "|", 2)
	if len(parts) != 2 {
		return "", nil, errs.New(errs.ErrTokenInvalid, nil)
	}
	openid, unionid := parts[0], parts[1]
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if !validate.ValidUsername(username) {
		return "", nil, errs.New(errs.ErrInvalidUsername, nil)
	}
	if len(password) < 6 || len(password) > 64 {
		return "", nil, errs.New(errs.ErrValidation, fmt.Errorf("密码长度需 6-64 位"))
	}
	if s.words != nil {
		if err := s.words.CheckUsername(username); err != nil {
			return "", nil, err
		}
	}
	// openid 已被占用（并发/重复绑定）
	if owner := s.socialOwner("wechat", openid, unionid); owner != 0 {
		return "", nil, errs.New(errs.ErrIdentityConflict, nil)
	}
	var cnt int64
	s.db.Model(&model.User{}).Where("username = ?", username).Count(&cnt)
	if cnt > 0 {
		return "", nil, errs.New(errs.ErrUserExists, nil)
	}
	email := fmt.Sprintf("wx_%s@tzwall.local", openid)
	s.db.Model(&model.User{}).Where("email = ?", email).Count(&cnt)
	if cnt > 0 {
		return "", nil, errs.New(errs.ErrEmailExists, nil)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, errs.New(errs.CodeInternal, err)
	}
	u := &model.User{
		Username: username, Email: email, PasswordHash: string(hash),
		Role: model.RoleUser, Status: model.UserStatusActive,
		EmailVerified: true, AgreementVersion: "1.0",
		WxOpenid: openid, WxUnionid: unionid,
	}
	if err := s.db.Create(u).Error; err != nil {
		// 并发兜底：若该微信已建号则直接登录
		if exist, ferr := s.findExistingSocial("wechat", openid, unionid); ferr == nil && exist != nil {
			tok, _ := s.IssueToken(exist)
			return tok, exist, nil
		}
		return "", nil, errs.New(errs.CodeInternal, err)
	}
	s.cache.Del("wx_reg_ticket:" + ticket)
	tok, err := s.IssueToken(u)
	if err != nil {
		return "", nil, errs.New(errs.CodeInternal, err)
	}
	return tok, u, nil
}

// ───────────────────────── QQ OAuth（网站应用） ─────────────────────────

type qqTokenResp struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	ErrorCode    int    `json:"error_code"`
	ErrorMsg     string `json:"error_description"`
}

type qqMeResp struct {
	ClientID string `json:"client_id"`
	Openid   string `json:"openid"`
	ErrorCode int   `json:"error_code"`
	ErrorMsg  string `json:"error"`
}

type qqUserinfoResp struct {
	Ret       int    `json:"ret"`
	Msg       string `json:"msg"`
	Nickname  string `json:"nickname"`
	FigureurlQQ2 string `json:"figureurl_qq_2"`
}

// QQAuthorize 生成登录授权地址，并签发/缓存 state（CSRF 防护）。
func (s *AuthService) QQAuthorize() (string, string, error) {
	state := randState()
	s.cache.Set("qq_state:"+state, "login", 10*time.Minute)
	u, err := s.qqAuthorizeURL(state)
	return u, state, err
}

// QQAuthorizeBind 生成“绑定到当前账号”的授权地址（state 记录绑定目标用户）。
func (s *AuthService) QQAuthorizeBind(uid uint) (string, string, error) {
	state := randState()
	s.cache.Set("qq_state:"+state, fmt.Sprintf("bind:%d", uid), 10*time.Minute)
	u, err := s.qqAuthorizeURL(state)
	return u, state, err
}

// QQStateConsume 校验并一次性消费 state，返回模式（login/bind）与绑定目标用户。
func (s *AuthService) QQStateConsume(state string) (mode string, uid uint, ok bool) {
	v, ok := s.cache.Get("qq_state:" + state)
	if !ok {
		return "", 0, false
	}
	s.cache.Del("qq_state:" + state)
	if strings.HasPrefix(v, "bind:") {
		fmt.Sscanf(strings.TrimPrefix(v, "bind:"), "%d", &uid)
		return "bind", uid, true
	}
	return "login", 0, true
}

func (s *AuthService) qqAuthorizeURL(state string) (string, error) {
	c := s.resolveSocial()
	if !c.QQEnabled {
		return "", errs.New(errs.ErrThirdPartyUnavailable, nil)
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.QQAppID)
	q.Set("redirect_uri", c.QQRedirectURI)
	q.Set("state", state)
	q.Set("scope", "get_user_info")
	return "https://graph.qq.com/oauth2.0/authorize?" + q.Encode(), nil
}



func (s *AuthService) qqExchangeToken(code string) (*qqTokenResp, error) {
	c := s.resolveSocial()
	if !c.QQEnabled {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, nil)
	}
	q := url.Values{}
	q.Set("grant_type", "authorization_code")
	q.Set("client_id", c.QQAppID)
	q.Set("client_secret", c.QQSecret)
	q.Set("code", code)
	q.Set("redirect_uri", c.QQRedirectURI)
	q.Set("fmt", "json")
	data, err := httpGet("https://graph.qq.com/oauth2.0/token?"+q.Encode(), 10*time.Second)
	if err != nil {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	var r qqTokenResp
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	if r.ErrorCode != 0 {
		// TODO: 凭证到位后按 error_code 细化
		return nil, errs.New(errs.ErrThirdPartyUnavailable, fmt.Errorf("qq token error_code=%d %s", r.ErrorCode, r.ErrorMsg))
	}
	return &r, nil
}

func (s *AuthService) qqOpenid(accessToken string) (string, error) {
	u := "https://graph.qq.com/oauth2.0/me?access_token=" + url.QueryEscape(accessToken) + "&fmt=json"
	data, err := httpGet(u, 10*time.Second)
	if err != nil {
		return "", errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	var r qqMeResp
	if err := json.Unmarshal(data, &r); err != nil {
		return "", errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	if r.ErrorCode != 0 || r.Openid == "" {
		return "", errs.New(errs.ErrThirdPartyUnavailable, fmt.Errorf("qq me error_code=%d %s", r.ErrorCode, r.ErrorMsg))
	}
	return r.Openid, nil
}

// qqUserInfo 拉取昵称/头像（非致命：失败不影响登录）。
func (s *AuthService) qqUserInfo(accessToken, openid, appid string) (nickname, avatar string, err error) {
	q := url.Values{}
	q.Set("access_token", accessToken)
	q.Set("oauth_consumer_key", appid)
	q.Set("openid", openid)
	data, e := httpGet("https://graph.qq.com/user/get_user_info?"+q.Encode(), 10*time.Second)
	if e != nil {
		return "", "", nil
	}
	var r qqUserinfoResp
	if json.Unmarshal(data, &r) != nil || r.Ret != 0 {
		return "", "", nil
	}
	return r.Nickname, r.FigureurlQQ2, nil
}

// QQCallback 处理 QQ 授权回调：换 token → 取 openid → 解析/建号。返回用户与是否新建。
func (s *AuthService) QQCallback(code string) (*model.User, bool, error) {
	if strings.TrimSpace(code) == "" {
		return nil, false, errs.New(errs.ErrValidation, nil)
	}
	tok, err := s.qqExchangeToken(code)
	if err != nil {
		return nil, false, err
	}
	openid, err := s.qqOpenid(tok.AccessToken)
	if err != nil {
		return nil, false, err
	}
	nickname, avatar := "", ""
	if c := s.resolveSocial(); c.QQEnabled {
		nickname, avatar, _ = s.qqUserInfo(tok.AccessToken, openid, c.QQAppID)
	}
	return s.findOrCreateSocialUser("qq", openid, "", nickname, avatar)
}

// ───────────────────────── 微信网页应用 OAuth（扫码 qrconnect） ─────────────────────────

type wechatWebTokenResp struct {
	AccessToken string `json:"access_token"`
	Openid      string `json:"openid"`
	Unionid     string `json:"unionid"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

func (s *AuthService) wechatWebCallbackURI() string {
	c := s.resolveSocial()
	if strings.TrimSpace(c.WechatWebRedirectURI) != "" {
		return c.WechatWebRedirectURI
	}
	return s.siteBaseURL() + "/api/auth/wechat/web/callback"
}

// WechatWebAuthorize 生成微信网页扫码授权地址。mode: "login" 登录 / "bind" 绑定（需 uid）。
func (s *AuthService) WechatWebAuthorize(mode string, uid uint) (string, error) {
	c := s.resolveSocial()
	if !c.WechatWebEnabled {
		return "", errs.New(errs.ErrThirdPartyUnavailable, nil)
	}
	state := randState()
	val := "login"
	if mode == "bind" {
		val = fmt.Sprintf("bind:%d", uid)
	}
	s.cache.Set("wechat_web_state:"+state, val, 10*time.Minute)
	q := url.Values{}
	q.Set("appid", c.WechatWebAppID)
	q.Set("redirect_uri", s.wechatWebCallbackURI())
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_login")
	q.Set("state", state)
	return "https://open.weixin.qq.com/connect/qrconnect?" + q.Encode() + "#wechat_redirect", nil
}

// WechatWebStateConsume 校验并一次性消费 state。
func (s *AuthService) WechatWebStateConsume(state string) (mode string, uid uint, ok bool) {
	v, ok := s.cache.Get("wechat_web_state:" + state)
	if !ok {
		return "", 0, false
	}
	s.cache.Del("wechat_web_state:" + state)
	if strings.HasPrefix(v, "bind:") {
		fmt.Sscanf(strings.TrimPrefix(v, "bind:"), "%d", &uid)
		return "bind", uid, true
	}
	return "login", 0, true
}

func (s *AuthService) wechatWebToken(code string) (*wechatWebTokenResp, error) {
	c := s.resolveSocial()
	if !c.WechatWebEnabled {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, nil)
	}
	q := url.Values{}
	q.Set("appid", c.WechatWebAppID)
	q.Set("secret", c.WechatWebSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	data, err := httpGet("https://api.weixin.qq.com/sns/oauth2/access_token?"+q.Encode(), 10*time.Second)
	if err != nil {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	var r wechatWebTokenResp
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, err)
	}
	if r.ErrCode != 0 || r.Openid == "" {
		return nil, errs.New(errs.ErrThirdPartyUnavailable, fmt.Errorf("wechat web oauth errcode=%d %s", r.ErrCode, r.ErrMsg))
	}
	return &r, nil
}

// WechatWebLogin 微信网页授权登录：code → openid → 登录/自动建号（跳过邮箱验证）。
func (s *AuthService) WechatWebLogin(code string) (*model.User, bool, error) {
	if strings.TrimSpace(code) == "" {
		return nil, false, errs.New(errs.ErrValidation, nil)
	}
	t, err := s.wechatWebToken(code)
	if err != nil {
		return nil, false, err
	}
	return s.findOrCreateSocialUser("wechat", t.Openid, t.Unionid, "", "")
}

// WechatWebBind 微信网页授权绑定到当前账号（冲突则 ErrIdentityConflict）。
func (s *AuthService) WechatWebBind(uid uint, code string) error {
	if _, err := s.userByID(uid); err != nil {
		return err
	}
	t, err := s.wechatWebToken(code)
	if err != nil {
		return err
	}
	return s.bindOpenid(uid, "wechat", t.Openid, t.Unionid, "", "")
}

// ───────────────────────── 账号查找 / 自动建号 ─────────────────────────

// findExistingSocial 按 unionid（跨应用统一身份）优先、再按 openid 查找已存在账号。
func (s *AuthService) findExistingSocial(provider, openid, unionid string) (*model.User, error) {
	if unionid != "" {
		var u model.User
		if err := s.db.Where("wx_unionid = ? OR qq_unionid = ?", unionid, unionid).First(&u).Error; err == nil {
			return &u, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	col := "wx_openid"
	if provider == "qq" {
		col = "qq_openid"
	}
	var u model.User
	if err := s.db.Where(col+" = ?", openid).First(&u).Error; err == nil {
		return &u, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, nil
}

// findOrCreateSocialUser 解析社交身份：命中则登录；未命中时（默认）自动建号，或按配置返回未关联错误。
func (s *AuthService) findOrCreateSocialUser(provider, openid, unionid, nickname, avatar string) (*model.User, bool, error) {
	if u, err := s.findExistingSocial(provider, openid, unionid); err != nil {
		return nil, false, err
	} else if u != nil {
		return u, false, nil
	}
	// 默认自动建号；仅当显式配置 social_auto_create=false 时关闭
	if strings.TrimSpace(s.store.Get("social_auto_create")) == "false" {
		return nil, false, errs.New(errs.ErrSocialNoAccount, nil)
	}
	return s.createSocialUser(provider, openid, unionid, nickname, avatar)
}

func (s *AuthService) createSocialUser(provider, openid, unionid, nickname, avatar string) (*model.User, bool, error) {
	prefix := "wx_"
	if provider == "qq" {
		prefix = "qq_"
	}
	username := genSocialUsername(prefix, openid)
	// 合成唯一、不可投递邮箱，规避 Email 唯一索引空串冲突
	email := fmt.Sprintf("%s%s@tzwall.local", prefix, openid)
	// 随机强密码：社交账号默认无法通过密码登录
	randPwd := uuid.NewString()
	hash, err := bcrypt.GenerateFromPassword([]byte(randPwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, false, errs.New(errs.CodeInternal, err)
	}
	u := &model.User{
		Username:         username,
		Email:            email,
		PasswordHash:     string(hash),
		Nickname:         nickname,
		Avatar:           avatar,
		Role:             model.RoleUser,
		Status:           model.UserStatusActive,
		EmailVerified:    true, // OAuth 登录/建号即视为已验证，跳过邮箱验证步骤
		AgreementVersion: "1.0",
	}
	if provider == "qq" {
		u.QqOpenid = openid
		u.QqUnionid = unionid
	} else {
		u.WxOpenid = openid
		u.WxUnionid = unionid
	}
	if err := s.db.Create(u).Error; err != nil {
		// 并发唯一冲突兜底：再查一次
		if exist, ferr := s.findExistingSocial(provider, openid, unionid); ferr == nil && exist != nil {
			return exist, false, nil
		}
		return nil, false, errs.New(errs.CodeInternal, err)
	}
	return u, true, nil
}

func genSocialUsername(prefix, openid string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, openid)
	if len(clean) > 8 {
		clean = clean[:8]
	}
	name := prefix + clean
	for len(name) < 6 {
		name += "0"
	}
	return name
}

// ───────────────────────── 绑定 / 解绑 ─────────────────────────

func (s *AuthService) socialOwner(provider, openid, unionid string) uint {
	var u model.User
	if unionid != "" {
		if err := s.db.Where("wx_unionid = ? OR qq_unionid = ?", unionid, unionid).First(&u).Error; err == nil {
			return u.ID
		}
	}
	col := "wx_openid"
	if provider == "qq" {
		col = "qq_openid"
	}
	if err := s.db.Where(col+" = ?", openid).First(&u).Error; err == nil {
		return u.ID
	}
	return 0
}

// BindProvider 将第三方身份绑定到当前已登录账号；若身份已属于他人则冲突。
func (s *AuthService) BindProvider(userID uint, provider, code string) error {
	if _, err := s.userByID(userID); err != nil {
		return err
	}
	var openid, unionid, nickname, avatar string
	switch provider {
	case "wechat":
		sess, e := s.wechatCode2Session(code)
		if e != nil {
			return e
		}
		openid, unionid = sess.Openid, sess.Unionid
	case "qq":
		tok, e := s.qqExchangeToken(code)
		if e != nil {
			return e
		}
		openid, e = s.qqOpenid(tok.AccessToken)
		if e != nil {
			return e
		}
		if c := s.resolveSocial(); c.QQEnabled {
			nickname, avatar, _ = s.qqUserInfo(tok.AccessToken, openid, c.QQAppID)
		}
	default:
		return errs.New(errs.ErrValidation, nil)
	}
	return s.bindOpenid(userID, provider, openid, unionid, nickname, avatar)
}

// bindOpenid 将已解析的第三方身份绑定到账号（跨账号冲突校验 + 更新）。
func (s *AuthService) bindOpenid(userID uint, provider, openid, unionid, nickname, avatar string) error {
	if owner := s.socialOwner(provider, openid, unionid); owner != 0 && owner != userID {
		return errs.New(errs.ErrIdentityConflict, nil)
	}
	updates := map[string]any{}
	switch provider {
	case "wechat":
		updates["wx_openid"] = openid
		if unionid != "" {
			updates["wx_unionid"] = unionid
		}
	case "qq":
		updates["qq_openid"] = openid
		if unionid != "" {
			updates["qq_unionid"] = unionid
		}
	default:
		return errs.New(errs.ErrValidation, nil)
	}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	if avatar != "" {
		updates["avatar"] = avatar
	}
	if err := s.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	return nil
}

// CreateBindCode 为当前账号生成微信绑定码（用于仅配置小程序时，在网页账号中心发起绑定）。
func (s *AuthService) CreateBindCode(userID uint) (string, error) {
	if _, err := s.userByID(userID); err != nil {
		return "", err
	}
	code := strings.ToUpper(randState()[:8])
	s.cache.Set("wx_bind_code:"+code, fmt.Sprintf("%d", userID), 10*time.Minute)
	return code, nil
}

// BindByCode 小程序内用绑定码把微信身份绑定到指定账号（凭绑定码 + wx.login 的 code）。
func (s *AuthService) BindByCode(bindCode, wxCode string) error {
	key := "wx_bind_code:" + strings.ToUpper(strings.TrimSpace(bindCode))
	v, ok := s.cache.Get(key)
	if !ok {
		return errs.New(errs.ErrTokenInvalid, fmt.Errorf("绑定码无效或已过期"))
	}
	if strings.TrimSpace(wxCode) == "" {
		return errs.New(errs.ErrValidation, nil)
	}
	var uid uint
	fmt.Sscanf(v, "%d", &uid)
	sess, err := s.wechatCode2Session(wxCode)
	if err != nil {
		return err
	}
	if err := s.bindOpenid(uid, "wechat", sess.Openid, sess.Unionid, "", ""); err != nil {
		return err
	}
	s.cache.Del(key)
	return nil
}

// UnbindProvider 解绑第三方身份；若解绑后将无任何登录方式则拒绝（需先设密码或保留其他方式）。
func (s *AuthService) UnbindProvider(userID uint, provider string) error {
	u, err := s.userByID(userID)
	if err != nil {
		return err
	}
	hasPwd := u.PasswordHash != ""
	remaining := 0
	if hasPwd {
		remaining++
	}
	if provider != "wechat" && u.WxOpenid != "" {
		remaining++
	}
	if provider != "qq" && u.QqOpenid != "" {
		remaining++
	}
	if remaining == 0 {
		return errs.New(errs.ErrValidation, fmt.Errorf("请先设置密码或保留至少一种登录方式"))
	}
	updates := map[string]any{}
	switch provider {
	case "wechat":
		updates["wx_openid"] = ""
		updates["wx_unionid"] = ""
	case "qq":
		updates["qq_openid"] = ""
		updates["qq_unionid"] = ""
	default:
		return errs.New(errs.ErrValidation, nil)
	}
	return s.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error
}

// ───────────────────────── 会话（JWT / 一次性 code） ─────────────────────────

func (s *AuthService) userByID(id uint) (*model.User, error) {
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, nil)
		}
		return nil, err
	}
	return &u, nil
}

// IssueToken 签发访问令牌（与密码登录同构：7 天、含 TokenVersion）。
func (s *AuthService) IssueToken(u *model.User) (string, error) {
	return token.Generate(s.cfg.JWTSecret, u.ID, string(u.Role), u.TokenVersion, 7*24*time.Hour)
}

// SaveSocialCode 将社交登录结果固化为一次性兑换码（避免 JWT 直接暴露在 URL）。
func (s *AuthService) SaveSocialCode(userID uint, isNew bool) (string, error) {
	code := randState()
	s.cache.Set("social_code:"+code, fmt.Sprintf("%d|%t", userID, isNew), 5*time.Minute)
	return code, nil
}

// RedeemSocialCode 兑换一次性 code 为用户与会话。
func (s *AuthService) RedeemSocialCode(code string) (*model.User, bool, error) {
	v, ok := s.cache.Get("social_code:" + code)
	if !ok {
		return nil, false, errs.New(errs.ErrTokenInvalid, nil)
	}
	s.cache.Del("social_code:" + code)
	parts := strings.SplitN(v, "|", 2)
	if len(parts) != 2 {
		return nil, false, errs.New(errs.ErrTokenInvalid, nil)
	}
	var id uint
	if _, err := fmt.Sscanf(parts[0], "%d", &id); err != nil {
		return nil, false, errs.New(errs.ErrTokenInvalid, nil)
	}
	isNew := parts[1] == "true"
	u, err := s.userByID(id)
	if err != nil {
		return nil, false, err
	}
	return u, isNew, nil
}

// SiteBaseURL 暴露站点外网地址（供 QQ 回调回跳前端）。
func (s *AuthService) SiteBaseURL() string { return s.siteBaseURL() }
