package service

import (
	"strings"
	"time"

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

// PublicPost 公开主页展示的历史发帖（仅非匿名已通过）。
type PublicPost struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// PublicProfile 他人可见的公开主页数据（按用户自选字段）。
type PublicProfile struct {
	ID       uint            `json:"id"`
	Username string          `json:"username"`
	Nickname string          `json:"nickname,omitempty"`
	Avatar   string          `json:"avatar,omitempty"`
	Title    string          `json:"title,omitempty"`
	Fields   map[string]string `json:"fields"`
	Posts    []PublicPost    `json:"posts"`
}

// GetPublic 取用户公开主页；未开启公开返回 403。
func (s *UserService) GetPublic(id uint) (*PublicProfile, error) {
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, nil)
	}
	if !u.ProfilePublic {
		return nil, errs.New(errs.CodeForbidden, nil)
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
		Order("created_at DESC").Limit(50).Find(&subs)
	posts := make([]PublicPost, 0, len(subs))
	for _, sub := range subs {
		posts = append(posts, PublicPost{ID: sub.ID, Title: sub.Title, CreatedAt: sub.CreatedAt})
	}
	return &PublicProfile{
		ID: u.ID, Username: u.Username, Nickname: u.Nickname, Avatar: u.Avatar, Title: u.Title,
		Fields: fields, Posts: posts,
	}, nil
}
