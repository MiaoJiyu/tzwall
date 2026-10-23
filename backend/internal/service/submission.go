package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/ai"
	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/config"
	"tzwall/backend/internal/mailer"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/moderation"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/storage"
)

// SubmissionService 投稿与标签业务。
type SubmissionService struct {
	db         *gorm.DB
	cfg        *config.Config
	storage    storage.Storage
	moderation moderation.Moderation
	store      *ConfigStore
	mailer     mailer.Mailer
	cache      cache.Cache
	words      *WordFilterService
	ai         *ai.Client
}

func NewSubmission(db *gorm.DB, cfg *config.Config, st storage.Storage, mod moderation.Moderation, store *ConfigStore, m mailer.Mailer, c cache.Cache) *SubmissionService {
	return &SubmissionService{db: db, cfg: cfg, storage: st, moderation: mod, store: store, mailer: m, cache: c}
}

// WithGovernance 注入词表与 AI 审核依赖（治理能力装配后调用）。
func (s *SubmissionService) WithGovernance(words *WordFilterService, aiClient *ai.Client) *SubmissionService {
	s.words = words
	s.ai = aiClient
	return s
}

type CreateSubmissionReq struct {
	Title          string   `json:"title"`
	Content        string   `json:"content"`
	TagIDs         []uint   `json:"tag_ids"`
	TagNames       []string `json:"tag_names"`
	ImageIDs       []uint   `json:"image_ids"`
	VideoIDs       []uint   `json:"video_ids"`
	Contact        string   `json:"contact"`
	ContactVisible string   `json:"contact_visible"`
	Anonymous      bool     `json:"anonymous"`
}

// Create 创建投稿。阶段一未接 COS 自动审核，全部转人工（pending）。
func (s *SubmissionService) Create(user *model.User, req CreateSubmissionReq) (*model.Submission, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if strings.TrimSpace(req.Content) == "" && len(req.ImageIDs) == 0 && len(req.VideoIDs) == 0 {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	maxTags := s.store.Int("tag_max_count", 5)
	if len(req.TagIDs)+len(req.TagNames) > maxTags {
		return nil, errs.New(errs.ErrTagLimit, nil)
	}
	maxImg := s.store.Int("image_max_count", 9)
	if len(req.ImageIDs) > maxImg {
		return nil, errs.New(errs.ErrImageTooLarge, nil)
	}
	maxVid := s.store.Int("video_max_count", 3)
	if len(req.VideoIDs) > maxVid {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if req.ContactVisible != model.ContactAll && req.ContactVisible != model.ContactLogin {
		req.ContactVisible = model.ContactLogin
	}

	// 标签：已审核的预选标签 + 自定义新增（待审核）
	tags := make([]*model.Tag, 0)
	if len(req.TagIDs) > 0 {
		var existing []model.Tag
		s.db.Where("id IN ? AND status = ?", req.TagIDs, model.TagApproved).Find(&existing)
		for i := range existing {
			t := existing[i]
			tags = append(tags, &t)
		}
		if len(existing) != len(req.TagIDs) {
			return nil, errs.New(errs.ErrTagPending, nil) // 含未审核标签
		}
	}
	for _, name := range req.TagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var t model.Tag
		// 去重：已存在则复用（若是待审核则不满足 approved）
		if err := s.db.Where("name = ?", name).First(&t).Error; err == nil {
			if t.Status != model.TagApproved {
				return nil, errs.New(errs.ErrTagPending, nil)
			}
			tags = append(tags, &t)
			continue
		}
		t = model.Tag{Name: name, Status: model.TagPending, CreatedBy: user.ID}
		s.db.Create(&t)
		tags = append(tags, &t)
	}

	sub := &model.Submission{
		UserID:         user.ID,
		Anonymous:      req.Anonymous,
		Title:          req.Title,
		Content:        req.Content,
		Tags:           tags,
		Contact:        req.Contact,
		ContactVisible: req.ContactVisible,
		Status:         model.StatusPending,
	}

	if err := s.db.Create(sub).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}

	status, auditStatus, note, auditComment := s.reviewContent(sub, req)
	if auditComment != "" {
		s.db.Model(sub).Update("audit_comment", auditComment)
		sub.AuditComment = auditComment
	}
	if status != sub.Status {
		s.db.Model(sub).Update("status", status)
		sub.Status = status
	}
	if note != "" {
		s.db.Model(sub).Update("admin_note", note)
		sub.AdminNote = note
	}
	s.db.Model(sub).Update("audit_status", auditStatus)

	// 关联已上传图片：命中违规图片立即驳回（含图片先被回调判违规、投稿后才创建的情况）；
	// 其余未通过审核的图片一律转人工。
	if len(req.ImageIDs) > 0 {
		s.db.Model(&model.SubmissionImage{}).Where("id IN ? AND submission_id IS NULL", req.ImageIDs).
			Update("submission_id", sub.ID)
		var statuses []string
		s.db.Model(&model.SubmissionImage{}).Where("submission_id = ?", sub.ID).Pluck("audit_status", &statuses)
		switch {
		case mediaHasBlock(statuses):
			note := "内容未通过审核：图片涉嫌违规"
			s.db.Model(sub).Updates(map[string]any{
				"status":       model.StatusRejected,
				"audit_status": model.ModerationBlock,
				"admin_note":   note,
			})
			sub.Status = model.StatusRejected
			sub.AuditStatus = model.ModerationBlock
			sub.AdminNote = note
		case sub.Status == model.StatusApproved && mediaNeedsManualReview(statuses):
			s.db.Model(sub).Update("status", model.StatusPending)
			sub.Status = model.StatusPending
		}
	}

	// 关联已上传视频：命中违规视频立即驳回；其余视频进入人工审核（pending）。
	if len(req.VideoIDs) > 0 {
		s.db.Model(&model.SubmissionVideo{}).Where("id IN ? AND submission_id IS NULL", req.VideoIDs).
			Update("submission_id", sub.ID)
		var statuses []string
		s.db.Model(&model.SubmissionVideo{}).Where("submission_id = ?", sub.ID).Pluck("audit_status", &statuses)
		switch {
		case mediaHasBlock(statuses):
			note := "内容未通过审核：视频涉嫌违规"
			s.db.Model(sub).Updates(map[string]any{
				"status":       model.StatusRejected,
				"audit_status": model.ModerationBlock,
				"admin_note":   note,
			})
			sub.Status = model.StatusRejected
			sub.AuditStatus = model.ModerationBlock
			sub.AdminNote = note
		case sub.Status == model.StatusApproved && mediaNeedsManualReview(statuses):
			s.db.Model(sub).Update("status", model.StatusPending)
			sub.Status = model.StatusPending
		}
	}

	// 回读关联，保证发布响应包含标签、图片与视频
	s.db.Preload("Tags").
		Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).
		Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).
		First(sub, sub.ID)
	return sub, nil
}

// reviewContent 内容审核状态机：
//   - 腾讯云（文本/媒体）审核拒绝 → 立即驳回稿件；
//   - AI 审核拒绝 → 进入人工审核流程（pending）；
//   - 腾讯云 + AI 审核均通过 → 立即放行（approved）。
//
// 返回值：投稿状态、机审状态、对作者可见的驳回原因、仅管理员可见的 AI 审核评论。
func (s *SubmissionService) reviewContent(sub *model.Submission, req CreateSubmissionReq) (model.SubmissionStatus, string, string, string) {
	text := strings.TrimSpace(req.Title + " " + req.Content)

	// 0) 词表预检：命中违禁词直接驳回，避免消耗审核额度（原因对作者可见）
	if s.words != nil && s.words.HasForbidden(text) {
		slog.Default().Info("投稿命中违禁词，直接驳回", "submission", sub.ID)
		return model.StatusRejected, model.ModerationBlock, "内容包含违禁词，请修改后重试", ""
	}

	// 1) 腾讯云数据万象文本审核
	if !s.moderation.Configured() {
		return model.StatusPending, model.ModerationReview, "", ""
	}
	v, err := s.moderation.SubmitText(context.Background(), fmt.Sprintf("sub-%d", sub.ID), text)
	switch {
	case err != nil:
		// 调用异常：直接驳回（按需求，不占用人工队列）
		slog.Default().Warn("腾讯云审核调用异常，驳回投稿", "submission", sub.ID, "err", err.Error())
		return model.StatusRejected, model.ModerationFailed, "内容审核服务暂时不可用，投稿未通过，请稍后重试", ""
	case v.Blocked:
		// 腾讯云审核拒绝 → 立即驳回
		return model.StatusRejected, model.ModerationBlock, "内容未通过审核：" + reasonOr(v.Reason, "涉嫌违规"), ""
	case !v.Pass:
		return model.StatusPending, model.ModerationReview, "", ""
	}

	// 2) AI 复审（云审核通过后才调用）。AI 的通过/拒绝原因作为“审核评论”仅管理员可见。
	if s.ai == nil || !s.ai.Configured() {
		return model.StatusPending, model.ModerationReview, "", ""
	}
	av, aerr := s.ai.Review(context.Background(), text)
	switch {
	case aerr != nil:
		// AI 异常：转人工，绝不自动放行
		slog.Default().Warn("AI 审核异常，转人工", "submission", sub.ID, "err", aerr.Error())
		s.recordAudit(fmt.Sprintf("sub-%d", sub.ID), "ai", model.ModerationFailed, "AI异常", aerr.Error())
		return model.StatusPending, model.ModerationReview, "", "AI 审核异常，转人工复核"
	case !av.Pass:
		// AI 审核拒绝 → 进入人工审核流程（不自动驳回）
		comment := "AI 审核不通过：" + reasonOr(av.Reason, "判定不适合公开")
		s.recordAudit(fmt.Sprintf("sub-%d", sub.ID), "ai", model.ModerationReview, "AI", av.Reason)
		return model.StatusPending, model.ModerationReview, "", comment
	}
	comment := "AI 审核通过：" + reasonOr(av.Reason, "内容合规")
	s.recordAudit(fmt.Sprintf("sub-%d", sub.ID), "ai", model.ModerationPass, "AI", av.Reason)
	return model.StatusApproved, model.ModerationPass, "", comment
}

// recordAudit 记录一次机审（腾讯云 / AI）结果，便于后台追溯驳回理由。
func (s *SubmissionService) recordAudit(bizID, kind, status, label, raw string) {
	if s.db == nil {
		return
	}
	if len(raw) > 2000 {
		raw = raw[:2000]
	}
	_ = s.db.Create(&model.ModerationTask{
		BizID:   bizID,
		Kind:    kind,
		Status:  status,
		Label:   label,
		RawResp: raw,
	}).Error
}

func reasonOr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return strings.TrimSpace(s)
}

// mediaHasBlock 是否存在明确违规(block)的媒体。
func mediaHasBlock(statuses []string) bool {
	for _, s := range statuses {
		if s == model.ModerationBlock {
			return true
		}
	}
	return false
}

// mediaNeedsManualReview 媒体是否需要转人工复核：仅「内容可疑(review)」需要。
// 审核失败(failed)、未审(pending/none)、通过(pass) 均不需要人工，
// 避免因机审基础设施异常（如文件过大、解码失败）误伤合规稿件。
func mediaNeedsManualReview(statuses []string) bool {
	for _, s := range statuses {
		if s == model.ModerationReview {
			return true
		}
	}
	return false
}

// Update 作者修改自己的稿件并重新提交审核。
// 允许状态：待审核/已通过/已驳回/需要修改/已撤回；已下架（机审违规）不可修改。
// 修改后进入待审核、清空历史审核意见并重置机审状态，保留修改历史。
func (s *SubmissionService) Update(user *model.User, id uint, req CreateSubmissionReq) (*model.Submission, error) {
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if sub.UserID != user.ID {
		return nil, errs.New(errs.CodeForbidden, nil)
	}
	if sub.AuditStatus == model.ModerationBlock {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("该稿件已被下架，无法修改"))
	}
	switch sub.Status {
	case model.StatusPending, model.StatusApproved, model.StatusRejected, model.StatusNeedEdit, model.StatusWithdrawn:
	default:
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("当前状态不可修改"))
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if strings.TrimSpace(req.Content) == "" && len(req.ImageIDs) == 0 && len(req.VideoIDs) == 0 {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	// 保留修改历史
	s.db.Create(&model.SubmissionRevision{
		SubmissionID: sub.ID, Title: sub.Title, Content: sub.Content, EditorID: user.ID, CreatedAt: time.Now(),
	})
	wasApproved := sub.Status == model.StatusApproved
	sub.Status = model.StatusPending
	sub.AdminNote = ""
	sub.AuditStatus = model.ModerationReview
	sub.RevisionCount++
	if wasApproved {
		// 已通过帖子被编辑：原版继续在主页展示并标注“审核中”，新内容暂存，通过后覆盖
		sub.PendingEdit = true
		sub.PendingTitle = req.Title
		sub.PendingContent = req.Content
	} else {
		sub.Title = req.Title
		sub.Content = req.Content
		sub.Contact = req.Contact
		sub.ContactVisible = req.ContactVisible
		sub.Anonymous = req.Anonymous
	}
	if err := s.db.Save(&sub).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	// 关联编辑时新上传的媒体（追加；已关联的不受影响）
	if len(req.ImageIDs) > 0 {
		s.db.Model(&model.SubmissionImage{}).Where("id IN ? AND submission_id IS NULL", req.ImageIDs).Update("submission_id", sub.ID)
	}
	if len(req.VideoIDs) > 0 {
		s.db.Model(&model.SubmissionVideo{}).Where("id IN ? AND submission_id IS NULL", req.VideoIDs).Update("submission_id", sub.ID)
	}
	// 回读关联，保证响应包含标签、图片与视频
	s.db.Preload("Tags").
		Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).
		Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).
		First(&sub, sub.ID)
	return &sub, nil
}

// Withdraw 作者撤回自己的稿件（仅待审核/已通过可撤回）。撤回后不在公开列表/详情展示，可后续修改重新提交。
func (s *SubmissionService) Withdraw(user *model.User, id uint) error {
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if sub.UserID != user.ID {
		return errs.New(errs.CodeForbidden, nil)
	}
	switch sub.Status {
	case model.StatusPending, model.StatusApproved:
	default:
		return errs.New(errs.ErrValidation, fmt.Errorf("当前状态不可撤回"))
	}
	return s.db.Model(&sub).Update("status", model.StatusWithdrawn).Error
}

// Delete 删除自己的投稿（软删），图片设为私有（不同步删除原图）。
func (s *SubmissionService) Delete(user *model.User, id uint) error {
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if sub.UserID != user.ID {
		return errs.New(errs.CodeForbidden, nil)
	}
	// 图片、视频设为私有
	var imgs []model.SubmissionImage
	s.db.Where("submission_id = ?", id).Find(&imgs)
	for _, im := range imgs {
		s.storage.SetPrivate(nil, im.StorageKey)
	}
	var vids []model.SubmissionVideo
	s.db.Where("submission_id = ?", id).Find(&vids)
	for _, vd := range vids {
		s.storage.SetPrivate(nil, vd.StorageKey)
	}
	s.db.Delete(&sub)
	return nil
}

// ListParams 列表查询参数。
type ListParams struct {
	Sort     string
	Tag      string
	Keyword  string
	Page     int
	PageSize int
}

// List 公开列表（仅 approved）。支持排序/标签/搜索/分页。
func (s *SubmissionService) List(p ListParams) ([]model.Submission, int64, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 || p.PageSize > 50 {
		p.PageSize = 20
	}
	// 公开列表：已通过 或 编辑待审核（保留原版展示并标注“审核中”）。
	q := s.db.Model(&model.Submission{}).Where("(submissions.status = ? OR submissions.pending_edit = ?)", model.StatusApproved, true)
	if p.Keyword != "" {
		kw := "%" + p.Keyword + "%"
		q = q.Where("submissions.title ILIKE ? OR submissions.content ILIKE ?", kw, kw)
	}
	if p.Tag != "" {
		q = q.Joins("JOIN submission_tags st ON st.submission_id = submissions.id").
			Joins("JOIN tags t ON t.id = st.tag_id").Where("t.name = ?", p.Tag)
	}
	var total int64
	q.Count(&total)

	q = q.Preload("Tags").Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("User").Offset((p.Page - 1) * p.PageSize).Limit(p.PageSize)
	if p.Sort == "hot" {
		gravity := s.store.Int("hot_decay_gravity", 2)
		q = q.Order(fmt.Sprintf(
			"(like_count - dislike_count + comment_count*2) / POWER(EXTRACT(EPOCH FROM (now()-submissions.created_at))/3600 + 2, %d) DESC", gravity))
	} else {
		q = q.Order("submissions.created_at DESC")
	}
	var subs []model.Submission
	if err := q.Find(&subs).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	return subs, total, nil
}

// ListMine 我的投稿（含所有状态）。
func (s *SubmissionService) ListMine(user *model.User) ([]model.Submission, error) {
	var subs []model.Submission
	if err := s.db.Where("user_id = ?", user.ID).Preload("User").Preload("Tags").Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).
		Order("created_at DESC").Find(&subs).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return subs, nil
}

// GetVisible 按可见性取详情：approved 公开；其余仅作者/管理员。
func (s *SubmissionService) GetVisible(id uint, viewer *model.User) (*model.Submission, error) {
	var sub model.Submission
	if err := s.db.Preload("Tags").Preload("Images", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("Videos", "audit_status <> ? AND is_private = ?", model.ModerationBlock, false).Preload("User").First(&sub, id).Error; err != nil {
		return nil, errs.New(errs.ErrValidation, nil)
	}
	if sub.Status == model.StatusApproved || sub.PendingEdit {
		return &sub, nil
	}
	if viewer != nil && (viewer.ID == sub.UserID || viewer.IsAdmin()) {
		return &sub, nil
	}
	return nil, errs.New(errs.ErrSubmissionInvisible, nil)
}

// ApprovedTags 已审核标签（供前端选择）。
func (s *SubmissionService) ApprovedTags() ([]model.Tag, error) {
	var tags []model.Tag
	if err := s.db.Where("status = ?", model.TagApproved).Order("name").Find(&tags).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return tags, nil
}

// Review 审核。
func (s *SubmissionService) Review(admin *model.User, id uint, status model.SubmissionStatus, note string) error {
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	updates := map[string]any{"status": status}
	if note != "" {
		updates["admin_note"] = note
	}
	if sub.PendingEdit {
		// 编辑待审核：通过则用新内容覆盖并清除暂存；否则丢弃本次编辑，原版继续展示。
		updates["pending_edit"] = false
		updates["pending_title"] = ""
		updates["pending_content"] = ""
		if status == model.StatusApproved {
			updates["title"] = sub.PendingTitle
			updates["content"] = sub.PendingContent
		} else {
			updates["status"] = model.StatusApproved // 保留原版
		}
	}
	if err := s.db.Model(&model.Submission{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	return nil
}

// Restore 恢复被撤下的投稿（重新公开并清空驳回说明）。
func (s *SubmissionService) Restore(admin *model.User, id uint) error {
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if err := s.db.Model(&sub).Updates(map[string]any{
		"status":     model.StatusApproved,
		"admin_note": "",
	}).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	return nil
}

// IncrView 浏览量自增（公开详情访问）。
func (s *SubmissionService) IncrView(id uint) {
	s.db.Model(&model.Submission{}).Where("id = ?", id).Update("view_count", gorm.Expr("view_count + 1"))
}

// IncrViewOnce 浏览量按访问者去重自增（同一用户/IP 在窗口期内仅计一次）。
func (s *SubmissionService) IncrViewOnce(id uint, viewerKey string) {
	if viewerKey != "" && s.cache != nil {
		ck := "viewed:sub:" + strconv.FormatUint(uint64(id), 10) + ":" + viewerKey
		if s.cache.Incr(ck, 24*time.Hour) > 1 {
			return
		}
	}
	s.db.Model(&model.Submission{}).Where("id = ?", id).Update("view_count", gorm.Expr("view_count + 1"))
}

// MyAttitude 当前用户对某投稿的态度（1 赞 / -1 踩 / 0 无）。
func (s *SubmissionService) MyAttitude(viewerID, id uint) int {
	if viewerID == 0 {
		return 0
	}
	var r model.Reaction
	if err := s.db.Where("user_id = ? AND target_type = ? AND target_id = ?", viewerID, "submission", id).First(&r).Error; err == nil {
		return r.Attitude
	}
	return 0
}

// MyAttitudes 批量取当前用户对一批投稿的态度。
func (s *SubmissionService) MyAttitudes(viewerID uint, ids []uint) map[uint]int {
	m := map[uint]int{}
	if viewerID == 0 || len(ids) == 0 {
		return m
	}
	var rs []model.Reaction
	s.db.Where("user_id = ? AND target_type = ? AND target_id IN ?", viewerID, "submission", ids).Find(&rs)
	for _, r := range rs {
		m[r.TargetID] = r.Attitude
	}
	return m
}
