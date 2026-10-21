package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// 写操作方法
func isWrite(method string) bool {
	switch strings.ToUpper(method) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

// 互动路径（禁言时禁止）：投稿、评论、表态、上传
func isInteractivePath(path string) bool {
	return path == "/api/submissions" ||
		strings.HasSuffix(path, "/comments") && strings.HasPrefix(path, "/api/submissions") ||
		strings.HasPrefix(path, "/api/comments") ||
		path == "/api/reactions" ||
		path == "/api/images" ||
		path == "/api/reports"
}

// 强制改密期间仍允许的路径（改密、查看自身、退出、处罚状态）
func allowedUnderForceReset(path string) bool {
	switch path {
	case "/api/auth/change-password", "/api/auth/me", "/api/auth/logout",
		"/api/punishments/status", "/api/punishments/mine", "/api/meta/routes", "/api/meta/site":
		return true
	}
	return strings.HasPrefix(path, "/api/punishments/")
}

// PunishmentMW 处罚门禁：封禁、强制改密、禁言、待确认警告。
type PunishmentMW struct {
	svc *service.PunishmentService
}

func NewPunishment(svc *service.PunishmentService) *PunishmentMW {
	return &PunishmentMW{svc: svc}
}

// Gate 在鉴权之后、权限判定之前执行。
func (p *PunishmentMW) Gate() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			c.Next()
			return
		}
		snap := p.svc.Snapshot(u.ID)
		path := c.FullPath()
		method := c.Request.Method

		// 封禁用户名：除退出登录外一律拒绝
		if snap.Banned && path != "/api/auth/logout" {
			response.Fail(c, http.StatusForbidden, errs.ErrBanned, "您已被封禁，理由见处罚记录")
			c.Abort()
			return
		}
		// 强制改密：仅允许改密相关路径
		if snap.ForceReset && !allowedUnderForceReset(path) {
			response.FailCode(c, errs.ErrForceReset, "")
			c.Abort()
			return
		}
		if isWrite(method) {
			// 禁言：禁止投稿/评论/点赞/分享/举报
			if snap.Muted && isInteractivePath(path) {
				response.FailCode(c, errs.ErrMuted, "")
				c.Abort()
				return
			}
			// 警告未确认：写操作前必须确认
			if snap.Warn != nil && path != "/api/punishments/:id/ack" {
				response.FailCode(c, errs.ErrWarnPending, "")
				c.Abort()
				return
			}
		}
		c.Set("punishment", snap)
		c.Next()
	}
}
