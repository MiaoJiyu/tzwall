package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// AdminService 后台管理业务（阶段一：用户、审核、标签）。
type AdminService struct {
	db    *gorm.DB
	store *ConfigStore
}

func NewAdmin(db *gorm.DB, store *ConfigStore) *AdminService {
	return &AdminService{db: db, store: store}
}

// ModerationByBiz 批量获取投稿机审（腾讯云 / AI）任务，按 biz_id 分组，
// 供后台审核页展示“为何被机审驳回”的判定依据。
func (s *AdminService) ModerationByBiz(bizIDs []string) (map[string][]model.ModerationTask, error) {
	m := map[string][]model.ModerationTask{}
	if len(bizIDs) == 0 {
		return m, nil
	}
	var tasks []model.ModerationTask
	if err := s.db.Where("biz_id IN ?", bizIDs).Order("id").Find(&tasks).Error; err != nil {
		return nil, err
	}
	for i := range tasks {
		m[tasks[i].BizID] = append(m[tasks[i].BizID], tasks[i])
	}
	return m, nil
}

// ListUsers 用户检索。普通管理员仅看普通用户；超管看全部。
func (s *AdminService) ListUsers(actor *model.User, keyword string, page, pageSize int) ([]model.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	q := s.db.Model(&model.User{})
	if !actor.IsSuperAdmin() {
		q = q.Where("role = ?", model.RoleUser)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		// 支持用户名 / 邮箱 / 真实姓名 / 用户 ID 搜索
		q = q.Where("username ILIKE ? OR email ILIKE ? OR real_name ILIKE ? OR id::text ILIKE ?", kw, kw, kw, kw)
	}
	var total int64
	q.Count(&total)
	var users []model.User
	q.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&users)
	return users, total, nil
}

// SetRole 变更角色。仅超管；不可授予/降级超级管理员（保持手动）。
func (s *AdminService) SetRole(actor *model.User, targetID uint, role model.Role) error {
	if !actor.IsSuperAdmin() {
		return errs.New(errs.CodeForbidden, nil)
	}
	if role == model.RoleSuperAdmin {
		return errs.New(errs.ErrValidation, nil)
	}
	var target model.User
	if err := s.db.First(&target, targetID).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if target.IsSuperAdmin() {
		return errs.New(errs.ErrValidation, nil)
	}
	s.db.Model(&target).Update("role", role)
	Audit(s.db, actor.ID, "set_role", "user", itoa(targetID), "role="+string(role))
	return nil
}

// PendingTags 待审核标签。
func (s *AdminService) PendingTags() ([]model.Tag, error) {
	var tags []model.Tag
	if err := s.db.Where("status = ?", model.TagPending).Order("created_at ASC").Find(&tags).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return tags, nil
}

// ApproveTag 审核标签。
func (s *AdminService) ApproveTag(actor *model.User, tagID uint, approve bool) error {
	var t model.Tag
	if err := s.db.First(&t, tagID).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	status := model.TagRejected
	if approve {
		status = model.TagApproved
	}
	s.db.Model(&t).Update("status", status)
	Audit(s.db, actor.ID, "review_tag", "tag", itoa(tagID), "approve="+btoa(approve))
	return nil
}

// MergeTag 合并去重：from -> to，from 标记 rejected。
func (s *AdminService) MergeTag(actor *model.User, fromID, toID uint) error {
	if fromID == toID {
		return errs.New(errs.ErrValidation, nil)
	}
	var to model.Tag
	if err := s.db.First(&to, toID).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	// 投稿标签关联转移
	s.db.Exec("UPDATE submission_tags SET tag_id = ? WHERE tag_id = ?", toID, fromID)
	s.db.Model(&model.Tag{}).Where("id = ?", fromID).Updates(map[string]any{
		"status":    model.TagRejected,
		"merged_to": toID,
	})
	Audit(s.db, actor.ID, "merge_tag", "tag", itoa(fromID), "to="+itoa(toID))
	return nil
}

// AllTags 全部标签（含各状态，支持关键词搜索）。
func (s *AdminService) AllTags(keyword string) ([]model.Tag, error) {
	q := s.db.Model(&model.Tag{})
	if strings.TrimSpace(keyword) != "" {
		q = q.Where("name ILIKE ?", "%"+strings.TrimSpace(keyword)+"%")
	}
	var tags []model.Tag
	if err := q.Order("created_at DESC").Find(&tags).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return tags, nil
}

// CreateTag 手动新增标签（直接为已审核状态）。
func (s *AdminService) CreateTag(actor *model.User, name string) (*model.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("标签名不能为空且不超过 64 字"))
	}
	var cnt int64
	s.db.Model(&model.Tag{}).Where("name = ?", name).Count(&cnt)
	if cnt > 0 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("标签已存在"))
	}
	t := &model.Tag{Name: name, Status: model.TagApproved, CreatedBy: actor.ID, CreatedAt: time.Now()}
	if err := s.db.Create(t).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	Audit(s.db, actor.ID, "create_tag", "tag", itoa(t.ID), "name="+name)
	return t, nil
}

// UpdateTag 修改标签名。
func (s *AdminService) UpdateTag(actor *model.User, id uint, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 {
		return errs.New(errs.ErrValidation, fmt.Errorf("标签名不能为空且不超过 64 字"))
	}
	var t model.Tag
	if err := s.db.First(&t, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	var cnt int64
	s.db.Model(&model.Tag{}).Where("name = ? AND id <> ?", name, id).Count(&cnt)
	if cnt > 0 {
		return errs.New(errs.ErrValidation, fmt.Errorf("标签名已存在"))
	}
	if err := s.db.Model(&t).Update("name", name).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	Audit(s.db, actor.ID, "update_tag", "tag", itoa(id), "name="+name)
	return nil
}

// DeleteTag 删除标签（解除投稿关联后删除）。
func (s *AdminService) DeleteTag(actor *model.User, id uint) error {
	var t model.Tag
	if err := s.db.First(&t, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	s.db.Exec("DELETE FROM submission_tags WHERE tag_id = ?", id)
	if err := s.db.Delete(&model.Tag{}, id).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	Audit(s.db, actor.ID, "delete_tag", "tag", itoa(id), "name="+t.Name)
	return nil
}

// PendingSubmissions 待审核投稿。
func (s *AdminService) PendingSubmissions(page, pageSize int) ([]model.Submission, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	q := s.db.Model(&model.Submission{}).Where("status = ?", model.StatusPending)
	var total int64
	q.Count(&total)
	var subs []model.Submission
	q.Preload("Tags").Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("User").
		Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at ASC").Find(&subs)
	return subs, total, nil
}

// AllSubmissions 全部稿件（支持状态筛选与关键词搜索）。
func (s *AdminService) AllSubmissions(page, pageSize int, status, keyword string) ([]model.Submission, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	q := s.db.Model(&model.Submission{})
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("title ILIKE ? OR content ILIKE ?", kw, kw)
	}
	var total int64
	q.Count(&total)
	var subs []model.Submission
	q.Preload("Tags").Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("User").
		Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&subs)
	return subs, total, nil
}

// Dashboard 概览统计。
func (s *AdminService) Dashboard() map[string]int64 {
	out := map[string]int64{}
	var users, subs, pendSub, pendTag, cmts, today int64
	s.db.Model(&model.User{}).Count(&users)
	s.db.Model(&model.Submission{}).Count(&subs)
	s.db.Model(&model.Submission{}).Where("status = ?", model.StatusPending).Count(&pendSub)
	s.db.Model(&model.Tag{}).Where("status = ?", model.TagPending).Count(&pendTag)
	s.db.Model(&model.Comment{}).Count(&cmts)
	t := time.Now().Format("2006-01-02")
	s.db.Model(&model.Submission{}).Where("created_at >= ?", t+" 00:00:00").Count(&today)
	out["users"] = users
	out["submissions"] = subs
	out["pending_submissions"] = pendSub
	out["pending_tags"] = pendTag
	out["comments"] = cmts
	out["today_submissions"] = today
	return out
}

func itoa(u uint) string {
	return uintToStr(u)
}

func btoa(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
