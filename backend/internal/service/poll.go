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
	LinkedSubmissionID uint      `json:"linked_submission_id,omitempty"` // 该投票已关联的投稿（供关联选择时禁用）
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
	if err != nil {
		return nil, 0, err
	}
	s.attachLinkedSubmission(views)
	return views, total, nil
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

// MaxPollsPerSubmission 读取“单篇投稿可关联投票上限”配置（限定 0~10）。
func (s *PollService) MaxPollsPerSubmission() int {
	n := s.store.Int("poll_max_per_submission", 3)
	if n < 0 {
		n = 0
	}
	if n > 10 {
		n = 10
	}
	return n
}

// SetSubmissionPolls 设置某投稿关联的投票（替换式）。
//
// 权限规则：
//   - 普通用户：仅能操作「自己」的投稿，且只能关联「自己发起」的投票；
//   - 管理员：可操作任意用户的投稿，并可关联任意用户（自己或他人）的投票；
//   - 通用约束：投票未截止、未被其他投稿占用，数量不超过配置上限。
//
// 越权访问：非本人投稿或关联他人投票时返回 403（CodeForbidden）并给出提示。
func (s *PollService) SetSubmissionPolls(actor *model.User, submissionID uint, pollIDs []uint) error {
	if !s.enabled() {
		return errs.New(errs.ErrValidation, fmt.Errorf("投票功能未开启"))
	}
	if actor == nil {
		return errs.New(errs.CodeUnauthorized, nil)
	}
	var sub model.Submission
	if err := s.db.First(&sub, submissionID).Error; err != nil {
		return errs.New(errs.CodeNotFound, nil)
	}
	if sub.UserID != actor.ID && !actor.IsAdmin() {
		return errs.New(errs.CodeForbidden, fmt.Errorf("仅投稿者本人或管理员可关联/取消关联投票"))
	}
	seen := map[uint]bool{}
	ids := make([]uint, 0, len(pollIDs))
	for _, id := range pollIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	max := s.MaxPollsPerSubmission()
	if len(ids) > max {
		return errs.New(errs.ErrValidation, fmt.Errorf("最多关联 %d 个投票", max))
	}
	if len(ids) > 0 {
		var polls []model.Poll
		if err := s.db.Where("id IN ?", ids).Find(&polls).Error; err != nil {
			return errs.New(errs.CodeInternal, err)
		}
		if len(polls) != len(ids) {
			return errs.New(errs.ErrValidation, fmt.Errorf("包含无效投票"))
		}
		now := time.Now()
		for _, p := range polls {
			if p.Deadline != nil && now.After(*p.Deadline) {
				return errs.New(errs.ErrValidation, fmt.Errorf("投票「%s」已截止，无法关联", p.Title))
			}
			if !actor.IsAdmin() && p.UserID != actor.ID {
				return errs.New(errs.CodeForbidden, fmt.Errorf("只能关联自己发起的投票（管理员可关联任意用户的投票）"))
			}
			var cnt int64
			s.db.Model(&model.SubmissionPoll{}).
				Where("poll_id = ? AND submission_id <> ?", p.ID, submissionID).Count(&cnt)
			if cnt > 0 {
				return errs.New(errs.ErrValidation, fmt.Errorf("投票「%s」已关联其他投稿", p.Title))
			}
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("submission_id = ?", submissionID).Delete(&model.SubmissionPoll{}).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if err := tx.Create(&model.SubmissionPoll{SubmissionID: submissionID, PollID: id, CreatedAt: time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SubmissionPolls 查询某投稿关联的投票视图（按关联顺序）。
func (s *PollService) SubmissionPolls(submissionID, viewerID uint) ([]PollView, error) {
	var links []model.SubmissionPoll
	if err := s.db.Where("submission_id = ?", submissionID).Order("id ASC").Find(&links).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	if len(links) == 0 {
		return []PollView{}, nil
	}
	ids := make([]uint, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.PollID)
	}
	var polls []model.Poll
	if err := s.db.Where("id IN ?", ids).Find(&polls).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	byID := map[uint]model.Poll{}
	for _, p := range polls {
		byID[p.ID] = p
	}
	ordered := make([]model.Poll, 0, len(polls))
	for _, l := range links {
		if p, ok := byID[l.PollID]; ok {
			ordered = append(ordered, p)
		}
	}
	return s.buildViews(ordered, viewerID)
}

// ListMine 当前用户发起的投票（供关联选择）；附带已关联投稿 ID。
func (s *PollService) ListMine(uid uint, limit int) ([]PollView, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var polls []model.Poll
	if err := s.db.Where("user_id = ?", uid).Order("created_at DESC").Limit(limit).Find(&polls).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	views, err := s.buildViews(polls, uid)
	if err != nil {
		return nil, err
	}
	s.attachLinkedSubmission(views)
	return views, nil
}

// attachLinkedSubmission 为投票视图填充其已关联的投稿 ID。
// 关联选择时据此禁用“已被其他投稿占用”的投票；对普通用户与其本人投票、
// 对管理员在“全部投票”选择场景下同样可用。
func (s *PollService) attachLinkedSubmission(views []PollView) {
	if len(views) == 0 {
		return
	}
	ids := make([]uint, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.ID)
	}
	var links []model.SubmissionPoll
	s.db.Where("poll_id IN ?", ids).Find(&links)
	byPoll := map[uint]uint{}
	for _, l := range links {
		byPoll[l.PollID] = l.SubmissionID
	}
	for i := range views {
		if sid, ok := byPoll[views[i].ID]; ok {
			views[i].LinkedSubmissionID = sid
		}
	}
}
