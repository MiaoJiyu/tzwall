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
	// ExtractTextWatermark 盲水印识别：将待识别图片字节上传到临时对象，
	// 以 watermark/4/type/3/version/3.0 提取文字盲水印，返回数据万象原始响应
	// （XML，含 <Text>/<WatermarkStatusCode>），随后删除临时对象。
	// 仅 COS 数据万象支持（要求与嵌入同账号同桶）；本地存储返回错误。
	ExtractTextWatermark(ctx context.Context, data []byte, contentType string) ([]byte, error)
	// GetRange 读取对象 [start, end] 字节区间（end < 0 表示到末尾），返回流与对象总大小。
	GetRange(ctx context.Context, key string, start, end int64) (io.ReadCloser, int64, error)
	// Head 轻量探测对象是否存在且可读；不可访问时返回 ErrObjectUnavailable。
	Head(ctx context.Context, key string) error
	SetPrivate(ctx context.Context, key string) error // 变更/删除后将对象设为私有读写
	// Purge 清除本地源站缓存副本（下架/删除时调用），并透传底层实现。
	Purge(ctx context.Context, key string) error
	Configured() bool
	// Probe 连通性探测（运维面板健康检查使用），仅做只读请求。
	Probe(ctx context.Context) error
}

// New 选择存储实现：配置完整凭证时使用 COS（私有读写），否则降级本地磁盘。
// 返回的存储统一包裹一层本地磁盘源站缓存（CachedStorage），避免每次请求回源 COS。
func New(cfg *config.Config, mediaCacheDir string, mediaCacheMaxGB int) Storage {
	var inner Storage
	if cfg.StorageConfigured {
		if s := NewCOS(cfg); s != nil {
			inner = s
		}
	}
	if inner == nil {
		inner = NewLocal(cfg)
	}
	return NewCached(inner, mediaCacheDir, mediaCacheMaxGB)
}
