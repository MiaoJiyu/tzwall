package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// ChatService 一对一私聊。
type ChatService struct {
	db *gorm.DB
}

func NewChat(db *gorm.DB) *ChatService {
	return &ChatService{db: db}
}

func convPair(a, b uint) (uint, uint) {
	if a < b {
		return a, b
	}
	return b, a
}

// Open 发起/获取与某用户的一对一会话。
func (s *ChatService) Open(meID, otherID uint) (*model.Conversation, error) {
	if meID == otherID {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("不能与自己发起会话"))
	}
	var ou model.User
	if err := s.db.First(&ou, otherID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, nil)
	}
	lo, hi := convPair(meID, otherID)
	var c model.Conversation
	if err := s.db.Where("user_low = ? AND user_high = ?", lo, hi).First(&c).Error; err != nil {
		c = model.Conversation{UserLow: lo, UserHigh: hi, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err := s.db.Create(&c).Error; err != nil {
			return nil, errs.New(errs.CodeInternal, err)
		}
	}
	return &c, nil
}

func (s *ChatService) otherOf(c *model.Conversation, meID uint) uint {
	if c.UserHigh == meID {
		return c.UserLow
	}
	return c.UserHigh
}

// List 我的会话列表（含对方信息、最后消息、未读数）。
func (s *ChatService) List(meID uint) ([]model.Conversation, error) {
	var convs []model.Conversation
	if err := s.db.Where("user_low = ? OR user_high = ?", meID, meID).Order("updated_at DESC").Find(&convs).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	if len(convs) == 0 {
		return convs, nil
	}
	ids := make([]uint, 0, len(convs))
	for i := range convs {
		ids = append(ids, s.otherOf(&convs[i], meID))
	}
	var users []model.User
	s.db.Select("id", "username", "nickname", "avatar").Where("id IN ?", ids).Find(&users)
	um := make(map[uint]*model.User, len(users))
	for i := range users {
		um[users[i].ID] = &users[i]
	}
	for i := range convs {
		convs[i].Other = um[s.otherOf(&convs[i], meID)]
		var last model.Message
		if err := s.db.Where("conversation_id = ?", convs[i].ID).Order("created_at DESC").First(&last).Error; err == nil {
			convs[i].LastMessage = last.Content
			t := last.CreatedAt
			convs[i].LastAt = &t
		}
		var unread int64
		s.db.Model(&model.Message{}).Where("conversation_id = ? AND sender_id <> ? AND read = ?", convs[i].ID, meID, false).Count(&unread)
		convs[i].Unread = int(unread)
	}
	return convs, nil
}

func (s *ChatService) ensureParticipant(meID, convID uint) (*model.Conversation, error) {
	var c model.Conversation
	if err := s.db.First(&c, convID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, nil)
	}
	if c.UserLow != meID && c.UserHigh != meID {
		return nil, errs.New(errs.CodeForbidden, nil)
	}
	return &c, nil
}

// Messages 会话消息（升序），并标记是否本人所发。
func (s *ChatService) Messages(meID, convID uint) ([]model.Message, error) {
	if _, err := s.ensureParticipant(meID, convID); err != nil {
		return nil, err
	}
	var msgs []model.Message
	if err := s.db.Where("conversation_id = ?", convID).Order("created_at ASC").Limit(500).Find(&msgs).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	for i := range msgs {
		msgs[i].Mine = msgs[i].SenderID == meID
	}
	return msgs, nil
}

// Send 发送消息。
func (s *ChatService) Send(meID, convID uint, content string) (*model.Message, error) {
	if _, err := s.ensureParticipant(meID, convID); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("消息不能为空"))
	}
	if len([]rune(content)) > 2000 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("消息过长"))
	}
	m := &model.Message{ConversationID: convID, SenderID: meID, Content: content, CreatedAt: time.Now()}
	if err := s.db.Create(m).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	s.db.Model(&model.Conversation{}).Where("id = ?", convID).Update("updated_at", time.Now())
	m.Mine = true
	return m, nil
}

// MarkRead 将会话中他人消息标记为已读。
func (s *ChatService) MarkRead(meID, convID uint) error {
	if _, err := s.ensureParticipant(meID, convID); err != nil {
		return err
	}
	return s.db.Model(&model.Message{}).
		Where("conversation_id = ? AND sender_id <> ? AND read = ?", convID, meID, false).
		Update("read", true).Error
}

// Unread 我的未读消息总数（头部角标）。
func (s *ChatService) Unread(meID uint) int64 {
	var n int64
	s.db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("messages.sender_id <> ? AND messages.read = ? AND (conversations.user_low = ? OR conversations.user_high = ?)", meID, false, meID, meID).
		Count(&n)
	return n
}
