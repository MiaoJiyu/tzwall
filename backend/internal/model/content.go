package model

import (
	"time"
)

// 投稿状态
type SubmissionStatus string

const (
	StatusPending   SubmissionStatus = "pending"   // 正在审核
	StatusApproved  SubmissionStatus = "approved"  // 通过
	StatusRejected  SubmissionStatus = "rejected"  // 不通过
	StatusNeedEdit  SubmissionStatus = "need_edit" // 需要修改
	StatusWithdrawn SubmissionStatus = "withdrawn" // 作者已撤回（不公开，可修改后重新提交）
)

// 标签状态
type TagStatus string

const (
	TagApproved TagStatus = "approved"
	TagPending  TagStatus = "pending"
	TagRejected TagStatus = "rejected"
)

// 联系方式可见性
const (
	ContactAll   = "all"   // 所有人可见
	ContactLogin = "login" // 仅登录用户可见
)

// Tag 预选/自定义标签，需审核后可用。
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:64" json:"name"`
	Status    TagStatus `gorm:"size:20;default:pending;index" json:"status"`
	CreatedBy uint      `json:"created_by"`
	MergedTo  *uint     `json:"merged_to,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Submission 投稿主体。
type Submission struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	UserID    uint              `gorm:"index" json:"user_id"`
	User      *User             `gorm:"foreignKey:UserID" json:"-"`
	Anonymous bool              `gorm:"default:false" json:"anonymous"`
	Title     string            `gorm:"size:200" json:"title"`
	Content   string            `gorm:"type:text" json:"content"`
	Tags      []*Tag            `gorm:"many2many:submission_tags" json:"tags"`
	Images    []SubmissionImage `json:"images,omitempty"`
	Videos    []SubmissionVideo `json:"videos,omitempty"`

	Contact        string `gorm:"type:text" json:"contact,omitempty"`
	ContactVisible string `gorm:"size:12;default:login" json:"contact_visible"`

	Status    SubmissionStatus `gorm:"size:20;default:pending;index" json:"status"`
	AdminNote string           `gorm:"type:text" json:"admin_note,omitempty"` // 仅投稿人可见

	// AuditComment AI 审核给出的通过/拒绝原因，作为“审核评论”仅管理员在后台可见。
	AuditComment string `gorm:"type:text" json:"-"`

	AuditStatus string `gorm:"size:20;default:none;index" json:"-"` // 机审状态：none/pass/review/block

	LikeCount     int `gorm:"default:0" json:"like_count"`
	DislikeCount  int `gorm:"default:0" json:"dislike_count"`
	CommentCount  int `gorm:"default:0" json:"comment_count"`
	ViewCount     int `gorm:"default:0" json:"view_count"`
	RevisionCount int `gorm:"default:0" json:"revision_count"`

	// 审核期间保留原版：已通过帖子被编辑后，原内容仍在主页展示并标注“审核中”，
	// 新内容暂存于 Pending*，审核通过后覆盖主字段。
	PendingEdit    bool   `gorm:"default:false;index" json:"-"`
	PendingTitle   string `gorm:"size:200" json:"-"`
	PendingContent string `gorm:"type:text" json:"-"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SubmissionVideo 视频元数据（COS 私有存储），上传后由 COS 自动审核回调判定合规性。
type SubmissionVideo struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SubmissionID *uint     `gorm:"index" json:"submission_id"` // 上传时未关联投稿，允许为空
	StorageKey   string    `gorm:"size:255" json:"-"`
	Size         int64     `json:"size"`
	ContentType  string    `gorm:"size:64" json:"content_type"`
	AuditStatus  string    `gorm:"size:20;default:review" json:"audit_status"` // none/pass/review/block
	IsPrivate    bool      `gorm:"default:false" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// SubmissionImage 图片元数据（本地或 COS），落库便于代理与审核状态查询。
type SubmissionImage struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SubmissionID *uint     `gorm:"index" json:"submission_id"` // 上传时未关联投稿，允许为空
	StorageKey   string    `gorm:"size:255" json:"-"`
	Hash         string    `gorm:"size:64" json:"-"`
	Size         int64     `json:"size"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	ContentType  string    `gorm:"size:64" json:"content_type"`
	AuditStatus  string    `gorm:"size:20;default:none" json:"audit_status"` // none/pass/block
	IsPrivate    bool      `gorm:"default:false" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// SubmissionRevision 修改历史（“需要修改”状态重提时保留）。
type SubmissionRevision struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SubmissionID uint      `gorm:"index" json:"submission_id"`
	Title        string    `gorm:"size:200" json:"title"`
	Content      string    `gorm:"type:text" json:"content"`
	EditorID     uint      `json:"editor_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// Comment 评论，支持多级回复。软删除标记保留原文给管理员。
type Comment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SubmissionID uint      `gorm:"index" json:"submission_id"`
	UserID       uint      `gorm:"index" json:"user_id"`
	ParentID     *uint     `gorm:"index" json:"parent_id,omitempty"`
	Content      string    `gorm:"type:text" json:"content"`
	Deleted      bool      `gorm:"default:false" json:"deleted"`             // 用户删除标记
	AuditStatus  string    `gorm:"size:20;default:none" json:"audit_status"` // none/pass/block
	LikeCount    int       `gorm:"default:0" json:"like_count"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Children []*Comment `gorm:"-" json:"children,omitempty"`
	Username string     `gorm:"-" json:"username,omitempty"`
	Title    string     `gorm:"-" json:"title,omitempty"` // 作者称号（展示用）
}

// Reaction 点赞/点踩。同一用户对同一目标唯一态度。
type Reaction struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"uniqueIndex:idx_react" json:"user_id"`
	TargetType string    `gorm:"size:16;uniqueIndex:idx_react" json:"target_type"` // submission/comment
	TargetID   uint      `gorm:"uniqueIndex:idx_react" json:"target_id"`
	Attitude   int       `json:"attitude"` // 1 赞 / -1 踩
	CreatedAt  time.Time `json:"created_at"`
}
