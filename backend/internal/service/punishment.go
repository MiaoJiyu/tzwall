package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/config"
	"tzwall/backend/internal/mailer"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/validate"
)

// Snapshot 用户处罚快照：由缓存驱动，避免每次请求查库。
type Snapshot struct {
	Banned        bool              `json:"banned"`
	Muted         bool              `json:"muted"`
	MuteReason    string            `json:"mute_reason,omitempty"`
	MuteExpires   string            `json:"mute_expires,omitempty"`
	MutePermanent bool              `json:"mute_permanent"`
	ForceReset    bool              `json:"force_reset"`
	Warn          *model.Punishment `json:"warn,omitempty"`
}

// PunishRequest 创建处罚请求。
type PunishRequest struct {
	UserID      uint
	Type        model.PunishmentType
	Reason      string
	Permanent   bool
	ExpiresAt   *time.Time
	TargetIP    string
	NewUsername string
}

// PunishmentService 处罚全流程：创建、撤销、到期自动解除、快照、IP 封禁。
type PunishmentService struct {
	db    *gorm.DB
	cfg   *config.Config
	c     cache.Cache
	store *ConfigStore
	m     mailer.Mailer
	words *WordFilterService
}

func NewPunishment(db *gorm.DB, cfg *config.Config, c cache.Cache, store *ConfigStore, m mailer.Mailer) *PunishmentService {
	return &PunishmentService{db: db, cfg: cfg, c: c, store: store, m: m}
}

// WithWordFilter 注入词表服务（强制改密的新用户名校验）。
func (s *PunishmentService) WithWordFilter(w *WordFilterService) *PunishmentService {
	s.words = w
	return s
}

const snapshotTTL = 30 * time.Second

// Snapshot 取用户处罚快照（缓存 30 秒）。
func (s *PunishmentService) Snapshot(userID uint) Snapshot {
	if userID == 0 {
		return Snapshot{}
	}
	key := fmt.Sprintf("pun:%d", userID)
	if s.c != nil {
		if v, ok := s.c.Get(key); ok {
			var snap Snapshot
			if err := json.Unmarshal([]byte(v), &snap); err == nil {
				return snap
			}
		}
	}
	snap := s.build(userID)
	if s.c != nil {
		if b, err := json.Marshal(snap); err == nil {
			s.c.Set(key, string(b), snapshotTTL)
		}
	}
	return snap
}

// Invalidate 失效某用户快照。
func (s *PunishmentService) Invalidate(userID uint) {
	if s.c != nil && userID > 0 {
		s.c.Del(fmt.Sprintf("pun:%d", userID))
	}
}

func (s *PunishmentService) build(userID uint) Snapshot {
	now := time.Now()
	var rows []model.Punishment
	s.db.Where("user_id = ? AND revoked = ?", userID, false).Find(&rows)

	snap := Snapshot{}
	var u model.User
	if s.db.First(&u, userID).Error == nil && u.IsBanned() {
		snap.Banned = true
	}
	for i := range rows {
		p := rows[i]
		if !p.Permanent && p.ExpiresAt != nil && p.ExpiresAt.Before(now) {
			continue // 已到期，自动解除
		}
		switch p.Type {
		case model.PunishBanUser:
			snap.Banned = true
		case model.PunishMute:
			if !snap.Muted {
				snap.Muted = true
				snap.MuteReason = p.Reason
				snap.MutePermanent = p.Permanent
				if p.ExpiresAt != nil {
					snap.MuteExpires = p.ExpiresAt.Format(time.RFC3339)
				}
			}
		case model.PunishForceReset:
			snap.ForceReset = true
		case model.PunishWarn:
			if p.AckAt == nil && snap.Warn == nil {
				w := p
				snap.Warn = &w
			}
		}
	}
	return snap
}

// Create 创建处罚（可叠加），写审计并失效快照。
func (s *PunishmentService) Create(actor *model.User, req PunishRequest) (*model.Punishment, string, error) {
	// 警告/禁言/封禁必须填理由
	switch req.Type {
	case model.PunishWarn, model.PunishMute, model.PunishBanUser, model.PunishBanIP:
		if req.Reason == "" {
			return nil, "", errs.New(errs.ErrValidation, fmt.Errorf("该处罚类型必须填写理由"))
		}
	}
	if !req.Permanent && req.ExpiresAt == nil {
		return nil, "", errs.New(errs.ErrValidation, fmt.Errorf("临时处罚必须指定到期时间"))
	}
	if req.Type != model.PunishBanIP && req.UserID == 0 {
		return nil, "", errs.New(errs.ErrValidation, fmt.Errorf("必须指定受处罚用户"))
	}
	// 强制改密的新用户名：格式与违禁词校验
	if req.Type == model.PunishForceReset && req.NewUsername != "" {
		if !validate.ValidUsername(req.NewUsername) {
			return nil, "", errs.New(errs.ErrInvalidUsername, nil)
		}
		if s.words != nil {
			if err := s.words.CheckUsername(req.NewUsername); err != nil {
				return nil, "", err
			}
		}
		var dup int64
		s.db.Model(&model.User{}).Where("username = ? AND id <> ?", req.NewUsername, req.UserID).Count(&dup)
		if dup > 0 {
			return nil, "", errs.New(errs.ErrUserExists, nil)
		}
	}

	now := time.Now()
	p := &model.Punishment{
		UserID:      req.UserID,
		Type:        req.Type,
		Reason:      req.Reason,
		OperatorID:  actorIDOf(actor),
		TargetIP:    req.TargetIP,
		Permanent:   req.Permanent,
		ExpiresAt:   req.ExpiresAt,
		NewUsername: req.NewUsername,
		CreatedAt:   now,
	}

	tempPwd := ""
	switch req.Type {
	case model.PunishBanUser:
		s.db.Model(&model.User{}).Where("id = ?", req.UserID).Update("status", model.UserStatusBanned)
	case model.PunishBanIP:
		s.db.Create(&model.IPBan{
			CIDR:       req.TargetIP,
			Reason:     req.Reason,
			OperatorID: actorIDOf(actor),
			Permanent:  req.Permanent,
			ExpiresAt:  req.ExpiresAt,
			CreatedAt:  now,
		})
		s.invalidateIPBans()
	case model.PunishForceReset:
		var err error
		tempPwd, err = s.forceReset(req)
		if err != nil {
			return nil, "", err
		}
	}
	if err := s.db.Create(p).Error; err != nil {
		return nil, "", errs.New(errs.CodeInternal, err)
	}
	s.Invalidate(req.UserID)
	var aid uint
	if actor != nil {
		aid = actor.ID
	}
	Audit(s.db, aid, "punishment.create", "punishment", uintToStr(p.ID),
		fmt.Sprintf("类型=%s 用户=%d 理由=%s", req.Type, req.UserID, req.Reason))
	return p, tempPwd, nil
}

// forceReset 强制修改用户名与密码：生成临时密码、踢下线、标记下次登录必须改密。
func (s *PunishmentService) forceReset(req PunishRequest) (string, error) {
	var u model.User
	if err := s.db.First(&u, req.UserID).Error; err != nil {
		return "", errs.New(errs.CodeNotFound, err)
	}
	tempPwd := randPassword()
	hash, err := bcrypt.GenerateFromPassword([]byte(tempPwd), bcrypt.DefaultCost)
	if err != nil {
		return "", errs.New(errs.CodeInternal, err)
	}
	updates := map[string]interface{}{
		"password_hash":   string(hash),
		"token_version":   gorm.Expr("token_version + 1"),
		"force_reset_pwd": true,
	}
	if req.NewUsername != "" {
		updates["username"] = req.NewUsername
	}
	if err := s.db.Model(&u).Updates(updates).Error; err != nil {
		return "", errs.New(errs.CodeInternal, err)
	}
	// 邮件通知（SMTP 未配置时降级为不发送）
	if s.m != nil && s.m.Configured() {
		body := fmt.Sprintf("你的账号已被管理员要求修改密码。\n新的临时密码：%s\n请登录后立即修改密码。", tempPwd)
		_ = s.m.Send(nil, u.Email, "【校园墙】账号密码已被重置", body)
	}
	return tempPwd, nil
}

// Revoke 撤销处罚：仅标记已撤销，不删除记录；强制改密撤销后不恢复原值。
func (s *PunishmentService) Revoke(actor *model.User, id uint) error {
	var p model.Punishment
	if err := s.db.First(&p, id).Error; err != nil {
		return errs.New(errs.CodeNotFound, err)
	}
	now := time.Now()
	p.Revoked = true
	p.RevokedAt = &now
	if err := s.db.Save(&p).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	// 封禁用户名：若没有其他生效封禁，恢复账号状态
	if p.Type == model.PunishBanUser {
		var cnt int64
		s.db.Model(&model.Punishment{}).
			Where("user_id = ? AND type = ? AND revoked = ? AND id != ? AND (permanent = ? OR expires_at > ?)",
				p.UserID, model.PunishBanUser, false, id, true, now).
			Count(&cnt)
		if cnt == 0 {
			s.db.Model(&model.User{}).Where("id = ?", p.UserID).Update("status", model.UserStatusActive)
		}
	}
	// IP 封禁：同步撤销对应 IPBan
	if p.Type == model.PunishBanIP && p.TargetIP != "" {
		s.db.Model(&model.IPBan{}).
			Where("cidr = ? AND revoked = ?", p.TargetIP, false).
			Updates(map[string]interface{}{"revoked": true, "revoked_at": now})
		s.invalidateIPBans()
	}
	s.Invalidate(p.UserID)
	var aid uint
	if actor != nil {
		aid = actor.ID
	}
	Audit(s.db, aid, "punishment.revoke", "punishment", uintToStr(id), "撤销处罚")
	return nil
}

// Ack 确认警告（确认后不再弹窗）。
func (s *PunishmentService) Ack(userID, id uint) error {
	var p model.Punishment
	if err := s.db.Where("id = ? AND user_id = ? AND type = ?", id, userID, model.PunishWarn).First(&p).Error; err != nil {
		return errs.New(errs.CodeNotFound, err)
	}
	now := time.Now()
	if err := s.db.Model(&p).Update("ack_at", now).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	s.Invalidate(userID)
	return nil
}

// ListMine 我的处罚记录（用户本人）。
func (s *PunishmentService) ListMine(userID uint) ([]model.Punishment, error) {
	var rows []model.Punishment
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return rows, nil
}

// ListForAdmin 后台处罚列表（可按用户筛选）。
func (s *PunishmentService) ListForAdmin(userID uint, page, pageSize int) ([]model.Punishment, int64, error) {
	q := s.db.Model(&model.Punishment{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	var total int64
	q.Count(&total)
	var rows []model.Punishment
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	return rows, total, nil
}

// ActiveIPBans 生效中的 IP 封禁（缓存 60 秒）。
func (s *PunishmentService) ActiveIPBans() []model.IPBan {
	const key = "ipban:active"
	if s.c != nil {
		if v, ok := s.c.Get(key); ok {
			var rows []model.IPBan
			if err := json.Unmarshal([]byte(v), &rows); err == nil {
				return rows
			}
		}
	}
	now := time.Now()
	var rows []model.IPBan
	s.db.Where("revoked = ? AND (permanent = ? OR expires_at > ?)", false, true, now).Find(&rows)
	if s.c != nil {
		if b, err := json.Marshal(rows); err == nil {
			s.c.Set(key, string(b), 60*time.Second)
		}
	}
	return rows
}

func (s *PunishmentService) invalidateIPBans() {
	if s.c != nil {
		s.c.Del("ipban:active")
	}
}

// ListIPBans IP 封禁列表。
func (s *PunishmentService) ListIPBans() ([]model.IPBan, error) {
	var rows []model.IPBan
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return rows, nil
}

// RevokeIPBan 撤销 IP 封禁。
func (s *PunishmentService) RevokeIPBan(actor *model.User, id uint) error {
	now := time.Now()
	if err := s.db.Model(&model.IPBan{}).Where("id = ?", id).
		Updates(map[string]interface{}{"revoked": true, "revoked_at": now}).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	s.invalidateIPBans()
	var aid uint
	if actor != nil {
		aid = actor.ID
	}
	Audit(s.db, aid, "ipban.revoke", "ipban", uintToStr(id), "撤销 IP 封禁")
	return nil
}

func actorIDOf(u *model.User) uint {
	if u == nil {
		return 0
	}
	return u.ID
}

func randPassword() string {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 10)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
