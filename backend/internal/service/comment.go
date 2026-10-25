package service

import (
	"fmt"
	"strings"
	"time"

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
// viewerID 为当前查看者（0 表示游客），用于标注每条评论的点赞状态 my_liked。
func (s *CommentService) List(submissionID, viewerID uint, isAdmin bool) ([]*model.Comment, error) {
	var all []model.Comment
	if err := s.db.Where("submission_id = ?", submissionID).Order("created_at ASC").Find(&all).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	// 当前查看者的点赞状态（批量一次查询，避免 N+1）
	if viewerID > 0 && len(all) > 0 {
		ids := make([]uint, 0, len(all))
		for i := range all {
			ids = append(ids, all[i].ID)
		}
		var liked []uint
		s.db.Model(&model.Reaction{}).
			Where("user_id = ? AND target_type = 'comment' AND attitude = 1 AND target_id IN ?", viewerID, ids).
			Pluck("target_id", &liked)
		likedSet := make(map[uint]bool, len(liked))
		for _, id := range liked {
			likedSet[id] = true
		}
		for i := range all {
			if likedSet[all[i].ID] {
				all[i].MyLiked = true
			}
		}
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
		if err := s.db.Select("username", "nickname", "title", "avatar").First(&u, c.UserID).Error; err == nil {
			c.Username = u.DisplayName() // 展示名：昵称优先
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

// AdminRestore 管理员恢复被撤下的评论（记录操作日志）。
func (s *CommentService) AdminRestore(operatorID, id uint) error {
	var c model.Comment
	if err := s.db.First(&c, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if err := s.db.Model(&c).Update("deleted", false).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	Audit(s.db, operatorID, "comment.restore", "comment", fmt.Sprintf("%d", id), "恢复评论")
	return nil
}

// Get 按 ID 取评论。
func (s *CommentService) Get(id uint) (*model.Comment, error) {
	var c model.Comment
	if err := s.db.First(&c, id).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, err)
	}
	return &c, nil
}

// AdminRemove 管理员撤下评论（软标记，保留正文供追溯），并记录操作日志。
func (s *CommentService) AdminRemove(operatorID, id uint) error {
	var c model.Comment
	if err := s.db.First(&c, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if err := s.db.Model(&c).Update("deleted", true).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	Audit(s.db, operatorID, "comment.remove", "comment", fmt.Sprintf("%d", id), "撤下评论")
	return nil
}

// ToggleLike 点赞/取消点赞评论（幂等切换：同一用户重复提交只会切换一次）。
// 返回最新点赞状态与点赞总数，供前端实时刷新。
func (s *CommentService) ToggleLike(userID, commentID uint) (bool, int, error) {
	var c model.Comment
	if err := s.db.First(&c, commentID).Error; err != nil {
		return false, 0, errs.New(errs.CodeNotFound, nil)
	}
	if c.Deleted {
		return false, 0, errs.New(errs.ErrValidation, fmt.Errorf("该评论已删除，无法点赞"))
	}
	var existing model.Reaction
	err := s.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, "comment", commentID).First(&existing).Error
	has := err == nil
	liked := false
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if has && existing.Attitude == 1 {
			// 已点赞 → 取消
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			return tx.Model(&model.Comment{}).Where("id = ?", commentID).
				Update("like_count", gorm.Expr("GREATEST(like_count - 1, 0)")).Error
		}
		if has {
			existing.Attitude = 1
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Create(&model.Reaction{UserID: userID, TargetType: "comment", TargetID: commentID, Attitude: 1}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Comment{}).Where("id = ?", commentID).
			Update("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
			return err
		}
		liked = true
		return nil
	}); err != nil {
		return false, 0, errs.New(errs.CodeInternal, err)
	}
	var fresh model.Comment
	if err := s.db.Select("like_count").First(&fresh, commentID).Error; err != nil {
		return liked, c.LikeCount, nil
	}
	return liked, fresh.LikeCount, nil
}

// AdminCommentRow 后台评论管理列表行。
type AdminCommentRow struct {
	ID              uint      `json:"id"`
	SubmissionID    uint      `json:"submission_id"`
	SubmissionTitle string    `json:"submission_title"`
	UserID          uint      `json:"user_id"`
	Author          string    `json:"author"`
	ParentID        *uint     `json:"parent_id"`
	Content         string    `json:"content"`
	Deleted         bool      `json:"deleted"`
	AuditStatus     string    `json:"audit_status"`
	LikeCount       int       `json:"like_count"`
	CreatedAt       time.Time `json:"created_at"`
}

// AdminCommentQuery 后台评论管理查询条件。
type AdminCommentQuery struct {
	Keyword      string // 评论内容 / 投稿标题
	Status       string // all/normal/deleted/blocked
	SubmissionID uint   // 所属投稿
	Author       string // 作者（用户名/昵称）
	Page         int
	PageSize     int
}

// AdminList 后台评论管理列表：关键词/状态/投稿/作者筛选 + 分页。
func (s *CommentService) AdminList(q AdminCommentQuery) ([]AdminCommentRow, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	base := func() *gorm.DB {
		d := s.db.Table("comments c").
			Select("c.id, c.submission_id, COALESCE(s.title, '') AS submission_title, c.user_id, "+
				"COALESCE(NULLIF(u.nickname, ''), u.username) AS author, c.parent_id, c.content, "+
				"c.deleted, c.audit_status, c.like_count, c.created_at").
			Joins("LEFT JOIN submissions s ON s.id = c.submission_id").
			Joins("LEFT JOIN users u ON u.id = c.user_id")
		if kw := strings.TrimSpace(q.Keyword); kw != "" {
			like := "%" + kw + "%"
			d = d.Where("c.content LIKE ? OR s.title LIKE ?", like, like)
		}
		if au := strings.TrimSpace(q.Author); au != "" {
			like := "%" + au + "%"
			d = d.Where("u.username LIKE ? OR u.nickname LIKE ?", like, like)
		}
		if q.SubmissionID > 0 {
			d = d.Where("c.submission_id = ?", q.SubmissionID)
		}
		switch strings.TrimSpace(q.Status) {
		case "normal": // 正常展示中
			d = d.Where("c.deleted = ? AND c.audit_status <> ?", false, "block")
		case "deleted": // 已撤回
			d = d.Where("c.deleted = ?", true)
		case "blocked": // 已屏蔽（审核拦截）
			d = d.Where("c.audit_status = ?", "block")
		}
		return d
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	rows := make([]AdminCommentRow, 0, q.PageSize)
	if err := base().Order("c.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	return rows, total, nil
}

func nilIfZero(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}
