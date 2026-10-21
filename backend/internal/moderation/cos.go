package moderation

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/config"
	"tzwall/backend/internal/model"
)

// ciJob 数据万象审核返回的单条任务结果。
type ciJob struct {
	JobId  string `json:"JobId"`
	State  string `json:"State"`
	Result int    `json:"Result"` // 0 正常，1 可疑，2 违规
	Label  string `json:"Label"`
}

// ciResponse 数据万象审核响应。
type ciResponse struct {
	JobsDetail []ciJob `json:"JobsDetail"`
}

// cosModeration 腾讯云数据万象内容审核（文本/图片），JSON 协议。
type cosModeration struct {
	cfg    *config.Config
	db     *gorm.DB
	client *http.Client
}

// NewCOSModeration 构造审核实现；凭证不完整返回 nil 由调用方降级。
func NewCOSModeration(cfg *config.Config, db *gorm.DB) *cosModeration {
	if cfg.CosBucket == "" || cfg.CosRegion == "" || cfg.CosSecretID == "" || cfg.CosSecretKey == "" {
		return nil
	}
	return &cosModeration{cfg: cfg, db: db, client: &http.Client{Timeout: 10 * time.Second}}
}

func (m *cosModeration) Configured() bool { return true }

// baseURL 数据万象（CI）服务域名。
func (m *cosModeration) baseURL() string {
	return fmt.Sprintf("https://%s.ci.%s.myqcloud.com", m.cfg.CosBucket, m.cfg.CosRegion)
}

// SubmitText 文本审核（同步；可疑、违规或失败一律转人工）。
func (m *cosModeration) SubmitText(ctx context.Context, bizID, text string) (Verdict, error) {
	body := map[string]any{"Input": map[string]any{
		"Content": base64.StdEncoding.EncodeToString([]byte(text)),
	}}
	if m.cfg.CosAuditBizType != "" {
		body["Conf"] = map[string]any{"BizType": m.cfg.CosAuditBizType}
	}
	raw, err := m.postJSON(ctx, "/text/auditing", body)
	if err != nil {
		m.record(bizID, "text", "", "", model.ModerationFailed, "", raw)
		return Verdict{Pass: false, Reason: "审核调用失败，转人工"}, err
	}
	v, jobID := parseCI(raw)
	m.record(bizID, "text", "", jobID, statusOf(v), v.Reason, raw)
	return v, nil
}

// SubmitImage 图片审核（按对象键）。
func (m *cosModeration) SubmitImage(ctx context.Context, bizID, key string) (Verdict, error) {
	body := map[string]any{"Input": map[string]any{"Object": key}}
	if m.cfg.CosAuditBizType != "" {
		body["Conf"] = map[string]any{"BizType": m.cfg.CosAuditBizType}
	}
	raw, err := m.postJSON(ctx, "/image/auditing", body)
	if err != nil {
		m.record(bizID, "image", key, "", model.ModerationFailed, "", raw)
		return Verdict{Pass: false, Reason: "审核调用失败，转人工"}, err
	}
	v, jobID := parseCI(raw)
	m.record(bizID, "image", key, jobID, statusOf(v), v.Reason, raw)
	return v, nil
}

// QueryJob 轮询查询异步任务结果。
func (m *cosModeration) QueryJob(ctx context.Context, jobID, kind string) (Verdict, error) {
	if kind == "" {
		kind = "image"
	}
	raw, err := m.get(ctx, "/"+kind+"/auditing/"+jobID)
	if err != nil {
		return Verdict{Pass: false, Reason: "轮询失败"}, err
	}
	v, _ := parseCI(raw)
	return v, nil
}

func (m *cosModeration) postJSON(ctx context.Context, path string, body any) (string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL()+path, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	m.signRequest(req, path)
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return string(out), fmt.Errorf("审核接口返回 %d: %s", resp.StatusCode, truncate(out, 200))
	}
	return string(out), nil
}

func (m *cosModeration) get(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL()+path, nil)
	if err != nil {
		return "", err
	}
	m.signRequest(req, path)
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return string(out), fmt.Errorf("查询接口返回 %d", resp.StatusCode)
	}
	return string(out), nil
}

// ciXML 数据万象 XML 响应（默认协议）。
type ciXML struct {
	XMLName    xml.Name `xml:"Response"`
	JobsDetail struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
		JobId   string `xml:"JobId"`
		State   string `xml:"State"`
		Result  string `xml:"Result"`
		Label   string `xml:"Label"`
	} `xml:"JobsDetail"`
}

// parseCI 解析审核响应：0 正常 / 1 可疑 / 2 违规；同时兼容 XML 与 JSON。
func parseCI(raw string) (Verdict, string) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "<") {
		var r ciXML
		if err := xml.Unmarshal([]byte(raw), &r); err != nil {
			return Verdict{Pass: false, Reason: "审核响应解析失败"}, ""
		}
		j := r.JobsDetail
		if j.Code != "" && j.Code != "Success" {
			return Verdict{Pass: false, Reason: "审核异常：" + j.Code + " " + j.Message}, ""
		}
		return verdictOf(atoi(j.Result), j.Label), j.JobId
	}
	var r ciResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return Verdict{Pass: false, Reason: "审核响应解析失败"}, ""
	}
	if len(r.JobsDetail) == 0 {
		return Verdict{Pass: false, Reason: "审核无结果"}, ""
	}
	j := r.JobsDetail[0]
	return verdictOf(j.Result, j.Label), j.JobId
}

// ciCallbackXML 数据万象 XML 回调。
type ciCallbackXML struct {
	XMLName    xml.Name `xml:"Response"`
	EventName  string   `xml:"EventName"`
	JobsDetail struct {
		JobId  string `xml:"JobId"`
		State  string `xml:"State"`
		Result string `xml:"Result"`
		Label  string `xml:"Label"`
	} `xml:"JobsDetail"`
}

// ciCallbackJSON 数据万象 JSON 回调。
type ciCallbackJSON struct {
	EventName  string `json:"EventName"`
	JobsDetail struct {
		JobId  string `json:"JobId"`
		State  string `json:"State"`
		Result int    `json:"Result"`
		Label  string `json:"Label"`
	} `json:"JobsDetail"`
}

// ParseCallback 解析审核回调内容（XML 或 JSON），返回 JobId 与判定。
func ParseCallback(body []byte) (jobID string, v Verdict, ok bool) {
	s := strings.TrimSpace(string(body))
	if strings.HasPrefix(s, "<") {
		var r ciCallbackXML
		if err := xml.Unmarshal(body, &r); err != nil {
			return "", Verdict{}, false
		}
		return r.JobsDetail.JobId, verdictOf(atoi(r.JobsDetail.Result), r.JobsDetail.Label), r.JobsDetail.JobId != ""
	}
	var r ciCallbackJSON
	if err := json.Unmarshal(body, &r); err != nil {
		return "", Verdict{}, false
	}
	return r.JobsDetail.JobId, verdictOf(r.JobsDetail.Result, r.JobsDetail.Label), r.JobsDetail.JobId != ""
}

// verdictOf 结果码转判定：0 正常 / 1 可疑 / 2 违规。
func verdictOf(result int, label string) Verdict {
	switch result {
	case 0:
		return Verdict{Pass: true, Reason: label}
	case 2:
		return Verdict{Pass: false, Blocked: true, Reason: label}
	default:
		return Verdict{Pass: false, Reason: label}
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range strings.TrimSpace(s) {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	if strings.TrimSpace(s) == "" {
		return -1
	}
	return n
}

// signRequest 腾讯云 XML API 签名（q-sign-algorithm=sha1）。
func (m *cosModeration) signRequest(req *http.Request, path string) {
	now := time.Now().Unix()
	keyTime := fmt.Sprintf("%d;%d", now, now+3600)
	signKey := hmacSHA1([]byte(m.cfg.CosSecretKey), keyTime)
	headerList, headerStr := buildList(map[string]string{"host": req.URL.Host})
	httpString := fmt.Sprintf("%s\n%s\n%s\n%s\n", strings.ToLower(req.Method), path, "", headerStr)
	h := sha1.Sum([]byte(httpString))
	stringToSign := fmt.Sprintf("sha1\n%s\n%s\n", keyTime, hex.EncodeToString(h[:]))
	signature := hmacSHA1([]byte(signKey), stringToSign)
	req.Header.Set("Authorization", fmt.Sprintf(
		"q-sign-algorithm=sha1&q-ak=%s&q-sign-time=%s&q-key-time=%s&q-header-list=%s&q-url-param-list=%s&q-signature=%s",
		m.cfg.CosSecretID, keyTime, keyTime, headerList, "", signature))
}

func hmacSHA1(key []byte, data string) string {
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}

// StatusOf 判定结果转任务状态（供上传后即时审核使用）。
func StatusOf(v Verdict) string { return statusOf(v) }

func statusOf(v Verdict) string {
	switch {
	case v.Blocked:
		return model.ModerationBlock
	case v.Pass:
		return model.ModerationPass
	default:
		return model.ModerationReview
	}
}

// record 记录审核任务（含原始响应，便于排查）。
func (m *cosModeration) record(bizID, kind, objectKey, jobID, status, label, raw string) {
	if m.db == nil {
		return
	}
	m.db.Create(&model.ModerationTask{
		BizID:     bizID,
		Kind:      kind,
		ObjectKey: objectKey,
		JobID:     jobID,
		Status:    status,
		Label:     label,
		RawResp:   truncate([]byte(raw), 2000),
		CreatedAt: time.Now(),
	})
}

// buildList 按小写键名排序生成签名的 header-list 与 header-str。
func buildList(headers map[string]string) (string, string) {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, strings.ToLower(k))
	}
	sort.Strings(keys)
	list := make([]string, 0, len(keys))
	val := make([]string, 0, len(keys))
	for _, k := range keys {
		list = append(list, k)
		val = append(val, k+"="+urlEncode(headers[k]))
	}
	return strings.Join(list, ";"), strings.Join(val, "&")
}

func urlEncode(s string) string {
	// 仅用于 header value 编码，保持简单
	return strings.ReplaceAll(s, " ", "%20")
}
