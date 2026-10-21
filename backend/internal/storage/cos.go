package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"

	"tzwall/backend/internal/config"
)

// cosStorage 腾讯云 COS 实现：对象私有读写，浏览器只经后端代理访问。
// 为兼容“先本地后配置 COS”的存量数据，Get 在 COS 未命中时回退本地磁盘。
type cosStorage struct {
	cfg        *config.Config
	client     *cos.Client
	httpClient *http.Client // 带签名传输，用于带 CI 规则的原始请求
	local      Storage
}

// NewCOS 构造 COS 存储；凭证不完整时返回 nil 由调用方降级。
func NewCOS(cfg *config.Config) *cosStorage {
	if cfg.CosBucket == "" || cfg.CosRegion == "" || cfg.CosSecretID == "" || cfg.CosSecretKey == "" {
		return nil
	}
	u, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.CosBucket, cfg.CosRegion))
	if err != nil {
		return nil
	}
	transport := &cos.AuthorizationTransport{
		SecretID:  cfg.CosSecretID,
		SecretKey: cfg.CosSecretKey,
	}
	hc := &http.Client{Transport: transport}
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, hc)
	local := NewLocal(cfg)
	return &cosStorage{cfg: cfg, client: client, httpClient: hc, local: local}
}

// Put 上传对象并设置为私有读写（仅后端可读取，浏览器经后端代理访问）。
func (s *cosStorage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: contentType},
		ACLHeaderOptions:       &cos.ACLHeaderOptions{XCosACL: "private"},
	}
	if _, err := s.client.Object.Put(ctx, key, r, opt); err != nil {
		return "", err
	}
	return key, nil
}

// Get 后端内部取流（不生成对外签名 URL）；COS 未命中时回退本地磁盘。
func (s *cosStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.client.Object.Get(ctx, key, nil)
	if err == nil {
		return resp.Body, nil
	}
	if s.local != nil {
		if rc, lerr := s.local.Get(ctx, key); lerr == nil {
			return rc, nil
		}
	}
	if st := cosStatusCode(err); st == http.StatusForbidden || st == http.StatusNotFound {
		return nil, ErrObjectUnavailable
	}
	return nil, err
}

// GetRange 分段读取对象（视频拖动播放），返回流与对象总大小。
func (s *cosStorage) GetRange(ctx context.Context, key string, start, end int64) (io.ReadCloser, int64, error) {
	rng := fmt.Sprintf("bytes=%d-", start)
	if end >= 0 {
		rng = fmt.Sprintf("bytes=%d-%d", start, end)
	}
	resp, err := s.client.Object.Get(ctx, key, &cos.ObjectGetOptions{Range: rng})
	if err == nil {
		total := totalFromContentRange(resp.Header.Get("Content-Range"))
		if total <= 0 {
			total = end + 1
		}
		return resp.Body, total, nil
	}
	if s.local != nil {
		if rc, total, lerr := s.local.GetRange(ctx, key, start, end); lerr == nil {
			return rc, total, nil
		}
	}
	if st := cosStatusCode(err); st == http.StatusForbidden || st == http.StatusNotFound {
		return nil, 0, ErrObjectUnavailable
	}
	return nil, 0, err
}

// totalFromContentRange 从 "bytes 0-1023/4096" 中解析对象总大小。
func totalFromContentRange(cr string) int64 {
	i := strings.LastIndex(cr, "/")
	if i < 0 {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(cr[i+1:]), 10, 64)
	return n
}

// ProcessedGet 带数据处理规则取流（如盲水印）。签名传输会自动将查询参数纳入签名。
func (s *cosStorage) ProcessedGet(ctx context.Context, key, ciRule string) (io.ReadCloser, error) {
	if ciRule == "" {
		return s.Get(ctx, key)
	}
	base := strings.TrimRight(strings.TrimSpace(s.cfg.CosBaseURL), "/")
	if base == "" || !strings.Contains(base, "://") {
		base = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", s.cfg.CosBucket, s.cfg.CosRegion)
	}
	u := base + "/" + url.PathEscape(key) + "?" + ciRule
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			return nil, ErrObjectUnavailable
		}
		return nil, fmt.Errorf("cos 图片处理请求失败: %s", resp.Status)
	}
	return resp.Body, nil
}

// Head 轻量探测对象可访问性。COS 内容审核冻结后：私有读返回 403、转移备份目录返回 404，
// 均视为不可用（除非本地仍有存量副本）。
func (s *cosStorage) Head(ctx context.Context, key string) error {
	_, err := s.client.Object.Head(ctx, key, nil)
	if err == nil {
		return nil
	}
	if st := cosStatusCode(err); st == http.StatusForbidden || st == http.StatusNotFound {
		if s.local != nil {
			if lerr := s.local.Head(ctx, key); lerr == nil {
				return nil
			}
		}
		return ErrObjectUnavailable
	}
	return err
}

// cosStatusCode 提取 COS 错误响应状态码，非 COS 错误返回 0。
func cosStatusCode(err error) int {
	var cerr *cos.ErrorResponse
	if errors.As(err, &cerr) && cerr.Response != nil {
		return cerr.Response.StatusCode
	}
	return 0
}

// SetPrivate 将对象 ACL 设为私有读写。
func (s *cosStorage) SetPrivate(ctx context.Context, key string) error {
	_, err := s.client.Object.PutACL(ctx, key, &cos.ObjectPutACLOptions{
		Header: &cos.ACLHeaderOptions{XCosACL: "private"},
	})
	return err
}

func (s *cosStorage) Configured() bool { return true }

// Probe 以只读方式探测 COS 连通性（查询对象是否存在，不产生副作用）。
func (s *cosStorage) Probe(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := s.client.Object.Head(probeCtx, "tzwall-health-probe", nil)
	if err == nil {
		return nil
	}
	// 404（对象不存在）同样说明连通正常
	var cerr *cos.ErrorResponse
	if errors.As(err, &cerr) && cerr.Response != nil && cerr.Response.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}
