package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// PollService 投票业务。
type PollService struct {
	db    *gorm.DB
	store *ConfigStore
	words *WordFilterService
}

func NewPoll(db *gorm.DB, store *ConfigStore) *PollService {
	return &PollService{db: db, store: store}
}

// WithWordFilter 注入词表服务。
func (s *PollService) WithWordFilter(w *WordFilterService) *PollService {
	s.words = w
	return s
}

func (s *PollService) enabled() bool { return s.store.Get("poll_enabled") != "false" }

// PollCreateReq 创建投票请求。
type PollCreateReq struct {
	Title       string
	Description string
	Multiple    bool
	Pinned      bool
	Deadline    *time.Time
	Options     []string
}

// PollOptionView 选项视图（含实时票数与投票人）。
type PollOptionView struct {
	ID     uint     `json:"id"`
	Text   string   `json:"text"`
	Count  int64    `json:"count"`
	Voters []string `json:"voters"`
}

// PollView 投票视图。
type PollView struct {
	ID          uint             `json:"id"`
	UserID      uint             `json:"user_id"`
	Creator     string           `json:"creator"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Multiple    bool             `json:"multiple"`
	Pinned      bool             `json:"pinned"`
	Deadline    *time.Time       `json:"deadline"`
	CreatedAt   time.Time        `json:"created_at"`
	Expired     bool             `json:"expired"`
	TotalVotes  int64            `json:"total_votes"`
	Options     []PollOptionView `json:"options"`
	MyOptionIDs []uint           `json:"my_option_ids"`
}

// List 公开投票列表（置顶优先）。uid=0 表示未登录。
func (s *PollService) List(uid uint, page, pageSize int, keyword string) ([]PollView, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	q := s.db.Model(&model.Poll{})
	if kw := strings.TrimSpace(keyword); kw != "" {
		q = q.Where("title ILIKE ?", "%"+kw+"%")
	}
	var total int64
	q.Count(&total)
	var polls []model.Poll
	if err := q.Order("pinned DESC, created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&polls).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	views, err := s.buildViews(polls, uid)
	return views, total, err
}

// Detail 投票详情。
func (s *PollService) Detail(uid, pollID uint) (*PollView, error) {
	var poll model.Poll
	if err := s.db.First(&poll, pollID).Error; err != nil {
		return nil, errs.New(errs.CodeNotFound, nil)
	}
	views, err := s.buildViews([]model.Poll{poll}, uid)
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, errs.New(errs.CodeNotFound, nil)
	}
	return &views[0], nil
}

// Create 创建投票。isAdmin=true 时允许置顶。
func (s *PollService) Create(actor *model.User, req PollCreateReq, isAdmin bool) (*model.Poll, error) {
	if !s.enabled() {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("投票功能未开启"))
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || len([]rune(title)) > 100 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("标题必填且不超过 100 字"))
	}
	maxOpt := s.store.Int("poll_max_options", 10)
	seen := map[string]bool{}
	opts := make([]string, 0, len(req.Options))
	for _, o := range req.Options {
		o = strings.TrimSpace(o)
		if o == "" || seen[o] {
			continue
		}
		if len([]rune(o)) > 100 {
			return nil, errs.New(errs.ErrValidation, fmt.Errorf("单个选项不超过 100 字"))
		}
		seen[o] = true
		opts = append(opts, o)
	}
	if len(opts) < 1 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("至少需要一个选项"))
	}
	if len(opts) > maxOpt {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("选项数量不能超过 %d 个", maxOpt))
	}
	if req.Deadline != nil && !req.Deadline.After(time.Now()) {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("截止时间必须晚于当前时间"))
	}
	pinned := isAdmin && req.Pinned
	if s.words != nil {
		if err := s.words.CheckContent(title + " " + req.Description + " " + strings.Join(opts, " ")); err != nil {
			return nil, err
		}
	}
	poll := &model.Poll{
		UserID:      actor.ID,
		Title:       title,
		Description: strings.TrimSpace(req.Description),
		Multiple:    req.Multiple,
		Pinned:      pinned,
		Deadline:    req.Deadline,
		CreatedAt:   time.Now(),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(poll).Error; err != nil {
			return err
		}
		for i, text := range opts {
			if err := tx.Create(&model.PollOption{PollID: poll.ID, Text: text, Sort: i}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	return poll, nil
}

// Vote 投票 / 改票：覆盖该用户在此投票的全部旧票。
func (s *PollService) Vote(uid, pollID uint, optionIDs []uint) error {
	if !s.enabled() {
		return errs.New(errs.ErrValidation, fmt.Errorf("投票功能未开启"))
	}
	var poll model.Poll
	if err := s.db.First(&poll, pollID).Error; err != nil {
		return errs.New(errs.CodeNotFound, nil)
	}
	if poll.Deadline != nil && time.Now().After(*poll.Deadline) {
		return errs.New(errs.ErrValidation, fmt.Errorf("投票已截止"))
	}
	seen := map[uint]bool{}
	ids := make([]uint, 0, len(optionIDs))
	for _, id := range optionIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return errs.New(errs.ErrValidation, fmt.Errorf("请选择选项"))
	}
	if !poll.Multiple && len(ids) != 1 {
		return errs.New(errs.ErrValidation, fmt.Errorf("单选只能选择一个选项"))
	}
	var cnt int64
	s.db.Model(&model.PollOption{}).Where("poll_id = ? AND id IN ?", pollID, ids).Count(&cnt)
	if int(cnt) != len(ids) {
		return errs.New(errs.ErrValidation, fmt.Errorf("选项无效"))
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("poll_id = ? AND user_id = ?", pollID, uid).Delete(&model.PollVote{}).Error; err != nil {
			return err
		}
		for _, oid := range ids {
			if err := tx.Create(&model.PollVote{PollID: pollID, OptionID: oid, UserID: uid, CreatedAt: time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	return nil
}

// DeleteOwn 用户删除自己的投票（软删）。
func (s *PollService) DeleteOwn(actor *model.User, pollID uint) error {
	var poll model.Poll
	if err := s.db.First(&poll, pollID).Error; err != nil {
		return errs.New(errs.CodeNotFound, nil)
	}
	if poll.UserID != actor.ID {
		return errs.New(errs.CodeForbidden, nil)
	}
	if err := s.db.Delete(&poll).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	return nil
}

// AdminDelete 管理员强删投票（级联删除选项与投票记录）。
func (s *PollService) AdminDelete(actor *model.User, pollID uint) error {
	var poll model.Poll
	if err := s.db.Unscoped().First(&poll, pollID).Error; err != nil {
		return errs.New(errs.CodeNotFound, nil)
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("poll_id = ?", pollID).Delete(&model.PollVote{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("poll_id = ?", pollID).Delete(&model.PollOption{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&model.Poll{}, pollID).Error
	}); err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	if actor != nil {
		Audit(s.db, actor.ID, "poll.delete", "poll", itoa(pollID), poll.Title)
	}
	return nil
}

// buildViews 批量组装投票视图（避免 N+1）。
func (s *PollService) buildViews(polls []model.Poll, uid uint) ([]PollView, error) {
	views := make([]PollView, 0, len(polls))
	if len(polls) == 0 {
		return views, nil
	}
	ids := make([]uint, 0, len(polls))
	uidSet := map[uint]bool{}
	for _, p := range polls {
		ids = append(ids, p.ID)
		uidSet[p.UserID] = true
	}
	var options []model.PollOption
	if err := s.db.Where("poll_id IN ?", ids).Order("sort ASC, id ASC").Find(&options).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	var votes []model.PollVote
	if err := s.db.Where("poll_id IN ?", ids).Find(&votes).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	for _, v := range votes {
		uidSet[v.UserID] = true
	}
	uids := make([]uint, 0, len(uidSet))
	for id := range uidSet {
		uids = append(uids, id)
	}
	uname := map[uint]string{}
	if len(uids) > 0 {
		var users []model.User
		s.db.Select("id, username").Where("id IN ?", uids).Find(&users)
		for _, u := range users {
			uname[u.ID] = u.Username
		}
	}
	optsByPoll := map[uint][]model.PollOption{}
	for _, o := range options {
		optsByPoll[o.PollID] = append(optsByPoll[o.PollID], o)
	}
	votesByPoll := map[uint][]model.PollVote{}
	for _, v := range votes {
		votesByPoll[v.PollID] = append(votesByPoll[v.PollID], v)
	}
	now := time.Now()
	for _, p := range polls {
		pvotes := votesByPoll[p.ID]
		countByOpt := map[uint]int64{}
		votersByOpt := map[uint][]string{}
		myIDs := make([]uint, 0)
		for _, v := range pvotes {
			countByOpt[v.OptionID]++
			if n := uname[v.UserID]; n != "" {
				votersByOpt[v.OptionID] = append(votersByOpt[v.OptionID], n)
			}
			if uid != 0 && v.UserID == uid {
				myIDs = append(myIDs, v.OptionID)
			}
		}
		pv := PollView{
			ID:          p.ID,
			UserID:      p.UserID,
			Creator:     uname[p.UserID],
			Title:       p.Title,
			Description: p.Description,
			Multiple:    p.Multiple,
			Pinned:      p.Pinned,
			Deadline:    p.Deadline,
			CreatedAt:   p.CreatedAt,
			Expired:     p.Deadline != nil && now.After(*p.Deadline),
			TotalVotes:  int64(len(pvotes)),
			Options:     make([]PollOptionView, 0, len(optsByPoll[p.ID])),
			MyOptionIDs: myIDs,
		}
		for _, o := range optsByPoll[p.ID] {
			pv.Options = append(pv.Options, PollOptionView{
				ID:     o.ID,
				Text:   o.Text,
				Count:  countByOpt[o.ID],
				Voters: votersByOpt[o.ID],
			})
		}
		views = append(views, pv)
	}
	return views, nil
}
