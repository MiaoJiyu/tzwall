package errs

// 业务错误码。HTTP 状态码与业务码分离，便于前端按 code 精确提示。
const (
	CodeOK           = 0
	CodeBadRequest   = 40000
	CodeUnauthorized = 40100
	CodeForbidden    = 40300
	CodeNotFound     = 40400
	CodeTooMany      = 42900
	CodeInternal     = 50000

	ErrInvalidUsername     = 41001
	ErrUserExists          = 41002
	ErrEmailExists         = 41003
	ErrBanned              = 41004
	ErrMuted               = 41005
	ErrEmailNotVerified    = 41006
	ErrInvalidCredentials  = 41007
	ErrAccountLocked       = 41008
	ErrTokenInvalid        = 41009
	ErrAgreementRequired   = 41010
	ErrEndpointForbidden   = 41011
	ErrWordFiltered        = 41012
	ErrTagPending          = 41013
	ErrSubmissionInvisible = 41014
	ErrCommentDeleted      = 41015
	ErrReactionConflict    = 41016
	ErrAlreadyReported     = 41017
	ErrImageTooLarge       = 41018
	ErrTagLimit            = 41019
	ErrValidation          = 41020
	ErrForceReset          = 41021
	ErrWarnPending         = 41022
	ErrIPBanned            = 41023
	ErrThirdPartyUnavailable = 41024 // 第三方登录未配置或不可用
	ErrIdentityConflict      = 41025 // 第三方账号已绑定到其他用户
	ErrSocialNoAccount       = 41026 // 第三方身份未关联本站账号（且未开启自动建号）
)

// Msg 业务码到默认文案的映射。
var Msg = map[int]string{
	CodeOK:           "ok",
	CodeBadRequest:   "请求参数错误",
	CodeUnauthorized: "未登录或登录已失效",
	CodeForbidden:    "权限不足",
	CodeNotFound:     "资源不存在",
	CodeTooMany:      "请求过于频繁，请稍后再试",
	CodeInternal:     "服务器内部错误",

	ErrInvalidUsername:     "用户名不符合规则（6-12 位，字母/数字/符号）",
	ErrUserExists:          "用户名已被注册",
	ErrEmailExists:         "邮箱已被注册",
	ErrBanned:              "您已被封禁",
	ErrMuted:               "您已被禁言",
	ErrEmailNotVerified:    "请先激活邮箱",
	ErrInvalidCredentials:  "用户名或密码错误",
	ErrAccountLocked:       "账号已锁定，请稍后再试",
	ErrTokenInvalid:        "登录凭证无效",
	ErrAgreementRequired:   "请先同意相关协议",
	ErrEndpointForbidden:   "请求端点失败，请联系系统管理员",
	ErrWordFiltered:        "内容包含违禁词",
	ErrTagPending:          "投稿含未审核标签，暂不能通过",
	ErrSubmissionInvisible: "该投稿当前不可查看",
	ErrCommentDeleted:      "评论已删除",
	ErrReactionConflict:    "态度已存在",
	ErrAlreadyReported:     "您已举报过该内容",
	ErrImageTooLarge:       "图片超过大小限制",
	ErrTagLimit:            "标签数量超过上限",
	ErrValidation:          "参数校验失败",
	ErrForceReset:          "请先修改密码后再继续操作",
	ErrWarnPending:         "请先确认警告后再继续操作",
	ErrIPBanned:            "该 IP 已被封禁",
	ErrThirdPartyUnavailable: "第三方登录未配置或暂不可用",
	ErrIdentityConflict:      "该第三方账号已绑定到其他用户",
	ErrSocialNoAccount:       "该第三方账号尚未关联本站账号，请先注册或用其他方式登录后绑定",
}

func Text(code int) string {
	if m, ok := Msg[code]; ok {
		return m
	}
	return "未知错误"
}

// AppError 携带业务码的错误，供 handler 精确提示。
type AppError struct {
	Code int
	Err  error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return Text(e.Code)
}

func New(code int, err error) *AppError {
	return &AppError{Code: code, Err: err}
}

// As 尝试提取 AppError 业务码。
func As(err error) (int, bool) {
	if e, ok := err.(*AppError); ok {
		return e.Code, true
	}
	return CodeInternal, false
}
