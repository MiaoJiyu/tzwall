package middleware

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/pkg/errs"
	"tzwall/backend/internal/pkg/response"
	"tzwall/backend/internal/service"
)

// IPBanMW IP 封禁：支持 IPv4/IPv6 单地址与 CIDR。
type IPBanMW struct {
	svc *service.PunishmentService
}

func NewIPBan(svc *service.PunishmentService) *IPBanMW {
	return &IPBanMW{svc: svc}
}

func (m *IPBanMW) Gate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := ClientIP(c)
		if ip == "" {
			c.Next()
			return
		}
		for _, b := range m.svc.ActiveIPBans() {
			if matchCIDR(b.CIDR, ip) {
				msg := "该 IP 已被封禁"
				if b.Reason != "" {
					msg += "，理由：" + b.Reason
				}
				if !b.Permanent && b.ExpiresAt != nil {
					msg += "，解封时间：" + b.ExpiresAt.Format("2006-01-02 15:04")
				} else if b.Permanent {
					msg += "（永久封禁）"
				}
				response.Fail(c, http.StatusForbidden, errs.ErrIPBanned, msg)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// matchCIDR 判断 IP 是否命中单地址或 CIDR 网段。
func matchCIDR(rule, ip string) bool {
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	if _, subnet, err := net.ParseCIDR(rule); err == nil {
		return subnet.Contains(addr)
	}
	if r := net.ParseIP(rule); r != nil {
		return r.Equal(addr)
	}
	return false
}
