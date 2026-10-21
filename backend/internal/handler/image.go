package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/google/uuid"
	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/config"
	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/moderation"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
	"tzwall/backend/internal/storage"
)

// ImageHandler 图片上传与代理（后端烧录水印、短缓存、防盗链）。
type ImageHandler struct {
	db      *gorm.DB
	storage storage.Storage
	cfg     *config.Config
	cache   cache.Cache
	store   *service.ConfigStore
	mod     moderation.Moderation
}

func NewImageHandler(db *gorm.DB, st storage.Storage, cfg *config.Config, c cache.Cache, store *service.ConfigStore, mod moderation.Moderation) *ImageHandler {
	return &ImageHandler{db: db, storage: st, cfg: cfg, cache: c, store: store, mod: mod}
}

// Upload 上传图片到存储，写入元数据。
func (h *ImageHandler) Upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	maxMB := h.store.Int("image_max_size_mb", 5)
	if fh.Size > int64(maxMB)*1024*1024 {
		response.FailCode(c, errs.ErrImageTooLarge, "")
		return
	}
	allowed := strings.Split(h.store.Get("image_formats"), ",")
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(fh.Filename)), ".")
	ok := false
	for _, a := range allowed {
		if strings.TrimSpace(a) == ext {
			ok = true
			break
		}
	}
	if !ok {
		response.FailCode(c, errs.ErrValidation, "不支持的图片格式")
		return
	}

	f, err := fh.Open()
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	key := uuid.NewString() + "." + ext
	if _, err := h.storage.Put(c.Request.Context(), key, bytes.NewReader(data), fh.Size, fh.Header.Get("Content-Type")); err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	sum := sha256.Sum256(data)
	audit := model.ModerationNone
	// 上传后即时审核：失败一律转人工，绝不自动放行
	if h.mod != nil && h.mod.Configured() {
		v, err := h.mod.SubmitImage(c.Request.Context(), "img-"+key, key)
		if err != nil {
			audit = model.ModerationReview
			slog.Default().Warn("图片审核调用失败，转人工", "key", key, "err", err.Error())
		} else {
			audit = moderation.StatusOf(v)
		}
	}
	im := &model.SubmissionImage{
		StorageKey:  key,
		Hash:        hex.EncodeToString(sum[:]),
		Size:        fh.Size,
		ContentType: fh.Header.Get("Content-Type"),
		AuditStatus: audit,
		CreatedAt:   time.Now(),
	}
	if err := h.db.Create(im).Error; err != nil {
		slog.Default().Warn("图片元数据写入失败", "key", key, "err", err.Error())
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	upUID := 0
	if cu := middleware.CurrentUser(c); cu != nil {
		upUID = int(cu.ID)
	}
	response.OK(c, gin.H{"id": im.ID, "content_type": im.ContentType, "url": signedImageURL(im.ID, upUID)})
}

// Proxy 鉴权图片代理：从存储取原图并短缓存返回（不再烧录水印，全站水印由前端覆盖层承担）。
func (h *ImageHandler) Proxy(c *gin.Context) {
	if !refererAllowed(c) {
		response.Fail(c, http.StatusForbidden, errs.CodeForbidden, "防盗链拒绝")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 签名校验：链接须带有效期内签名，防外链盗用；u 为浏览者ID（参与签名与盲水印）
	uid, _ := strconv.Atoi(c.Query("u"))
	if !verifyImageURL(uint(id), uid, c.Query("e"), c.Query("s")) {
		response.Fail(c, http.StatusForbidden, errs.CodeForbidden, "图片链接无效或已过期")
		return
	}

	var im model.SubmissionImage
	if err := h.db.First(&im, id).Error; err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}

	// 违规/已置为私有的图片不再对外提供（腾讯云敏感标记后 ACL 私有）
	if im.AuditStatus == model.ModerationBlock || im.IsPrivate {
		response.Fail(c, http.StatusForbidden, errs.ErrSubmissionInvisible, "图片已下架")
		return
	}

	cacheKey := fmt.Sprintf("img:%d", id)
	freshKey := fmt.Sprintf("imgok:%d", id)

	// 可用性探测：COS 内容审核自动冻结后（私有读→403 / 转移备份目录→404），
	// 对象将不可访问。这里最多每 60 秒探测一次，命中即标记违规并停止展示。
	needCheck := true
	if h.cache != nil {
		if _, ok := h.cache.Get(freshKey); ok {
			needCheck = false
		}
	}
	if needCheck {
		headCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		herr := h.storage.Head(headCtx, im.StorageKey)
		cancel()
		if herr != nil {
			if errors.Is(herr, storage.ErrObjectUnavailable) {
				h.markBlocked(&im)
				h.dropImageCache(cacheKey, freshKey)
				response.Fail(c, http.StatusForbidden, errs.ErrSubmissionInvisible, "图片已下架")
				return
			}
			// 其他错误（网络抖动等）不阻断，继续尝试取流
		} else if h.cache != nil {
			h.cache.Set(freshKey, "1", 60*time.Second)
		}
	}

	// 盲水印：逐请求写入 用户名+IP+请求时间，故不共享字节缓存（默认开启）
	wmCfg := h.store.Get("image_blind_watermark")
	useWatermark := strings.HasPrefix(im.ContentType, "image/") && (wmCfg == "" || wmCfg == "true")

	// 缓存键仅按图片 ID：仅在无水印时使用共享缓存
	if !useWatermark && h.cache != nil {
		if v, ok := h.cache.Get(cacheKey); ok {
			dec, err := hex.DecodeString(v)
			if err == nil {
				c.Data(http.StatusOK, contentTypeOrDefault(im.ContentType), dec)
				return
			}
		}
	}

	var r io.ReadCloser
	var err error
	if useWatermark {
		rule := blindWatermarkRule(c, uid)
		r, err = h.storage.ProcessedGet(c.Request.Context(), im.StorageKey, rule)
		if err != nil {
			// 图片处理不可用时回退原图，保证仍可显示
			slog.Default().Warn("盲水印处理失败，回退原图", "key", im.StorageKey, "err", err.Error())
			r, err = h.storage.Get(c.Request.Context(), im.StorageKey)
		}
	} else {
		r, err = h.storage.Get(c.Request.Context(), im.StorageKey)
	}
	if err != nil {
		if errors.Is(err, storage.ErrObjectUnavailable) {
			// 已被冻结/删除：标记违规并下架
			h.markBlocked(&im)
			h.dropImageCache(cacheKey, freshKey)
			response.Fail(c, http.StatusForbidden, errs.ErrSubmissionInvisible, "图片已下架")
			return
		}
		slog.Default().Warn("图片取流失败", "key", im.StorageKey, "err", err.Error())
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, 32<<20))
	if err != nil {
		slog.Default().Warn("图片读取失败", "key", im.StorageKey, "err", err.Error())
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	if !useWatermark && h.cache != nil {
		ttl := time.Duration(h.store.Int("watermark_cache_seconds", 600)) * time.Second
		h.cache.Set(cacheKey, hex.EncodeToString(out), ttl)
	}
	c.Data(http.StatusOK, contentTypeOrDefault(im.ContentType), out)
}

// markBlocked 将图片标记为违规并置私有（幂等），同时立即驳回该图片所属投稿。
func (h *ImageHandler) markBlocked(im *model.SubmissionImage) {
	if err := h.db.Model(&model.SubmissionImage{}).Where("id = ?", im.ID).
		Updates(map[string]any{"audit_status": model.ModerationBlock, "is_private": true}).Error; err != nil {
		slog.Default().Warn("标记图片违规失败", "id", im.ID, "err", err.Error())
		return
	}
	if im.SubmissionID != nil {
		if err := h.db.Model(&model.Submission{}).Where("id = ?", *im.SubmissionID).Updates(map[string]any{
			"status":       model.StatusRejected,
			"audit_status": model.ModerationBlock,
			"admin_note":   "内容未通过审核：图片涉嫌违规",
		}).Error; err != nil {
			slog.Default().Warn("驳回投稿失败", "submission", *im.SubmissionID, "err", err.Error())
		}
	}
	slog.Default().Info("图片判定违规，已下架并驳回投稿", "id", im.ID, "key", im.StorageKey, "submission", im.SubmissionID)
}

// dropImageCache 清除图片内容与可用性标记缓存。
func (h *ImageHandler) dropImageCache(cacheKey, freshKey string) {
	if h.cache == nil {
		return
	}
	h.cache.Del(cacheKey)
	h.cache.Del(freshKey)
}

// SyncPrivate 将全部图片对象设为私有读写（撤销可能存在的公有读）。
func (h *ImageHandler) SyncPrivate(c *gin.Context) {
	var imgs []model.SubmissionImage
	h.db.Find(&imgs)
	fixed := 0
	for _, im := range imgs {
		if err := h.storage.SetPrivate(context.Background(), im.StorageKey); err != nil {
			slog.Default().Warn("设置图片私有失败", "id", im.ID, "err", err.Error())
			continue
		}
		fixed++
	}
	response.OK(c, gin.H{"total": len(imgs), "private": fixed})
}

// blindWatermarkRule 构造 COS 数据万象盲水印处理规则。
// 文本 = 用户ID + IP + 请求时间（文字盲水印仅支持 [A-Za-z0-9]，故用单字母分隔）。
func blindWatermarkRule(c *gin.Context, uid int) string {
	text := "u" + strconv.FormatInt(int64(uid), 36) + "i" + compactIP(middleware.ClientIP(c)) + "t" + strconv.FormatInt(time.Now().Unix(), 36)
	b64 := base64.URLEncoding.EncodeToString([]byte(text))
	return "watermark/3/type/3/text/" + b64
}

// compactIP 将 IP 压缩为 base36：IPv4 转整数；其他类型取哈希前 4 字节。
func compactIP(s string) string {
	if ip := net.ParseIP(s); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			n := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
			return strconv.FormatUint(uint64(n), 36)
		}
		sum := sha256.Sum256(ip)
		n := uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3])
		return strconv.FormatUint(uint64(n), 36)
	}
	return keepAlnum(s)
}

// keepAlnum 仅保留字母与数字（盲水印文字限制）。
func keepAlnum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// contentTypeOrDefault 缺失或不规范的 Content-Type 时回退通用图片类型。
func contentTypeOrDefault(ct string) string {
	if strings.HasPrefix(ct, "image/") {
		return ct
	}
	return "image/jpeg"
}

// refererAllowed 基础防盗链：Referer 非空且 host 与本站不一致则拒绝。
func refererAllowed(c *gin.Context) bool {
	ref := c.GetHeader("Referer")
	if ref == "" {
		return true
	}
	parsed, err := url.Parse(ref)
	if err != nil || parsed.Host == "" {
		return true
	}
	return parsed.Host == c.Request.Host
}
