package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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
		base = s.stdBase()
	}
	rc, err := s.processedGetFrom(ctx, base, key, ciRule)
	if err == nil {
		return rc, nil
	}
	// 配置的自定义域名不可达（解析/证书/内网 SNI 不兼容等）时，回退标准 COS 域名重试一次，
	// 避免盲水印长期静默失败（曾因自定义域名不可用导致全部回退原图）。
	if std := s.stdBase(); std != base {
		if rc2, err2 := s.processedGetFrom(ctx, std, key, ciRule); err2 == nil {
			return rc2, nil
		}
	}
	return nil, err
}

// stdBase 标准 COS 域名（同地域内网由 VPC  DNS 自动解析，无需公网下行）。
func (s *cosStorage) stdBase() string {
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com", s.cfg.CosBucket, s.cfg.CosRegion)
}

// processedGetFrom 从指定域名发起带 CI 规则的取流（签名传输覆盖查询参数）。
func (s *cosStorage) processedGetFrom(ctx context.Context, base, key, ciRule string) (io.ReadCloser, error) {
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

// ExtractTextWatermark 盲水印识别：将待识别图片上传到临时对象，以
// watermark/4/type/3/version/3.0 同步提取文字盲水印，返回数据万象原始响应
// （XML，含 <Text>/<WatermarkStatusCode>），随后删除临时对象。
// 提取需与嵌入同账号同桶；fileid 与临时对象一致，避免额外落对象。
func (s *cosStorage) ExtractTextWatermark(ctx context.Context, data []byte, contentType string) ([]byte, error) {
	// 临时对象：识别后即删，避免桶内残留
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	key := "wmcheck/" + hex.EncodeToString(b)
	// PUT Object + Pic-Operations（上传即提取）。签名传输会将 Pic-Operations 等头签入。
	picOps := fmt.Sprintf(`{"is_pic_info":1,"rules":[{"fileid":%q,"rule":"watermark/4/type/3/version/3.0"}]}`, key)

	var lastErr error
	for _, base := range s.ciBases() {
		body, status, err := s.putPicOps(ctx, base, key, data, contentType, picOps)
		// 识别完成后删除临时对象（与成败无关，且每个候选域名都清理）
		s.deleteObject(ctx, base, key)
		if err != nil {
			lastErr = err
			continue // 域名不可达：换下一个候选
		}
		if status != http.StatusOK {
			if status == http.StatusForbidden || status == http.StatusNotFound {
				return nil, ErrObjectUnavailable
			}
			lastErr = fmt.Errorf("cos 盲水印识别失败: %s", http.StatusText(status))
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

// ciBases 候选域名列表：优先使用配置的 COS_BASE_URL，随后是标准 COS 域名（去重）。
// 自定义域名在内网不可达时（证书/SNI 不兼容等）会自动回退标准域名。
func (s *cosStorage) ciBases() []string {
	std := s.stdBase()
	cfgBase := strings.TrimRight(strings.TrimSpace(s.cfg.CosBaseURL), "/")
	if cfgBase == "" || !strings.Contains(cfgBase, "://") || cfgBase == std {
		return []string{std}
	}
	return []string{cfgBase, std}
}

// putPicOps 上传临时对象并携带 Pic-Operations（用于盲水印识别）。
func (s *cosStorage) putPicOps(ctx context.Context, base, key string, data []byte, contentType, picOps string) ([]byte, int, error) {
	u := base + "/" + url.PathEscape(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Pic-Operations", picOps)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	return body, resp.StatusCode, nil
}

// deleteObject 尽力删除 COS 对象（清理识别用的临时对象）。
func (s *cosStorage) deleteObject(ctx context.Context, base, key string) {
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(dctx, http.MethodDelete, base+"/"+url.PathEscape(key), nil)
	if err != nil {
		return
	}
	if resp, err := s.httpClient.Do(req); err == nil {
		_ = resp.Body.Close()
	}
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

// Purge 透传清除底层本地兜底副本；COS 对象由审核下架逻辑置为私有，无需物理删除。
func (s *cosStorage) Purge(ctx context.Context, key string) error {
	if s.local != nil {
		return s.local.Purge(ctx, key)
	}
	return nil
}

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
