package storage

import (
	"context"
	"errors"
	"io"

	"tzwall/backend/internal/config"
)

// ErrObjectUnavailable 表示对象不可访问：已被 COS 内容审核冻结
// （私有读 → 403，或转移至备份目录 → 404）或已被删除。
var ErrObjectUnavailable = errors.New("object unavailable")

// Storage 文件存储抽象。COS 与本地磁盘均实现此接口，浏览器不直接访问。
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// ProcessedGet 带数据处理规则取流（如 COS 图片盲水印），本地存储忽略规则返回原图。
	ProcessedGet(ctx context.Context, key, ciRule string) (io.ReadCloser, error)
	// Head 轻量探测对象是否存在且可读；不可访问时返回 ErrObjectUnavailable。
	Head(ctx context.Context, key string) error
	SetPrivate(ctx context.Context, key string) error // 变更/删除后将对象设为私有读写
	Configured() bool
	// Probe 连通性探测（运维面板健康检查使用），仅做只读请求。
	Probe(ctx context.Context) error
}

// New 选择存储实现：配置完整凭证时使用 COS（私有读写），否则降级本地磁盘。
func New(cfg *config.Config) Storage {
	if cfg.StorageConfigured {
		if s := NewCOS(cfg); s != nil {
			return s
		}
	}
	return NewLocal(cfg)
}
