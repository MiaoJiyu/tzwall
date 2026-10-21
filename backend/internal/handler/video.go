package handler

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tzwall/backend/internal/middleware"
	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
	"tzwall/backend/internal/storage"
)

// VideoHandler 视频上传与播放代理。视频对象私有存储，浏览器只经后端代理访问。
type VideoHandler struct {
	db      *gorm.DB
	storage storage.Storage
	store   *service.ConfigStore
}

func NewVideoHandler(db *gorm.DB, st storage.Storage, store *service.ConfigStore) *VideoHandler {
	return &VideoHandler{db: db, storage: st, store: store}
}

// Upload 直传 COS（流式，不落内存），落库后进入人工审核；违规由 COS 内容审核回调驳回。
func (h *VideoHandler) Upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	maxMB := h.store.Int("video_max_size_mb", 100)
	if maxMB < 1 {
		maxMB = 100
	}
	if fh.Size > int64(maxMB)*1024*1024 {
		response.FailCode(c, errs.ErrValidation, fmt.Sprintf("视频大小超出限制（最大 %d MB）", maxMB))
		return
	}
	allowed := strings.Split(h.store.Get("video_formats"), ",")
	if h.store.Get("video_formats") == "" {
		allowed = strings.Split("mp4,mov,webm,m4v", ",")
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(fh.Filename)), ".")
	ok := false
	for _, a := range allowed {
		if strings.TrimSpace(a) == ext {
			ok = true
			break
		}
	}
	if !ok {
		response.FailCode(c, errs.ErrValidation, "不支持的视频格式")
		return
	}

	f, err := fh.Open()
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	defer f.Close()

	ct := strings.TrimSpace(fh.Header.Get("Content-Type"))
	if !strings.HasPrefix(ct, "video/") {
		if guessed := mime.TypeByExtension("." + ext); strings.HasPrefix(guessed, "video/") {
			ct = guessed
		} else {
			ct = "video/mp4"
		}
	}
	key := uuid.NewString() + "." + ext
	if _, err := h.storage.Put(c.Request.Context(), key, f, fh.Size, ct); err != nil {
		slog.Default().Warn("视频上传失败", "key", key, "err", err.Error())
		response.FailCode(c, errs.CodeInternal, "")
		return
	}

	v := &model.SubmissionVideo{
		StorageKey:  key,
		Size:        fh.Size,
		ContentType: ct,
		AuditStatus: model.ModerationReview, // 视频统一进入人工审核，违规由 COS 回调改判
		CreatedAt:   time.Now(),
	}
	if err := h.db.Create(v).Error; err != nil {
		slog.Default().Warn("视频元数据写入失败", "key", key, "err", err.Error())
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	uid := 0
	if cu := middleware.CurrentUser(c); cu != nil {
		uid = int(cu.ID)
	}
	response.OK(c, gin.H{
		"id":           v.ID,
		"url":          signedVideoURL(v.ID, uid),
		"content_type": v.ContentType,
		"size":         v.Size,
	})
}

// Proxy 鉴权视频代理，支持 HTTP Range（拖动播放）。
func (h *VideoHandler) Proxy(c *gin.Context) {
	if !refererAllowed(c) {
		response.Fail(c, http.StatusForbidden, errs.CodeForbidden, "防盗链拒绝")
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, _ := strconv.Atoi(c.Query("u"))
	if !verifyVideoURL(uint(id), uid, c.Query("e"), c.Query("s")) {
		response.Fail(c, http.StatusForbidden, errs.CodeForbidden, "视频链接无效或已过期")
		return
	}

	var v model.SubmissionVideo
	if err := h.db.First(&v, id).Error; err != nil {
		response.FailCode(c, errs.ErrValidation, "")
		return
	}
	if v.AuditStatus == model.ModerationBlock || v.IsPrivate {
		response.Fail(c, http.StatusForbidden, errs.ErrSubmissionInvisible, "视频已下架")
		return
	}

	ct := v.ContentType
	if !strings.HasPrefix(ct, "video/") {
		ct = "video/mp4"
	}
	c.Header("Content-Type", ct)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "private, max-age=600")
	c.Header("X-Content-Type-Options", "nosniff")

	total := v.Size
	if rng := c.GetHeader("Range"); rng != "" && total > 0 {
		start, end, ok := parseByteRange(rng, total)
		if !ok {
			c.Header("Content-Range", fmt.Sprintf("bytes */%d", total))
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		rc, _, err := h.storage.GetRange(c.Request.Context(), v.StorageKey, start, end)
		if err != nil {
			h.handleFetchError(c, &v, err)
			return
		}
		defer rc.Close()
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		c.Header("Content-Length", strconv.FormatInt(end-start+1, 10))
		c.Status(http.StatusPartialContent)
		_, _ = io.Copy(c.Writer, rc)
		return
	}

	rc, err := h.storage.Get(c.Request.Context(), v.StorageKey)
	if err != nil {
		h.handleFetchError(c, &v, err)
		return
	}
	defer rc.Close()
	if total > 0 {
		c.Header("Content-Length", strconv.FormatInt(total, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// handleFetchError 对象不可读（COS 冻结 403 / 转移 404）时标记违规、驳回投稿并返回 403。
func (h *VideoHandler) handleFetchError(c *gin.Context, v *model.SubmissionVideo, err error) {
	if errors.Is(err, storage.ErrObjectUnavailable) {
		h.markBlocked(v)
		response.Fail(c, http.StatusForbidden, errs.ErrSubmissionInvisible, "视频已下架")
		return
	}
	slog.Default().Warn("视频取流失败", "key", v.StorageKey, "err", err.Error())
	response.FailCode(c, errs.CodeInternal, "")
}

// markBlocked 将视频标记违规并置私有（幂等），同时驳回所属投稿。
func (h *VideoHandler) markBlocked(v *model.SubmissionVideo) {
	if err := h.db.Model(&model.SubmissionVideo{}).Where("id = ?", v.ID).
		Updates(map[string]any{"audit_status": model.ModerationBlock, "is_private": true}).Error; err != nil {
		slog.Default().Warn("标记视频违规失败", "id", v.ID, "err", err.Error())
		return
	}
	if v.SubmissionID != nil {
		if err := h.db.Model(&model.Submission{}).Where("id = ?", *v.SubmissionID).Updates(map[string]any{
			"status":       model.StatusRejected,
			"audit_status": model.ModerationBlock,
			"admin_note":   "内容未通过审核：视频涉嫌违规",
		}).Error; err != nil {
			slog.Default().Warn("驳回投稿失败", "submission", *v.SubmissionID, "err", err.Error())
		}
	}
	slog.Default().Info("视频判定违规，已下架并驳回投稿", "id", v.ID, "key", v.StorageKey, "submission", v.SubmissionID)
}

// parseByteRange 解析单段 Range 请求头，返回闭区间 [start,end]。
func parseByteRange(h string, total int64) (int64, int64, bool) {
	if total <= 0 || !strings.HasPrefix(h, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimSpace(strings.TrimPrefix(h, "bytes="))
	if i := strings.Index(spec, ","); i >= 0 {
		spec = strings.TrimSpace(spec[:i])
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	startStr, endStr := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	var start, end int64
	if startStr == "" {
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > total {
			n = total
		}
		start, end = total-n, total-1
	} else {
		s, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return 0, 0, false
		}
		start = s
		if endStr == "" {
			end = total - 1
		} else {
			e, err := strconv.ParseInt(endStr, 10, 64)
			if err != nil {
				return 0, 0, false
			}
			end = e
		}
	}
	if start < 0 || start >= total || end < start {
		return 0, 0, false
	}
	if end >= total {
		end = total - 1
	}
	return start, end, true
}
