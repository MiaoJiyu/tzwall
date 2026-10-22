package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/service"
)

// 图片访客令牌：把查看者 uid 以短时效、防篡改的方式编入图片 URL。
// 图片代理是公开的（<img> 无法携带 Authorization 头），借此让代理取到查看者身份，
// 从而在盲水印中写入“用户名”。令牌缺失/篡改/过期时按游客处理，图片仍正常展示。
var (
	imgSecret []byte
	imgStore  *service.ConfigStore
)

// initImageLinks 由路由装配调用，注入签名密钥与配置源（供读取链接有效期）。
func initImageLinks(secret string, store *service.ConfigStore) {
	imgSecret = []byte(secret)
	imgStore = store
}

// imgLinkTTL 图片链接有效期（秒），取后台配置，缺省 600。
func imgLinkTTL() int {
	if imgStore != nil {
		if t := imgStore.Int("image_link_ttl_seconds", 600); t > 0 {
			return t
		}
	}
	return 600
}

// imageURL 登录查看者返回带签名访客令牌的地址；游客返回纯路径。
func imageURL(im model.SubmissionImage, uid int) string {
	return signedImageURL(im.ID, uid)
}

// videoURL 返回视频对外地址（纯路径，无签名；视频不做盲水印）。
func videoURL(v model.SubmissionVideo, uid int) string {
	return fmt.Sprintf("/api/videos/%d", v.ID)
}

// signedImageURL 生成图片地址；uid>0 时附加 u/e/s 访客令牌（uid、过期时间、HMAC-SHA256 签名）。
func signedImageURL(id uint, uid int) string {
	if uid <= 0 || len(imgSecret) == 0 {
		return fmt.Sprintf("/api/images/%d", id)
	}
	exp := time.Now().Unix() + int64(imgLinkTTL())
	payload := strconv.Itoa(uid) + ":" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, imgSecret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("/api/images/%d?u=%d&e=%d&s=%s", id, uid, exp, sig)
}

// signedVideoURL 兼容旧调用签名，返回纯路径（视频无访客令牌）。
func signedVideoURL(id uint, uid int) string {
	return fmt.Sprintf("/api/videos/%d", id)
}

// viewerUIDFromToken 校验图片链接访客令牌，返回 uid；缺失/篡改/过期返回 0。
func viewerUIDFromToken(c *gin.Context) uint {
	u := c.Query("u")
	e := c.Query("e")
	s := c.Query("s")
	if u == "" || e == "" || s == "" || len(imgSecret) == 0 {
		return 0
	}
	exp, err := strconv.ParseInt(e, 10, 64)
	if err != nil || exp < time.Now().Unix() {
		return 0
	}
	payload := u + ":" + e
	mac := hmac.New(sha256.New, imgSecret)
	mac.Write([]byte(payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(want), []byte(s)) != 1 {
		return 0
	}
	uid, err := strconv.ParseUint(u, 10, 64)
	if err != nil {
		return 0
	}
	return uint(uid)
}
