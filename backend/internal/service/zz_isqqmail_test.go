package service

import "testing"

func TestIsQQMail(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"a@qq.com", true},
		{"A@QQ.COM", true},        // 不区分大小写
		{"user@Qq.Com", true},     // 混合大小写
		{"a@qq.com ", true},       // 域名带空格（TrimSpace）
		{"a@vip.qq.com", false},   // 子域名不算
		{"a@mail.qq.com", false},  // 子域名不算
		{"a@qq.com.evil", false},  // 伪装后缀
		{"a@qqcompany.com", false},// 变体
		{"a@163.com", false},
		{"plainaddress", false}, // 无 @
		{"", false},
		{"a@", false}, // 空域名
	}
	for _, c := range cases {
		if got := IsQQMail(c.in); got != c.want {
			t.Errorf("IsQQMail(%q)=%v want %v", c.in, got, c.want)
		}
	}
}
