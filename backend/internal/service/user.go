package service

import (
	"strings"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// UserService 用户公开资料。
type UserService struct {
	db *gorm.DB
}

func NewUser(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// PublicProfile 他人可见的公开主页资料（按用户自选字段）。
type PublicProfile struct {
	ID       uint              `json:"id"`
	Username string            `json:"username"`
	Nickname string            `json:"nickname,omitempty"`
	Avatar   string            `json:"avatar,omitempty"`
	Title    string            `json:"title,omitempty"`
	Fields   map[string]string `json:"fields"`
}

// GetPublic 取用户公开主页；未开启公开返回 403。
// 同时返回其公开投稿（已预加载作者/标签/图片/视频），由 handler 以统一 submissionView 组装，
// 确保点赞/评论/浏览等计数与站内卡片一致（避免字段缺失导致前端 NaN）。
func (s *UserService) GetPublic(id uint) (*PublicProfile, []model.Submission, error) {
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, nil, errs.New(errs.CodeNotFound, nil)
	}
	if !u.ProfilePublic {
		return nil, nil, errs.New(errs.CodeForbidden, nil)
	}
	fields := map[string]string{}
	for _, f := range strings.Split(u.PublicFields, ",") {
		switch strings.TrimSpace(f) {
		case "wechat":
			if u.Wechat != "" {
				fields["wechat"] = u.Wechat
			}
		case "qq":
			if u.QQ != "" {
				fields["qq"] = u.QQ
			}
		case "phone":
			if u.Phone != "" {
				fields["phone"] = u.Phone
			}
		case "school":
			if u.School != "" {
				fields["school"] = u.School
			}
		case "real_name":
			if u.RealName != "" {
				fields["real_name"] = u.RealName
			}
		}
	}
	var subs []model.Submission
	s.db.Where("user_id = ? AND status = ? AND anonymous = ?", id, model.StatusApproved, false).
		Preload("User").Preload("Tags").
		Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).
		Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).
		Order("created_at DESC").Limit(50).Find(&subs)
	return &PublicProfile{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname, Avatar: u.Avatar, Title: u.Title,
		Fields: fields,
	}, subs, nil
}
