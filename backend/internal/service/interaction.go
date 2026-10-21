package service

import (
	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// InteractionService 点赞/点踩。
type InteractionService struct {
	db *gorm.DB
}

func NewInteraction(db *gorm.DB) *InteractionService {
	return &InteractionService{db: db}
}

// React 唯一态度：attitude=1 赞，-1 踩，0 取消。
func (s *InteractionService) React(user *model.User, targetType string, targetID uint, attitude int) error {
	if targetType != "submission" && targetType != "comment" {
		return errs.New(errs.ErrValidation, nil)
	}
	if attitude != 1 && attitude != -1 && attitude != 0 {
		return errs.New(errs.ErrValidation, nil)
	}

	var existing model.Reaction
	err := s.db.Where("user_id = ? AND target_type = ? AND target_id = ?", user.ID, targetType, targetID).First(&existing).Error
	has := err == nil

	if !has {
		if attitude == 0 {
			return nil
		}
		s.db.Create(&model.Reaction{UserID: user.ID, TargetType: targetType, TargetID: targetID, Attitude: attitude})
		s.applyDelta(targetType, targetID, attitude, 1)
		return nil
	}

	if attitude == 0 || existing.Attitude == attitude {
		// 取消
		s.db.Delete(&existing)
		s.applyDelta(targetType, targetID, existing.Attitude, -1)
		return nil
	}
	// 异态度切换
	s.applyDelta(targetType, targetID, existing.Attitude, -1)
	existing.Attitude = attitude
	s.db.Save(&existing)
	s.applyDelta(targetType, targetID, attitude, 1)
	return nil
}

func (s *InteractionService) applyDelta(targetType string, targetID uint, attitude int, delta int) {
	if targetType == "submission" {
		if attitude == 1 {
			s.db.Model(&model.Submission{}).Where("id = ?", targetID).Update("like_count", gorm.Expr("like_count + ?", delta))
		} else {
			s.db.Model(&model.Submission{}).Where("id = ?", targetID).Update("dislike_count", gorm.Expr("dislike_count + ?", delta))
		}
	} else {
		if attitude == 1 {
			s.db.Model(&model.Comment{}).Where("id = ?", targetID).Update("like_count", gorm.Expr("like_count + ?", delta))
		}
	}
}
