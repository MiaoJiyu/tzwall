package model

import "time"

// 工单类型
type TicketType string

const (
	TicketTypeBug      TicketType = "bug"      // 功能故障
	TicketTypeFeedback TicketType = "feedback" // 意见反馈
	TicketTypeReport   TicketType = "report"   // 违规举报
	TicketTypeAppeal   TicketType = "appeal"   // 处罚申诉
	TicketTypeAccount  TicketType = "account"  // 账号/安全
	TicketTypeOther    TicketType = "other"    // 其他
)

// ValidTicketType 校验工单类型是否合法。
func ValidTicketType(t string) bool {
	switch TicketType(t) {
	case TicketTypeBug, TicketTypeFeedback, TicketTypeReport, TicketTypeAppeal, TicketTypeAccount, TicketTypeOther:
		return true
	}
	return false
}

// 工单状态
type TicketStatus string

const (
	TicketPending    TicketStatus = "pending"    // 待处理
	TicketProcessing TicketStatus = "processing" // 处理中
	TicketResolved   TicketStatus = "resolved"   // 已解决
	TicketRejected   TicketStatus = "rejected"   // 已驳回
	TicketClosed     TicketStatus = "closed"     // 已关闭（用户或管理员）
)

// TicketTransition 允许的状态流转（管理员驱动；用户关闭另走 CloseOwn）。
var TicketTransition = map[TicketStatus][]TicketStatus{
	TicketPending:    {TicketProcessing, TicketResolved, TicketRejected, TicketClosed},
	TicketProcessing: {TicketPending, TicketResolved, TicketRejected, TicketClosed},
	TicketResolved:   {TicketClosed},
	TicketRejected:   {},
	TicketClosed:     {},
}

// CanTicketTransition 判断 from→to 是否合法。
func CanTicketTransition(from, to TicketStatus) bool {
	for _, s := range TicketTransition[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Ticket 工单主体。
type Ticket struct {
	ID      uint `gorm:"primaryKey" json:"id"`
	UserID  uint `gorm:"index" json:"user_id"`
	Type    string `gorm:"size:20;index" json:"type"`
	Title   string `gorm:"size:200" json:"title"`
	Content string `gorm:"type:text" json:"content"`

	// 附件：复用图片上传，JSON 存储 [{id,url,content_type}]
	AttachmentsData string           `gorm:"type:text" json:"-"`
	Attachments     []TicketAttachment `gorm:"-" json:"attachments,omitempty"`

	Status     TicketStatus `gorm:"size:20;default:pending;index" json:"status"`
	HandlerID  uint         `gorm:"index" json:"handler_id"` // 最近处理人（管理员）
	ResolvedAt *time.Time   `json:"resolved_at"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 瞬态
	Username string        `gorm:"-" json:"username,omitempty"`
	Replies  []TicketReply `gorm:"-" json:"replies,omitempty"`
}

// TicketAttachment 工单附件（图片）。
type TicketAttachment struct {
	ID          uint   `json:"id"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
}

// TicketReply 工单处理记录（用户/管理员消息 + 状态变更系统备注）。
type TicketReply struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TicketID   uint      `gorm:"index" json:"ticket_id"`
	UserID     uint      `gorm:"index" json:"user_id"` // 0 = 系统
	IsAdmin    bool      `gorm:"default:false" json:"is_admin"`
	Content    string    `gorm:"type:text" json:"content"`
	StatusFrom string    `gorm:"size:20" json:"status_from,omitempty"`
	StatusTo   string    `gorm:"size:20" json:"status_to,omitempty"`
	CreatedAt  time.Time `json:"created_at"`

	Username string `gorm:"-" json:"username,omitempty"`
}
