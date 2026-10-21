package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tzwall/backend/internal/cache"
)

// Settings AI 审核配置快照（由调用方从系统配置解密后组装）。
type Settings struct {
	Enabled    bool
	BaseURL    string
	APIKey     string
	Model      string
	Prompt     string
	Timeout    time.Duration
	DailyLimit int
}

// SettingsFunc 配置快照读取函数；每次审核时调用，实现保存即生效。
type SettingsFunc func() Settings

// Client OpenAI 兼容的文本审核客户端。
type Client struct {
	settings SettingsFunc
	cache    cache.Cache
	client   *http.Client
}

// NewClient 构造 AI 审核客户端。
func NewClient(fn SettingsFunc, c cache.Cache) *Client {
	return &Client{settings: fn, cache: c, client: &http.Client{}}
}

// Configured 是否已启用且配置完整。
func (c *Client) Configured() bool {
	s := c.settings()
	return s.Enabled && s.BaseURL != "" && s.APIKey != ""
}

// Review 审核文本。返回 error 表示「异常」（转人工），非异常时不代表放行。
func (c *Client) Review(ctx context.Context, text string) (Verdict, error) {
	s := c.settings()
	if !s.Enabled || s.BaseURL == "" || s.APIKey == "" {
		return Verdict{}, ErrNotConfigured
	}
	if err := c.checkQuota(s); err != nil {
		return Verdict{}, err
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body := map[string]any{
		"model":       s.Model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": s.Prompt},
			{"role": "user", "content": text},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Verdict{}, err
	}
	// 归一化 BaseURL：用户可填完整端点（含 /chat/completions）或仅填到 /v1
	base := strings.TrimSuffix(s.BaseURL, "/")
	base = strings.TrimSuffix(base, "/chat/completions")
	url := base + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Verdict{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return Verdict{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return Verdict{}, err
	}
	if resp.StatusCode >= 300 {
		return Verdict{}, fmt.Errorf("AI 接口返回 %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	v, err := parseCompletion(raw)
	v.Raw = truncate(string(raw), 2000)
	return v, err
}

// checkQuota 每日调用上限（超限按异常处理，转人工）。
func (c *Client) checkQuota(s Settings) error {
	if s.DailyLimit <= 0 || c.cache == nil {
		return nil
	}
	day := time.Now().Format("20060102")
	n := c.cache.Incr("ai_audit:count:"+day, 25*time.Hour)
	if int(n) > s.DailyLimit {
		return ErrQuotaExceeded
	}
	return nil
}

// chatResponse OpenAI 兼容响应。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// parseCompletion 解析模型输出：要求 JSON {"pass":bool,"reason":string}。
func parseCompletion(raw []byte) (Verdict, error) {
	var r chatResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return Verdict{}, fmt.Errorf("AI 响应解析失败")
	}
	if r.Error != nil && r.Error.Message != "" {
		return Verdict{}, fmt.Errorf("AI 接口错误：%s", truncate(r.Error.Message, 200))
	}
	if len(r.Choices) == 0 {
		return Verdict{}, fmt.Errorf("AI 响应缺少结果")
	}
	content := strings.TrimSpace(r.Choices[0].Message.Content)
	// 模型可能包裹 ```json 代码块
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	content = strings.TrimSpace(content)
	// 模型可能在 JSON 前后夹带说明文字，提取首个 { 到末尾 } 之间的内容
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	var out struct {
		Pass   bool   `json:"pass"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return Verdict{}, fmt.Errorf("AI 输出不是预期 JSON：%s", truncate(content, 120))
	}
	return Verdict{Pass: out.Pass, Reason: truncate(out.Reason, 200)}, nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
