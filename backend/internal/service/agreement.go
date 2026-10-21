package service

import (
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// AgreementService 协议管理：当前生效内容 + 历史版本（发布与回滚）。
type AgreementService struct {
	db *gorm.DB
}

func NewAgreement(db *gorm.DB) *AgreementService {
	return &AgreementService{db: db}
}

// 协议键
const (
	AgreementKeyUser    = "user"
	AgreementKeyPrivacy = "privacy"
	AgreementKeyData    = "data"
)

// Current 返回当前生效的协议（三条）。
func (s *AgreementService) Current() ([]model.Agreement, error) {
	var list []model.Agreement
	if err := s.db.Order("key ASC").Find(&list).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return list, nil
}

// Versions 返回指定协议的历史版本（最新在前）。
func (s *AgreementService) Versions(key string) ([]model.AgreementVersion, error) {
	var list []model.AgreementVersion
	if err := s.db.Where("key = ?", key).Order("created_at DESC").Limit(50).Find(&list).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return list, nil
}

// PublishReq 发布参数。
type PublishReq struct {
	Title       string
	Version     string
	Content     string
	EffectiveAt time.Time
}

// Publish 发布新版本：写历史版本并覆盖当前生效内容。
func (s *AgreementService) Publish(actor *model.User, key string, req PublishReq) error {
	if key != AgreementKeyUser && key != AgreementKeyPrivacy && key != AgreementKeyData {
		return errs.New(errs.ErrValidation, nil)
	}
	if req.Content == "" {
		return errs.New(errs.ErrValidation, nil)
	}
	version := req.Version
	if version == "" {
		version = time.Now().Format("2006.0102.1504")
	}
	effective := req.EffectiveAt
	if effective.IsZero() {
		effective = time.Now()
	}
	title := req.Title
	if title == "" {
		title = key
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.AgreementVersion{
			Key:         key,
			Version:     version,
			Title:       title,
			Content:     req.Content,
			EffectiveAt: effective,
			CreatedBy:   actor.ID,
			CreatedAt:   time.Now(),
		}).Error; err != nil {
			return err
		}
		var cur model.Agreement
		err := tx.Where("key = ?", key).First(&cur).Error
		if err != nil {
			if err := tx.Create(&model.Agreement{
				Key:         key,
				Title:       title,
				Version:     version,
				Content:     req.Content,
				EffectiveAt: effective,
				CreatedAt:   time.Now(),
			}).Error; err != nil {
				return err
			}
			return nil
		}
		return tx.Model(&cur).Updates(map[string]any{
			"title":        title,
			"version":      version,
			"content":      req.Content,
			"effective_at": effective,
		}).Error
	})
	if err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	Audit(s.db, actor.ID, "agreement.publish", "agreement", key, "发布版本 "+version)
	return nil
}

// Rollback 回滚到指定历史版本（生成新的当前内容，不删除历史）。
func (s *AgreementService) Rollback(actor *model.User, versionID uint) error {
	var v model.AgreementVersion
	if err := s.db.First(&v, versionID).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	return s.Publish(actor, v.Key, PublishReq{
		Title:       v.Title,
		Version:     v.Version,
		Content:     v.Content,
		EffectiveAt: time.Now(),
	})
}
