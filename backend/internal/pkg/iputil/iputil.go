package iputil

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP 从 X-Forwarded-For（EdgeOne 开启真实 IP 传递）取可信客户端地址。
// trusted 为可信代理链（CIDR 或精确 IP）。取 XFF 中第一个非可信来源；
// 若全部可信则取最左（客户端）地址；否则回退 X-Real-IP / RemoteAddr。
func ClientIP(r *http.Request, trusted []string) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip == "" {
				continue
			}
			if isTrusted(ip, trusted) {
				continue
			}
			return ip
		}
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isTrusted(ip string, trusted []string) bool {
	parsed := net.ParseIP(ip)
	for _, t := range trusted {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if t == ip {
			return true
		}
		if strings.Contains(t, "/") {
			if _, cidr, err := net.ParseCIDR(t); err == nil && cidr.Contains(parsed) {
				return true
			}
		}
	}
	return false
}
