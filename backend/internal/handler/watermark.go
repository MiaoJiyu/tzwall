package handler

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/storage"
)

// wmMaxBytes 待识别图片大小上限。
const wmMaxBytes = 20 << 20

// WatermarkHandler 盲水印识别（仅超级管理员可用）。
// 用于泄露溯源：将外泄图片上传至此，还原其中嵌入的访客 IP 与分发时间。
type WatermarkHandler struct {
	storage storage.Storage
}

func NewWatermarkHandler(st storage.Storage) *WatermarkHandler {
	return &WatermarkHandler{storage: st}
}

// wmExtractResult 数据万象盲水印提取响应（version 3.0 文字盲水印）。
// 提取成功时 <Text> 直接返回嵌入的文字，<WatermarkStatusCode> 为 1。
type wmExtractResult struct {
	XMLName        xml.Name `xml:"UploadResult"`
	ProcessResults struct {
		Text                string `xml:"Text"`
		WatermarkStatusCode int    `xml:"WatermarkStatusCode"`
	} `xml:"ProcessResults"`
}

// Extract 识别上传图片中嵌入的盲水印。
//
//	POST /api/admin/watermark/extract   (multipart/form-data，字段 file)
func (h *WatermarkHandler) Extract(c *gin.Context) {
	if requireSuperAdmin(c) == nil {
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		response.FailCode(c, errs.ErrValidation, "请上传待识别的图片")
		return
	}
	if fh.Size > wmMaxBytes {
		response.FailCode(c, errs.ErrValidation, "图片不能超过 20MB")
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, wmMaxBytes))
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "")
		return
	}
	if !h.storage.Configured() {
		// 当前仅 COS 数据万象支持盲水印识别。
		response.FailCode(c, errs.CodeInternal, "当前存储非 COS，无法识别盲水印")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	body, err := h.storage.ExtractTextWatermark(ctx, data, fh.Header.Get("Content-Type"))
	if err != nil {
		response.FailCode(c, errs.CodeInternal, "盲水印识别失败，请确认图片有效且存储为 COS")
		return
	}

	var res wmExtractResult
	if err := xml.Unmarshal(body, &res); err != nil {
		response.OK(c, gin.H{"found": false, "note": "未能解析盲水印响应"})
		return
	}
	status := res.ProcessResults.WatermarkStatusCode
	text := strings.TrimSpace(res.ProcessResults.Text)
	if status != 1 || text == "" {
		response.OK(c, gin.H{
			"found":      false,
			"status_code": status,
			"note":       "未检测到盲水印（可能非本站分发的图片，或未经 version 3.0 水印处理）",
		})
		return
	}

	// 嵌入文本为 base64（URL 安全）后的 "u~<用户名>~i~<IP>~t~<时间戳>"。
	decoded := text
	if raw, derr := base64.URLEncoding.DecodeString(text); derr == nil {
		decoded = string(raw)
	}
	user, ip, ts := parseWatermarkText(decoded)
	embeddedAt := ""
	if sec, perr := strconv.ParseInt(ts, 36, 64); perr == nil {
		embeddedAt = time.Unix(sec, 0).Format("2006-01-02 15:04:05")
	}
	response.OK(c, gin.H{
		"found":       true,
		"status_code": status,
		"raw":         text,
		"decoded":     decoded,
		"username":    user,
		"ip":          ip,
		"ip_addr":     expandIP(ip),
		"embedded_at": embeddedAt,
	})
}

// expandIP 将 base36 编码的 IP 还原为 IPv4 点分地址；非 base36 或越界时原样返回。
// 注意：IPv6/异常来源在嵌入时被压缩为 4 字节哈希指纹，此处只能还原为等长 IPv4 形态（非原始地址）。
func expandIP(s string) string {
	n, err := strconv.ParseUint(s, 36, 64)
	if err != nil || n > 0xFFFFFFFF {
		return s
	}
	v := uint32(n)
	return fmt.Sprintf("%d.%d.%d.%d", v>>24, (v>>16)&0xFF, (v>>8)&0xFF, v&0xFF)
}

// parseWatermarkText 解析 "u~<用户名>~i~<IP base36>~t~<时间戳 base36>" 格式；
// 兼容无用户名的旧格式 "i~<IP base36>~t~<时间戳 base36>"。
func parseWatermarkText(s string) (user, ip, ts string) {
	p := strings.Split(s, "~")
	if len(p) >= 6 && p[0] == "u" && p[2] == "i" && p[4] == "t" {
		return p[1], p[3], p[5]
	}
	if len(p) >= 4 && p[0] == "i" && p[2] == "t" {
		return "", p[1], p[3]
	}
	return "", "", ""
}
