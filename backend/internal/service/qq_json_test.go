package service

import (
	"encoding/json"
	"testing"
)

// TestQQTokenRespStringFields 复现并验证报错场景：
// QQ 开放平台把 expires_in / error_code 以字符串返回，
// 此前反序列化到 int 会报 "cannot unmarshal string into ... of type int"。
func TestQQTokenRespStringFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "expires_in 为字符串(成功响应)",
			body: `{"access_token":"FE04xxxx","expires_in":"7776000","refresh_token":"88E4xxxx"}`,
		},
		{
			name: "expires_in 与 error_code 均为字符串(错误响应)",
			body: `{"error":"100010","error_description":"redirect uri is illegal","error_code":"100010"}`,
		},
		{
			name: "expires_in 为数字(兼容旧格式)",
			body: `{"access_token":"abc","expires_in":7776000,"refresh_token":"def"}`,
		},
		{
			name: "expires_in 为浮点字符串(容错)",
			body: `{"access_token":"abc","expires_in":"7776000.0","refresh_token":"def"}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r qqTokenResp
			if err := json.Unmarshal([]byte(c.body), &r); err != nil {
				t.Fatalf("反序列化失败: %v", err)
			}
			t.Logf("解析结果: access_token=%q expires_in=%d error_code=%d errmsg=%q",
				r.AccessToken, r.ExpiresIn.Int(), r.ErrorCode.Int(), r.ErrorMsg)
		})
	}
}

// TestQQMeRespStringError 验证 qqMeResp 的 error_code 字符串也能解析。
func TestQQMeRespStringError(t *testing.T) {
	body := `{"client_id":"APPID","openid":"OPENID123","error_code":"0"}`
	var r qqMeResp
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if r.Openid != "OPENID123" {
		t.Fatalf("openid 解析错误: %q", r.Openid)
	}
	if r.ErrorCode != 0 {
		t.Fatalf("error_code 应解析为 0，实际 %d", r.ErrorCode.Int())
	}
}
