package service

import (
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
)

// ConfigStore 运行时系统配置，带内存缓存与热更新。
type ConfigStore struct {
	db     *gorm.DB
	aesKey []byte // 敏感配置加密密钥（来自 .env）
	mu     sync.RWMutex
	m      map[string]string
	ts     time.Time
}

func NewConfigStore(db *gorm.DB) *ConfigStore {
	return NewConfigStoreWithKey(db, nil)
}

// NewConfigStoreWithKey 构造配置存储；aesKey 为空时敏感配置无法加密保存。
func NewConfigStoreWithKey(db *gorm.DB, aesKey []byte) *ConfigStore {
	s := &ConfigStore{db: db, aesKey: aesKey, m: map[string]string{}}
	s.refresh()
	return s
}

func (s *ConfigStore) refresh() {
	var all []model.SystemConfig
	s.db.Find(&all)
	m := make(map[string]string, len(all))
	for _, c := range all {
		m[c.Key] = c.Value
	}
	s.mu.Lock()
	s.m = m
	s.ts = time.Now()
	s.mu.Unlock()
}

// DB 暴露底层连接（供审计等写入使用）。
func (s *ConfigStore) DB() *gorm.DB { return s.db }

// Get 读取配置（内存优先，缺失回源 DB）。
func (s *ConfigStore) Get(key string) string {
	s.mu.RLock()
	v, ok := s.m[key]
	s.mu.RUnlock()
	if ok {
		return v
	}
	var cfg model.SystemConfig
	if err := s.db.Where("key = ?", key).First(&cfg).Error; err == nil {
		s.mu.Lock()
		s.m[key] = cfg.Value
		s.mu.Unlock()
		return cfg.Value
	}
	return ""
}

// Set 写入并刷新缓存（UPSERT）。
func (s *ConfigStore) Set(key, val string) error {
	var cfg model.SystemConfig
	err := s.db.Where("key = ?", key).First(&cfg).Error
	if err == nil {
		cfg.Value = val
		cfg.UpdatedAt = time.Now()
		err = s.db.Save(&cfg).Error
	} else {
		err = s.db.Create(&model.SystemConfig{Key: key, Value: val, UpdatedAt: time.Now()}).Error
	}
	if err == nil {
		s.mu.Lock()
		s.m[key] = val
		s.mu.Unlock()
	}
	return err
}

func (s *ConfigStore) Int(key string, def int) int {
	v := s.Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func (s *ConfigStore) Bool(key string) bool {
	return s.Get(key) == "true"
}
