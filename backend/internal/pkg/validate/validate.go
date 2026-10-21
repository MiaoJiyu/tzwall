package validate

import (
	"regexp"
	"strings"
)

// usernameRe 6-12 位：英文大小写字母 + 数字 + 常见特殊符号。
var usernameRe = regexp.MustCompile(`^[A-Za-z0-9!@#$%^&*()_+\-=\[\]{}|;:,.<>?]{6,12}$`)

// ValidUsername 校验用户名格式。
func ValidUsername(u string) bool {
	return usernameRe.MatchString(u)
}

// HitsWordList 是否命中违禁/常用词表（包含匹配，不区分大小写）。
func HitsWordList(u string, words []string) bool {
	lower := strings.ToLower(u)
	for _, w := range words {
		if w == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// ValidEmail 简单邮箱格式校验。
func ValidEmail(e string) bool {
	return strings.Contains(e, "@") && len(e) <= 254
}
