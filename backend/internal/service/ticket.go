package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// TicketService 工单业务。
type TicketService struct {
	db    *gorm.DB
	store *ConfigStore
	words *WordFilterService
}

func NewTicket(db *gorm.DB, store *ConfigStore) *TicketService {
	return &TicketService{db: db, store: store}
}

// WithWordFilter 注入词表服务。
func (s *TicketService) WithWordFilter(w *WordFilterService) *TicketService {
	s.words = w
	return s
}

type CreateTicketReq struct {
	Type        string               `json:"type"`
	Title       string               `json:"title"`
	Content     string               `json:"content"`
	Attachments []model.TicketAttachment `json:"attachments"`
}

func encodeAttachments(as []model.TicketAttachment) string {
	if len(as) == 0 {
		return ""
	}
	b, err := json.Marshal(as)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeAttachments(data string) []model.TicketAttachment {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var as []model.TicketAttachment
	if err := json.Unmarshal([]byte(data), &as); err != nil {
		return nil
	}
	return as
}

// attachView 将附件数据的 JSON 列还原为视图字段。
func attachView(t *model.Ticket) {
	t.Attachments = decodeAttachments(t.AttachmentsData)
}

// Create 用户提交工单（待处理）。
func (s *TicketService) Create(u *model.User, req CreateTicketReq) (*model.Ticket, error) {
	if !model.ValidTicketType(req.Type) {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("请选择有效的工单类型"))
	}
	title := strings.TrimSpace(req.Title)
	content := strings.TrimSpace(req.Content)
	if title == "" || len([]rune(title)) > 100 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("标题必填且不超过 100 字"))
	}
	if content == "" {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("请填写工单描述"))
	}
	if len([]rune(content)) > 5000 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("描述不超过 5000 字"))
	}
	if len(req.Attachments) > 9 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("附件最多 9 张"))
	}
	if s.words != nil {
		if err := s.words.CheckContent(title + " " + content); err != nil {
			return nil, err
		}
	}
	t := &model.Ticket{
		UserID:          u.ID,
		Type:            req.Type,
		Title:           title,
		Content:         content,
		AttachmentsData: encodeAttachments(req.Attachments),
		Status:          model.TicketPending,
	}
	if err := s.db.Create(t).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	t.Username = u.Username
	attachView(t)
	return t, nil
}

// ListMine 我的工单（可按状态筛选）。
func (s *TicketService) ListMine(u *model.User, status string) ([]model.Ticket, error) {
	q := s.db.Model(&model.Ticket{}).Where("user_id = ?", u.ID)
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	var list []model.Ticket
	if err := q.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	for i := range list {
		attachView(&list[i])
	}
	return list, nil
}

// Get 工单详情（作者或管理员可见），含处理记录。
func (s *TicketService) Get(viewer *model.User, id uint) (*model.Ticket, error) {
	var t model.Ticket
	if err := s.db.First(&t, id).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, nil)
	}
	if viewer == nil || (viewer.ID != t.UserID && !viewer.IsAdmin()) {
		return nil, errs.New(errs.CodeForbidden, nil)
	}
	s.loadDetail(&t)
	return &t, nil
}

// CloseOwn 用户关闭自己的工单（待处理/处理中/已解决 → 已关闭）。
func (s *TicketService) CloseOwn(u *model.User, id uint) error {
	var t model.Ticket
	if err := s.db.First(&t, id).Error; err != nil {
		return errs.New(errs.CodeNotFound, nil)
	}
	if t.UserID != u.ID {
		return errs.New(errs.CodeForbidden, nil)
	}
	switch t.Status {
	case model.TicketPending, model.TicketProcessing, model.TicketResolved:
	default:
		return errs.New(errs.ErrValidation, fmt.Errorf("当前状态不可关闭"))
	}
	return s.applyStatus(u, &t, model.TicketClosed, "用户关闭工单")
}

// AdminList 后台工单列表（状态/类型/关键词筛选 + 分页）。
func (s *TicketService) AdminList(page, pageSize int, status, typ, keyword string) ([]model.Ticket, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	q := s.db.Model(&model.Ticket{})
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	if typ != "" && typ != "all" {
		q = q.Where("type = ?", typ)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("title ILIKE ? OR content ILIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var list []model.Ticket
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	s.fillUsernames(list)
	for i := range list {
		attachView(&list[i])
	}
	return list, total, nil
}

// Reply 管理员回复工单；待处理状态自动转为处理中。
func (s *TicketService) Reply(admin *model.User, id uint, content string) error {
	var t model.Ticket
	if err := s.db.First(&t, id).Error; err != nil {
		return errs.New(errs.CodeNotFound, nil)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return errs.New(errs.ErrValidation, fmt.Errorf("回复内容不能为空"))
	}
	if len([]rune(content)) > 2000 {
		return errs.New(errs.ErrValidation, fmt.Errorf("回复不超过 2000 字"))
	}
	if s.words != nil {
		if err := s.words.CheckContent(content); err != nil {
			return err
		}
	}
	now := time.Now()
	reply := &model.TicketReply{TicketID: t.ID, UserID: admin.ID, IsAdmin: true, Content: content, CreatedAt: now}
	if t.Status == model.TicketPending {
		reply.StatusFrom = string(model.TicketPending)
		reply.StatusTo = string(model.TicketProcessing)
	}
	if err := s.db.Create(reply).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	updates := map[string]any{"handler_id": admin.ID}
	if t.Status == model.TicketPending {
		updates["status"] = model.TicketProcessing
	}
	if err := s.db.Model(&model.Ticket{}).Where("id = ?", t.ID).Updates(updates).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	Audit(s.db, admin.ID, "ticket.reply", "ticket", itoa(id), "")
	return nil
}

// SetStatus 管理员流转工单状态（校验合法转换），记录系统备注与审计。
func (s *TicketService) SetStatus(admin *model.User, id uint, to model.TicketStatus) error {
	switch to {
	case model.TicketPending, model.TicketProcessing, model.TicketResolved, model.TicketRejected, model.TicketClosed:
	default:
		return errs.New(errs.ErrValidation, fmt.Errorf("非法目标状态"))
	}
	var t model.Ticket
	if err := s.db.First(&t, id).Error; err != nil {
		return errs.New(errs.CodeNotFound, nil)
	}
	if t.Status == to {
		return nil
	}
	if !model.CanTicketTransition(t.Status, to) {
		return errs.New(errs.ErrValidation, fmt.Errorf("不允许从「%s」变更为「%s」", statusText(t.Status), statusText(to)))
	}
	return s.applyStatus(admin, &t, to, "")
}

// applyStatus 执行状态变更：写系统备注、更新字段（解决时记录时间）、审计。
func (s *TicketService) applyStatus(actor *model.User, t *model.Ticket, to model.TicketStatus, sysNote string) error {
	from := t.Status
	now := time.Now()
	note := sysNote
	if note == "" {
		note = "状态变更为 " + statusText(to)
	}
	reply := &model.TicketReply{
		TicketID: t.ID, UserID: 0, IsAdmin: actor != nil && actor.IsAdmin(),
		Content: note, StatusFrom: string(from), StatusTo: string(to), CreatedAt: now,
	}
	updates := map[string]any{"status": to}
	if to == model.TicketResolved {
		updates["resolved_at"] = &now
	}
	if actor != nil && actor.IsAdmin() {
		updates["handler_id"] = actor.ID
	}
	if err := s.db.Create(reply).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	if err := s.db.Model(&model.Ticket{}).Where("id = ?", t.ID).Updates(updates).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	if actor != nil {
		Audit(s.db, actor.ID, "ticket.status", "ticket", itoa(t.ID), string(from)+"->"+string(to))
	}
	return nil
}

func (s *TicketService) loadDetail(t *model.Ticket) {
	attachView(t)
	var replies []model.TicketReply
	s.db.Where("ticket_id = ?", t.ID).Order("created_at ASC").Find(&replies)
	// 附带回复者用户名
	if len(replies) > 0 {
		ids := make([]uint, 0, len(replies))
		for _, r := range replies {
			if r.UserID > 0 {
				ids = append(ids, r.UserID)
			}
		}
		if len(ids) > 0 {
			var users []model.User
			s.db.Select("id, username").Where("id IN ?", ids).Find(&users)
			m := make(map[uint]string, len(users))
			for _, u := range users {
				m[u.ID] = u.Username
			}
			for i := range replies {
				replies[i].Username = m[replies[i].UserID]
			}
		}
	}
	t.Replies = replies
}

func (s *TicketService) fillUsernames(list []model.Ticket) {
	if len(list) == 0 {
		return
	}
	ids := make([]uint, 0, len(list))
	for _, t := range list {
		ids = append(ids, t.UserID)
	}
	var users []model.User
	s.db.Select("id, username").Where("id IN ?", ids).Find(&users)
	m := make(map[uint]string, len(users))
	for _, u := range users {
		m[u.ID] = u.Username
	}
	for i := range list {
		list[i].Username = m[list[i].UserID]
	}
}

func statusText(s model.TicketStatus) string {
	switch s {
	case model.TicketPending:
		return "待处理"
	case model.TicketProcessing:
		return "处理中"
	case model.TicketResolved:
		return "已解决"
	case model.TicketRejected:
		return "已驳回"
	case model.TicketClosed:
		return "已关闭"
	}
	return string(s)
}
