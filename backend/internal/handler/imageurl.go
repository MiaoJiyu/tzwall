package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"tzwall/backend/internal/model"
)

// imgSign 图片链接签名配置（密钥由 AES 密钥派生，默认时效 10 分钟）。
var imgSign = struct {
	key []byte
	ttl time.Duration
}{nil, 10 * time.Minute}

// SetImageSigner 注入图片链接签名密钥与有效期。
func SetImageSigner(key []byte, ttl time.Duration) {
	imgSign.key = key
	if ttl > 0 {
		imgSign.ttl = ttl
	}
}

// imageSig 计算图片链接签名（HMAC-SHA256，绑定图片ID、浏览者ID与过期时间）。
func imageSig(id uint, uid int, exp int64) string {
	mac := hmac.New(sha256.New, imgSign.key)
	fmt.Fprintf(mac, "img:%d:%d:%d", id, uid, exp)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// signedImageURL 生成带时效、防盗并与浏览者绑定的图片链接。
// u 为浏览者用户ID（匿名 0），会随签名一起校验，并用于盲水印。
func signedImageURL(id uint, uid int) string {
	exp := time.Now().Add(imgSign.ttl).Unix()
	return fmt.Sprintf("/api/images/%d?e=%d&u=%d&s=%s", id, exp, uid, imageSig(id, uid, exp))
}

// verifyImageURL 校验图片链接签名与时效。
func verifyImageURL(id uint, uid int, e, s string) bool {
	if s == "" {
		return false
	}
	exp, err := strconv.ParseInt(e, 10, 64)
	if err != nil || exp < time.Now().Unix() {
		return false
	}
	return hmac.Equal([]byte(imageSig(id, uid, exp)), []byte(s))
}

// imageURL 返回图片对外地址（签名时效链接），uid 为浏览者用户ID。
func imageURL(im model.SubmissionImage, uid int) string {
	return signedImageURL(im.ID, uid)
}

// videoSig 计算视频链接签名（与图片区分命名空间，避免 ID 撞车）。
func videoSig(id uint, uid int, exp int64) string {
	mac := hmac.New(sha256.New, imgSign.key)
	fmt.Fprintf(mac, "vid:%d:%d:%d", id, uid, exp)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// signedVideoURL 生成带时效、防盗并与浏览者绑定的视频链接。
func signedVideoURL(id uint, uid int) string {
	exp := time.Now().Add(imgSign.ttl).Unix()
	return fmt.Sprintf("/api/videos/%d?e=%d&u=%d&s=%s", id, exp, uid, videoSig(id, uid, exp))
}

// verifyVideoURL 校验视频链接签名与时效。
func verifyVideoURL(id uint, uid int, e, s string) bool {
	if s == "" {
		return false
	}
	exp, err := strconv.ParseInt(e, 10, 64)
	if err != nil || exp < time.Now().Unix() {
		return false
	}
	return hmac.Equal([]byte(videoSig(id, uid, exp)), []byte(s))
}

// videoURL 返回视频对外地址（签名时效链接）。
func videoURL(v model.SubmissionVideo, uid int) string {
	return signedVideoURL(v.ID, uid)
}
