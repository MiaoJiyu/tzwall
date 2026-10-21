package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"tzwall/backend/internal/config"
)

// localStorage 本地磁盘存储，私有读写由后端代理控制。
type localStorage struct {
	dir        string
	configured bool
}

func NewLocal(cfg *config.Config) *localStorage {
	dir := "./data/uploads"
	_ = os.MkdirAll(dir, 0o750)
	return &localStorage{dir: dir, configured: true}
}

func (s *localStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	p := filepath.Join(s.dir, filepath.Base(key)) // 防路径穿越
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return key, nil
}

func (s *localStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.dir, filepath.Base(key)))
}

// ProcessedGet 本地存储不支持图片处理，忽略规则返回原图。
func (s *localStorage) ProcessedGet(ctx context.Context, key, ciRule string) (io.ReadCloser, error) {
	return s.Get(ctx, key)
}

// Head 探测本地对象是否存在。
func (s *localStorage) Head(ctx context.Context, key string) error {
	if _, err := os.Stat(filepath.Join(s.dir, filepath.Base(key))); err != nil {
		return ErrObjectUnavailable
	}
	return nil
}

func (s *localStorage) SetPrivate(ctx context.Context, key string) error {
	// 本地磁盘文件本身即私有，仅后端可读，无需额外处理。
	return nil
}

func (s *localStorage) Configured() bool { return s.configured }

// Probe 本地目录可写即视为健康。
func (s *localStorage) Probe(ctx context.Context) error {
	f, err := os.CreateTemp(s.dir, ".probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}
