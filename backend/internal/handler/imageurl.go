package handler

import (
	"fmt"

	"tzwall/backend/internal/model"
)

// imageURL 返回图片对外地址（纯路径，无签名）。内容已审核且对象置私有，仅在经授权展示时返回该路径。
func imageURL(im model.SubmissionImage, uid int) string {
	return fmt.Sprintf("/api/images/%d", im.ID)
}

// videoURL 返回视频对外地址（纯路径，无签名）。
func videoURL(v model.SubmissionVideo, uid int) string {
	return fmt.Sprintf("/api/videos/%d", v.ID)
}

// signedImageURL 兼容旧调用签名，现直接返回纯路径（不再携带 e/u/s 查询参数）。
func signedImageURL(id uint, uid int) string {
	return fmt.Sprintf("/api/images/%d", id)
}

// signedVideoURL 兼容旧调用签名，现直接返回纯路径（不再携带 e/u/s 查询参数）。
func signedVideoURL(id uint, uid int) string {
	return fmt.Sprintf("/api/videos/%d", id)
}
