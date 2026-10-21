package service

import (
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// 词表分类
const (
	WordCategoryForbidden = "forbidden" // 违禁词：直接拦截
	WordCategoryCommon    = "common"    // 常用词：命中仅记录，不阻断
)

// WordFilterService 词表服务：全量载入内存，变更即刷新，避免逐次查库。
type WordFilterService struct {
	db        *gorm.DB
	mu        sync.RWMutex
	forbidden []string
	common    []string
	loadedAt  time.Time
}

// NewWordFilter 构造词表服务并预热缓存。
func NewWordFilter(db *gorm.DB) *WordFilterService {
	w := &WordFilterService{db: db}
	w.Refresh()
	return w
}

// Refresh 重新加载词表（写入后调用）。
func (w *WordFilterService) Refresh() {
	if w.db == nil {
		return
	}
	var all []model.WordFilter
	if err := w.db.Find(&all).Error; err != nil {
		return
	}
	fb := make([]string, 0, len(all))
	cm := make([]string, 0, len(all))
	for _, it := range all {
		word := strings.TrimSpace(it.Word)
		if word == "" {
			continue
		}
		switch it.Category {
		case WordCategoryCommon:
			cm = append(cm, word)
		default:
			fb = append(fb, word)
		}
	}
	w.mu.Lock()
	w.forbidden = fb
	w.common = cm
	w.loadedAt = time.Now()
	w.mu.Unlock()
}

// Hit 命中信息。
type Hit struct {
	Word     string `json:"word"`
	Category string `json:"category"`
}

// Check 返回文本命中的词（含常用词，不阻断）。
func (w *WordFilterService) Check(text string) []Hit {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var hits []Hit
	w.mu.RLock()
	for _, word := range w.forbidden {
		if word != "" && strings.Contains(lower, strings.ToLower(word)) {
			hits = append(hits, Hit{Word: word, Category: WordCategoryForbidden})
		}
	}
	for _, word := range w.common {
		if word != "" && strings.Contains(lower, strings.ToLower(word)) {
			hits = append(hits, Hit{Word: word, Category: WordCategoryCommon})
		}
	}
	w.mu.RUnlock()
	return hits
}

// HasForbidden 是否命中违禁词。
func (w *WordFilterService) HasForbidden(text string) bool {
	return len(w.forbiddenHits(text)) > 0
}

func (w *WordFilterService) forbiddenHits(text string) []Hit {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var hits []Hit
	w.mu.RLock()
	for _, word := range w.forbidden {
		if word != "" && strings.Contains(lower, strings.ToLower(word)) {
			hits = append(hits, Hit{Word: word, Category: WordCategoryForbidden})
		}
	}
	w.mu.RUnlock()
	return hits
}

// CheckUsername 校验证用户名：违禁词直接拒绝。
func (w *WordFilterService) CheckUsername(name string) error {
	if w.HasForbidden(name) {
		return errs.New(errs.ErrWordFiltered, nil)
	}
	return nil
}

// CheckContent 校验内容：命中违禁词返回错误，命中常用词不阻断。
func (w *WordFilterService) CheckContent(text string) error {
	if w.HasForbidden(text) {
		return errs.New(errs.ErrWordFiltered, nil)
	}
	return nil
}

// List 词表管理列表（支持分类与关键字筛选）。
func (w *WordFilterService) List(category, keyword string, page, pageSize int) ([]model.WordFilter, int64, error) {
	q := w.db.Model(&model.WordFilter{})
	if category == WordCategoryForbidden || category == WordCategoryCommon {
		q = q.Where("category = ?", category)
	}
	if keyword != "" {
		q = q.Where("word ILIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	var items []model.WordFilter
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	return items, total, nil
}

// Add 新增词条（已存在则忽略）。
func (w *WordFilterService) Add(actor *model.User, words []string, category string) (int, error) {
	if category != WordCategoryForbidden && category != WordCategoryCommon {
		category = WordCategoryForbidden
	}
	added := 0
	for _, raw := range words {
		word := strings.TrimSpace(raw)
		if word == "" {
			continue
		}
		var cnt int64
		w.db.Model(&model.WordFilter{}).Where("word = ?", word).Count(&cnt)
		if cnt > 0 {
			continue
		}
		if err := w.db.Create(&model.WordFilter{Word: word, Category: category}).Error; err != nil {
			continue
		}
		added++
	}
	if added > 0 {
		w.Refresh()
		if actor != nil {
			Audit(w.db, actor.ID, "wordfilter.add", "word", "", "新增词条 "+itoa(uint(added)))
		}
	}
	return added, nil
}

// Remove 删除词条。
func (w *WordFilterService) Remove(actor *model.User, id uint) error {
	if err := w.db.Delete(&model.WordFilter{}, id).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	w.Refresh()
	if actor != nil {
		Audit(w.db, actor.ID, "wordfilter.remove", "word", itoa(id), "删除词条")
	}
	return nil
}
