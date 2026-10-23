package service

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// AnnouncementService 校园公告。
type AnnouncementService struct {
	db *gorm.DB
}

func NewAnnouncement(db *gorm.DB) *AnnouncementService {
	return &AnnouncementService{db: db}
}

// AnnouncementView 公开列表视图，按查看者标注确认状态。
type AnnouncementView struct {
	ID             uint      `json:"id"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	RequireConfirm bool      `json:"require_confirm"`
	Pinned         bool      `json:"pinned"`
	CreatedAt      time.Time `json:"created_at"`
	Confirmed      bool      `json:"confirmed"` // 无需确认=true；需确认且该用户已确认=true
}

// AdminList 后台列表（含未上架）。
func (s *AnnouncementService) AdminList() ([]model.Announcement, error) {
	var list []model.Announcement
	if err := s.db.Order("pinned DESC, id DESC").Find(&list).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return list, nil
}

// Create 发布公告。
func (s *AnnouncementService) Create(actor *model.User, title, content string, requireConfirm, pinned, active bool) (*model.Announcement, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if len([]rune(title)) > 200 {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	a := &model.Announcement{
		Title: title, Content: content,
		RequireConfirm: requireConfirm, Pinned: pinned, Active: active,
		CreatedBy: actor.ID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.db.Create(a).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return a, nil
}

// Update 修改公告。
func (s *AnnouncementService) Update(actor *model.User, id uint, title, content string, requireConfirm, pinned, active bool) error {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" {
		return errs.New(errs.ErrValidation, nil)
	}
	var a model.Announcement
	if err := s.db.First(&a, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	updates := map[string]any{
		"title": title, "content": content,
		"require_confirm": requireConfirm, "pinned": pinned, "active": active,
		"updated_at": time.Now(),
	}
	return s.db.Model(&a).Updates(updates).Error
}

// Delete 删除公告（连同确认记录）。
func (s *AnnouncementService) Delete(actor *model.User, id uint) error {
	if err := s.db.Delete(&model.Announcement{}, id).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	s.db.Where("announcement_id = ?", id).Delete(&model.AnnouncementConfirm{})
	return nil
}

// ListPublic 公开列表（仅上架），按查看者标注确认状态。
func (s *AnnouncementService) ListPublic(viewerID uint) ([]AnnouncementView, error) {
	var anns []model.Announcement
	if err := s.db.Where("active = ?", true).Order("pinned DESC, id DESC").Find(&anns).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	views := make([]AnnouncementView, 0, len(anns))
	if len(anns) == 0 {
		return views, nil
	}
	confirmed := map[uint]bool{}
	if viewerID > 0 {
		ids := make([]uint, len(anns))
		for i, a := range anns {
			ids[i] = a.ID
		}
		var confs []model.AnnouncementConfirm
		s.db.Where("user_id = ? AND announcement_id IN ?", viewerID, ids).Find(&confs)
		for _, c := range confs {
			confirmed[c.AnnouncementID] = true
		}
	}
	for _, a := range anns {
		views = append(views, AnnouncementView{
			ID: a.ID, Title: a.Title, Content: a.Content,
			RequireConfirm: a.RequireConfirm, Pinned: a.Pinned, CreatedAt: a.CreatedAt,
			Confirmed: !a.RequireConfirm || confirmed[a.ID],
		})
	}
	return views, nil
}

// Confirm 用户确认公告（幂等）。
func (s *AnnouncementService) Confirm(userID, id uint) error {
	if userID == 0 {
		return errs.New(errs.CodeUnauthorized, nil)
	}
	var a model.Announcement
	if err := s.db.First(&a, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	var c model.AnnouncementConfirm
	if err := s.db.Where("announcement_id = ? AND user_id = ?", id, userID).First(&c).Error; err == nil {
		return nil // 已确认
	}
	return s.db.Create(&model.AnnouncementConfirm{AnnouncementID: id, UserID: userID, CreatedAt: time.Now()}).Error
}
