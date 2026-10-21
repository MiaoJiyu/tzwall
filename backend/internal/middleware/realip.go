package middleware

import (
	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/pkg/iputil"
)

// RealIP 解析可信真实客户端 IP，写入 context 供鉴权与审计使用。
func RealIP(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := iputil.ClientIP(c.Request, cfg.TrustedProxies)
		c.Set("client_ip", ip)
		c.Next()
	}
}

// ClientIP 取中间件写入的 IP。
func ClientIP(c *gin.Context) string {
	if v, ok := c.Get("client_ip"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return c.ClientIP()
}
