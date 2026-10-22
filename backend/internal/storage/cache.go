package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// cachedStorage 在底层存储之上叠加一层本地磁盘源站缓存（源站盾）：
// 首次取流时将 COS 对象完整下载到本地，后续 Get/GetRange 直接读本地磁盘，
// 不再回源，从而显著减少 COS 出流量。对象内容不可变（key 唯一），
// 缓存可长期复用，仅在 Purge（下架/删除）时失效。
type cachedStorage struct {
	inner    Storage
	dir      string
	maxBytes int64

	mu        sync.Mutex
	inflight  sync.Map // key -> *sync.Mutex，保证同一对象并发只下载一次
	lastSweep time.Time
}

// NewCached 包裹底层存储，启用本地磁盘源站缓存。dir 为空时回退到 ./data/media_cache，
// maxGB<=0 时默认 10GB。
func NewCached(inner Storage, dir string, maxGB int) Storage {
	if dir == "" {
		dir = "./data/media_cache"
	}
	_ = os.MkdirAll(dir, 0o750)
	if maxGB <= 0 {
		maxGB = 10
	}
	return &cachedStorage{
		inner:    inner,
		dir:      dir,
		maxBytes: int64(maxGB) * 1024 * 1024 * 1024,
	}
}

func (s *cachedStorage) hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func (s *cachedStorage) cachePath(key string) string {
	h := s.hashKey(key)
	return filepath.Join(s.dir, h[:2], h+".bin")
}

func (s *cachedStorage) sizePath(key string) string {
	h := s.hashKey(key)
	return filepath.Join(s.dir, h[:2], h+".size")
}

// ensure 确保对象已缓存到本地磁盘，返回缓存文件路径与对象总大小。
func (s *cachedStorage) ensure(ctx context.Context, key string) (string, int64, error) {
	cp, sp := s.cachePath(key), s.sizePath(key)
	if path, size, ok := s.cachedHit(cp, sp); ok {
		return path, size, nil
	}

	// 串行下载：同一 key 并发请求只下载一次
	muI, _ := s.inflight.LoadOrStore(key, &sync.Mutex{})
	mu := muI.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// 双重检查，避免并发等待者重复下载
	if path, size, ok := s.cachedHit(cp, sp); ok {
		return path, size, nil
	}

	rc, err := s.inner.Get(ctx, key)
	if err != nil {
		return "", 0, err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(cp), 0o750); err != nil {
		return "", 0, err
	}
	tmp := cp + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", 0, err
	}
	written, err := io.Copy(f, rc)
	if err != nil {
		f.Close()
		os.Remove(tmp)
		return "", 0, err
	}
	f.Close()
	if err := os.WriteFile(sp, []byte(fmt.Sprintf("%d", written)), 0o640); err != nil {
		os.Remove(tmp)
		return "", 0, err
	}
	if err := os.Rename(tmp, cp); err != nil {
		os.Remove(tmp)
		return "", 0, err
	}
	s.maybeEvict()
	return cp, written, nil
}

// cachedHit 校验本地缓存命中且大小一致。
func (s *cachedStorage) cachedHit(cp, sp string) (string, int64, bool) {
	data, err := os.ReadFile(sp)
	if err != nil {
		return "", 0, false
	}
	var expected int64
	if _, err := fmt.Sscanf(string(data), "%d", &expected); err != nil || expected <= 0 {
		return "", 0, false
	}
	f, err := os.Open(cp)
	if err != nil {
		return "", 0, false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() != expected {
		return "", 0, false
	}
	return cp, expected, true
}

func (s *cachedStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.inner == nil {
		return nil, ErrObjectUnavailable
	}
	path, _, err := s.ensure(ctx, key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// ProcessedGet 盲水印等处理结果含按用户变化的参数，不缓存，直接回源。
func (s *cachedStorage) ProcessedGet(ctx context.Context, key, ciRule string) (io.ReadCloser, error) {
	if s.inner == nil {
		return nil, ErrObjectUnavailable
	}
	return s.inner.ProcessedGet(ctx, key, ciRule)
}

// ExtractTextWatermark 盲水印识别含一次性临时对象，不缓存，直接透传底层实现。
func (s *cachedStorage) ExtractTextWatermark(ctx context.Context, data []byte, contentType string) ([]byte, error) {
	if s.inner == nil {
		return nil, ErrObjectUnavailable
	}
	return s.inner.ExtractTextWatermark(ctx, data, contentType)
}

func (s *cachedStorage) GetRange(ctx context.Context, key string, start, end int64) (io.ReadCloser, int64, error) {
	if s.inner == nil {
		return nil, 0, ErrObjectUnavailable
	}
	path, _, err := s.ensure(ctx, key)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	st, _ := f.Stat()
	total := st.Size()
	if start < 0 || start >= total {
		f.Close()
		return nil, total, ErrObjectUnavailable
	}
	if end < 0 || end >= total {
		end = total - 1
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		f.Close()
		return nil, total, err
	}
	return &limitedReadCloser{Reader: io.LimitReader(f, end-start+1), c: f}, total, nil
}

func (s *cachedStorage) Head(ctx context.Context, key string) error {
	if s.inner == nil {
		return ErrObjectUnavailable
	}
	return s.inner.Head(ctx, key)
}

func (s *cachedStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	if s.inner == nil {
		return "", ErrObjectUnavailable
	}
	s.purgeFile(key)
	return s.inner.Put(ctx, key, r, size, contentType)
}

func (s *cachedStorage) SetPrivate(ctx context.Context, key string) error {
	if s.inner == nil {
		return ErrObjectUnavailable
	}
	return s.inner.SetPrivate(ctx, key)
}

// Purge 清除本地源站缓存副本，并透传底层实现。
func (s *cachedStorage) Purge(ctx context.Context, key string) error {
	s.purgeFile(key)
	if p, ok := s.inner.(interface{ Purge(context.Context, string) error }); ok {
		_ = p.Purge(ctx, key)
	}
	return nil
}

func (s *cachedStorage) Configured() bool {
	if s.inner == nil {
		return false
	}
	return s.inner.Configured()
}

func (s *cachedStorage) Probe(ctx context.Context) error {
	if s.inner == nil {
		return ErrObjectUnavailable
	}
	return s.inner.Probe(ctx)
}

func (s *cachedStorage) purgeFile(key string) {
	_ = os.Remove(s.cachePath(key))
	_ = os.Remove(s.sizePath(key))
}

// maybeEvict 超出容量时按修改时间淘汰最旧文件（LRU 近似），限频执行避免频繁全目录扫描。
func (s *cachedStorage) maybeEvict() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastSweep) < 30*time.Second {
		return
	}
	s.lastSweep = time.Now()

	var used int64
	type finfo struct {
		path  string
		mtime time.Time
		size  int64
	}
	var files []finfo
	_ = filepath.Walk(s.dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".bin" {
			used += info.Size()
			files = append(files, finfo{path: p, mtime: info.ModTime(), size: info.Size()})
		}
		return nil
	})
	if used <= s.maxBytes {
		return
	}
	threshold := int64(float64(s.maxBytes) * 0.9)
	sort.Slice(files, func(i, j int) bool { return files[i].mtime.Before(files[j].mtime) })
	for _, f := range files {
		if used <= threshold {
			break
		}
		_ = os.Remove(f.path)
		_ = os.Remove(strings.TrimSuffix(f.path, ".bin") + ".size")
		used -= f.size
	}
}
