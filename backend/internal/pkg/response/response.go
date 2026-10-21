package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tzwall/backend/internal/pkg/errs"
)

// Body 统一响应体。
type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	// Endpoint 用于无权限提示，前端展示“请求端点‘xxx’失败”
	Endpoint string `json:"endpoint,omitempty"`
}

func reqID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: errs.CodeOK, Message: errs.Text(errs.CodeOK), Data: data, RequestID: reqID(c)})
}

func Fail(c *gin.Context, httpStatus, code int, msg string) {
	if msg == "" {
		msg = errs.Text(code)
	}
	c.JSON(httpStatus, Body{Code: code, Message: msg, RequestID: reqID(c)})
}

// FailCode 仅传业务码，HTTP 状态按码推导。
func FailCode(c *gin.Context, code int, msg string) {
	Fail(c, httpStatusFor(code), code, msg)
}

// FailEndpoint 无权限提示，携带端点键。
func FailEndpoint(c *gin.Context, endpoint string) {
	c.JSON(http.StatusForbidden, Body{
		Code:      errs.ErrEndpointForbidden,
		Message:   errs.Text(errs.ErrEndpointForbidden),
		RequestID: reqID(c),
		Endpoint:  endpoint,
	})
}

func httpStatusFor(code int) int {
	switch code {
	case errs.CodeUnauthorized:
		return http.StatusUnauthorized
	case errs.CodeForbidden, errs.ErrEndpointForbidden:
		return http.StatusForbidden
	case errs.CodeNotFound:
		return http.StatusNotFound
	case errs.CodeTooMany:
		return http.StatusTooManyRequests
	default:
		return http.StatusOK
	}
}
