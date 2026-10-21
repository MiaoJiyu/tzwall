package service

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// 举报处理结果
const (
	ReportResultDismiss = "dismiss" // 无明显违规
	ReportResultRemove  = "remove"  // 撤下内容
)

// ResolveRequest 举报处理请求。
type ResolveRequest struct {
	Result       string // dismiss | remove
	Reason       string // 对举报人展示的处理理由
	Note         string // 内部备注
	PunishType   model.PunishmentType
	PunishTarget string // reporter | author
	PunishReason string
	Permanent    bool
	ExpiresAt    *time.Time
}

// ReportService 举报：创建、频率与唯一性、处理结果。
type ReportService struct {
	db    *gorm.DB
	c     cache.Cache
	store *ConfigStore
	pun   *PunishmentService
}

func NewReport(db *gorm.DB, c cache.Cache, store *ConfigStore, pun *PunishmentService) *ReportService {
	return &ReportService{db: db, c: c, store: store, pun: pun}
}

// Create 提交举报：禁言禁止、频率限制、同一对象仅一次。
func (s *ReportService) Create(u *model.User, targetType string, targetID uint, reason string) error {
	if u == nil {
		return errs.New(errs.CodeUnauthorized, nil)
	}
	if targetType != "submission" && targetType != "comment" {
		return errs.New(errs.ErrValidation, fmt.Errorf("举报对象类型不合法"))
	}
	// 禁言即禁止举报
	if s.pun != nil && s.pun.Snapshot(u.ID).Muted {
		return errs.New(errs.ErrMuted, nil)
	}
	maxLen := s.store.Int("report_reason_max_len", 200)
	if reason == "" {
		return errs.New(errs.ErrValidation, fmt.Errorf("举报理由必填"))
	}
	if maxLen > 0 && len([]rune(reason)) > maxLen {
		return errs.New(errs.ErrValidation, fmt.Errorf("举报理由超出长度限制"))
	}
	// 频率限制（每用户每小时/每日）
	if s.c != nil {
		hourLimit := int64(s.store.Int("report_hour_limit", 5))
		dayLimit := int64(s.store.Int("report_day_limit", 20))
		if n := s.c.Incr(fmt.Sprintf("rp:h:%d", u.ID), time.Hour); hourLimit > 0 && n > hourLimit {
			return errs.New(errs.CodeTooMany, nil)
		}
		if n := s.c.Incr(fmt.Sprintf("rp:d:%d", u.ID), 24*time.Hour); dayLimit > 0 && n > dayLimit {
			return errs.New(errs.CodeTooMany, nil)
		}
	}
	// 唯一性（应用层校验 + 数据库唯一索引兜底）
	var cnt int64
	s.db.Model(&model.Report{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", u.ID, targetType, targetID).
		Count(&cnt)
	if cnt > 0 {
		return errs.New(errs.ErrAlreadyReported, nil)
	}
	r := &model.Report{
		UserID:     u.ID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	if err := s.db.Create(r).Error; err != nil {
		return errs.New(errs.ErrAlreadyReported, err)
	}
	return nil
}

// Get 按 ID 取举报详情。
func (s *ReportService) Get(id uint) (*model.Report, error) {
	var r model.Report
	if err := s.db.First(&r, id).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, err)
	}
	return &r, nil
}

// Mine 我的举报（含处理结果）。
func (s *ReportService) Mine(userID uint) ([]model.Report, error) {
	var rows []model.Report
	if err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return rows, nil
}

// ListForAdmin 后台举报列表。
func (s *ReportService) ListForAdmin(status, targetType string, page, pageSize int) ([]model.Report, int64, error) {
	q := s.db.Model(&model.Report{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if targetType != "" {
		q = q.Where("target_type = ?", targetType)
	}
	var total int64
	q.Count(&total)
	var rows []model.Report
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	return rows, total, nil
}

// targetAuthor 取被举报内容的作者。
func (s *ReportService) targetAuthor(r *model.Report) uint {
	if r.TargetType == "submission" {
		var sub model.Submission
		if s.db.Select("user_id").First(&sub, r.TargetID).Error == nil {
			return sub.UserID
		}
		return 0
	}
	var cm model.Comment
	if s.db.Select("user_id").First(&cm, r.TargetID).Error == nil {
		return cm.UserID
	}
	return 0
}

// Resolve 处理举报：撤下内容 / 判定无违规 / 处罚相关用户。
func (s *ReportService) Resolve(actor *model.User, id uint, req ResolveRequest) error {
	var r model.Report
	if err := s.db.First(&r, id).Error; err != nil {
		return errs.New(errs.CodeNotFound, err)
	}
	// 允许对已处理的举报再次审核/修改（后台可复核纠错）
	// 撤下内容与驳回都必须填写理由（理由对举报人展示）
	if req.Reason == "" {
		return errs.New(errs.ErrValidation, fmt.Errorf("处理理由必填"))
	}

	if req.Result == ReportResultRemove {
		if r.TargetType == "submission" {
			s.db.Model(&model.Submission{}).Where("id = ?", r.TargetID).
				Update("status", model.StatusRejected)
		} else {
			s.db.Model(&model.Comment{}).Where("id = ?", r.TargetID).
				Updates(map[string]interface{}{"deleted": true})
		}
	} else if r.Result == ReportResultRemove {
		// 复核：由“撤下”改为“未违规”，恢复此前被本举报撤下的内容
		if r.TargetType == "submission" {
			s.db.Model(&model.Submission{}).
				Where("id = ? AND status = ?", r.TargetID, model.StatusRejected).
				Update("status", model.StatusApproved)
		} else {
			s.db.Model(&model.Comment{}).Where("id = ?", r.TargetID).
				Update("deleted", false)
		}
	}

	// 处罚（处罚用户不会自动撤下内容）
	if req.PunishType != "" {
		target := uint(0)
		switch req.PunishTarget {
		case "reporter":
			target = r.UserID
		case "author":
			target = s.targetAuthor(&r)
		}
		if target == 0 {
			return errs.New(errs.ErrValidation, fmt.Errorf("处罚对象无效"))
		}
		reason := req.PunishReason
		if reason == "" {
			reason = req.Reason
		}
		if _, _, err := s.pun.Create(actor, PunishRequest{
			UserID:    target,
			Type:      req.PunishType,
			Reason:    reason,
			Permanent: req.Permanent,
			ExpiresAt: req.ExpiresAt,
		}); err != nil {
			return err
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":        "resolved",
		"result":        req.Result,
		"result_reason": req.Reason,
		"admin_note":    req.Note,
	}
	if actor != nil {
		updates["handled_by"] = actor.ID
	}
	if err := s.db.Model(&r).Updates(updates).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	_ = now
	var aid uint
	if actor != nil {
		aid = actor.ID
	}
	Audit(s.db, aid, "report.resolve", "report", uintToStr(id),
		fmt.Sprintf("结果=%s 理由=%s", req.Result, req.Reason))
	return nil
}
