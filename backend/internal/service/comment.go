package service

import (
	"strings"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/moderation"
	"tzwall/backend/internal/pkg/errs"
)

// CommentService 评论业务。
type CommentService struct {
	db         *gorm.DB
	store      *ConfigStore
	moderation moderation.Moderation
	words      *WordFilterService
}

func NewComment(db *gorm.DB, store *ConfigStore, mod moderation.Moderation) *CommentService {
	return &CommentService{db: db, store: store, moderation: mod}
}

// WithWordFilter 注入词表服务（评论内容校验）。
func (s *CommentService) WithWordFilter(w *WordFilterService) *CommentService {
	s.words = w
	return s
}

// Create 发表评论。
func (s *CommentService) Create(user *model.User, submissionID, parentID uint, content string) (*model.Comment, error) {
	content = strings.TrimSpace(content)
	maxLen := s.store.Int("comment_max_len", 500)
	if len([]rune(content)) == 0 || len([]rune(content)) > maxLen {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	// 违禁词拦截（常用词不阻断）
	if s.words != nil {
		if err := s.words.CheckContent(content); err != nil {
			return nil, err
		}
	}
	var sub model.Submission
	if err := s.db.First(&sub, submissionID).Error; err != nil {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if sub.Status != model.StatusApproved {
		return nil, errs.New(errs.ErrSubmissionInvisible, nil)
	}

	depth := 1
	if parentID != 0 {
		var parent model.Comment
		if err := s.db.First(&parent, parentID).Error; err != nil {
			return nil, errs.New(errs.ErrValidation, nil)
		}
		depth = s.commentDepth(parentID) + 1
	}
	maxDepth := s.store.Int("comment_max_depth", 3)
	if depth > maxDepth {
		return nil, errs.New(errs.ErrValidation, nil)
	}

	cm := &model.Comment{
		SubmissionID: submissionID,
		UserID:       user.ID,
		ParentID:     nilIfZero(parentID),
		Content:      content,
		AuditStatus:  "pass",
	}
	if s.moderation.Configured() {
		v, _ := s.moderation.SubmitText(nil, "comment", content)
		if !v.Pass || v.Blocked {
			cm.AuditStatus = "block"
		} else {
			cm.AuditStatus = "pass"
		}
	} else {
		cm.AuditStatus = "pass"
	}
	if err := s.db.Create(cm).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	// 计数
	s.db.Model(&sub).Update("comment_count", gorm.Expr("comment_count + 1"))
	return cm, nil
}

func (s *CommentService) commentDepth(id uint) int {
	d := 1
	cur := id
	for {
		var c model.Comment
		if err := s.db.Select("parent_id").First(&c, cur).Error; err != nil {
			break
		}
		if c.ParentID == nil {
			break
		}
		d++
		cur = *c.ParentID
	}
	return d
}

// List 返回评论树（内存组树）。已删除评论对普通用户隐藏正文，管理员可见原文。
func (s *CommentService) List(submissionID uint, isAdmin bool) ([]*model.Comment, error) {
	var all []model.Comment
	if err := s.db.Where("submission_id = ?", submissionID).Order("created_at ASC").Find(&all).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	byID := make(map[uint]*model.Comment, len(all))
	for i := range all {
		c := all[i]
		// 已删除评论对普通用户清空正文，避免接口泄露
		if c.Deleted && !isAdmin {
			c.Content = ""
		}
		byID[c.ID] = &c
	}
	roots := make([]*model.Comment, 0)
	for i := range all {
		c := byID[all[i].ID]
		// 附加作者名与称号
		var u model.User
		if err := s.db.Select("username", "title", "avatar").First(&u, c.UserID).Error; err == nil {
			c.Username = u.Username
			c.Title = u.Title
			c.Avatar = u.Avatar
		}
		if c.ParentID == nil {
			roots = append(roots, c)
		} else if parent, ok := byID[*c.ParentID]; ok {
			parent.Children = append(parent.Children, c)
		} else {
			roots = append(roots, c)
		}
	}
	return roots, nil
}

// Delete 删除自己的评论（软标记，保留正文）。
func (s *CommentService) Delete(user *model.User, id uint) error {
	var c model.Comment
	if err := s.db.First(&c, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if c.UserID != user.ID {
		return errs.New(errs.CodeForbidden, nil)
	}
	s.db.Model(&c).Update("deleted", true)
	return nil
}

// AdminRestore 管理员恢复被撤下的评论。
func (s *CommentService) AdminRestore(id uint) error {
	var c model.Comment
	if err := s.db.First(&c, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	return s.db.Model(&c).Update("deleted", false).Error
}

// Get 按 ID 取评论。
func (s *CommentService) Get(id uint) (*model.Comment, error) {
	var c model.Comment
	if err := s.db.First(&c, id).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, err)
	}
	return &c, nil
}

// AdminRemove 管理员撤下评论（软标记，保留正文供追溯）。
func (s *CommentService) AdminRemove(id uint) error {
	var c model.Comment
	if err := s.db.First(&c, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	return s.db.Model(&c).Update("deleted", true).Error
}

func nilIfZero(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}
