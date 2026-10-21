package service

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"tzwall/backend/internal/model"
	"tzwall/backend/internal/pkg/errs"
)

// SongService 点歌业务。
type SongService struct {
	db    *gorm.DB
	store *ConfigStore
	words *WordFilterService
}

func NewSong(db *gorm.DB, store *ConfigStore) *SongService {
	return &SongService{db: db, store: store}
}

// WithWordFilter 注入词表服务。
func (s *SongService) WithWordFilter(w *WordFilterService) *SongService {
	s.words = w
	return s
}

// weekStartOf 返回给定时间所在自然周的周一 00:00（本地时区）。
func weekStartOf(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 { // 周日
		wd = 7
	}
	d := t.AddDate(0, 0, -(wd - 1))
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
}

func (s *SongService) enabled() bool { return s.store.Get("song_enabled") != "false" }

// List 公开点歌列表（分页，可按状态筛选）。
func (s *SongService) List(page, pageSize int, status string) ([]model.Song, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	q := s.db.Model(&model.Song{})
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	var total int64
	q.Count(&total)
	var songs []model.Song
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&songs).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	s.fillUsernames(songs)
	return songs, total, nil
}

// AdminList 后台列表（支持状态与关键词筛选）。
func (s *SongService) AdminList(page, pageSize int, status, keyword string) ([]model.Song, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	q := s.db.Model(&model.Song{})
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("title ILIKE ? OR artist ILIKE ? OR note ILIKE ?", like, like, like)
	}
	var total int64
	q.Count(&total)
	var songs []model.Song
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&songs).Error; err != nil {
		return nil, 0, errs.New(errs.CodeInternal, err)
	}
	s.fillUsernames(songs)
	return songs, total, nil
}

func (s *SongService) fillUsernames(songs []model.Song) {
	if len(songs) == 0 {
		return
	}
	ids := make([]uint, 0, len(songs))
	for _, x := range songs {
		ids = append(ids, x.UserID)
	}
	var users []model.User
	s.db.Select("id, username").Where("id IN ?", ids).Find(&users)
	m := make(map[uint]string, len(users))
	for _, u := range users {
		m[u.ID] = u.Username
	}
	for i := range songs {
		songs[i].Username = m[songs[i].UserID]
	}
}

// Quota 本周点歌配额。
func (s *SongService) Quota(uid uint) map[string]any {
	limit := s.store.Int("song_weekly_limit", 1)
	ws := weekStartOf(time.Now())
	var used int64
	s.db.Model(&model.Song{}).Where("user_id = ? AND created_at >= ?", uid, ws).Count(&used)
	remaining := -1 // 不限
	if limit > 0 {
		remaining = limit - int(used)
		if remaining < 0 {
			remaining = 0
		}
	}
	return map[string]any{
		"limit":      limit,
		"used":       used,
		"remaining":  remaining,
		"week_start": ws,
		"enabled":    s.enabled(),
	}
}

// Create 提交点歌（每周限次 + 违禁词过滤）。
func (s *SongService) Create(u *model.User, title, artist, note string) (*model.Song, error) {
	if !s.enabled() {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("点歌功能未开启"))
	}
	title = strings.TrimSpace(title)
	artist = strings.TrimSpace(artist)
	note = strings.TrimSpace(note)
	if title == "" || len([]rune(title)) > 100 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("歌名必填且不超过 100 字"))
	}
	if len([]rune(artist)) > 100 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("歌手不超过 100 字"))
	}
	if len([]rune(note)) > 200 {
		return nil, errs.New(errs.ErrValidation, fmt.Errorf("留言不超过 200 字"))
	}
	limit := s.store.Int("song_weekly_limit", 1)
	if limit > 0 {
		ws := weekStartOf(time.Now())
		var used int64
		s.db.Model(&model.Song{}).Where("user_id = ? AND created_at >= ?", u.ID, ws).Count(&used)
		if int(used) >= limit {
			return nil, errs.New(errs.ErrValidation, fmt.Errorf("本周点歌已达上限（%d 首），下周再来吧", limit))
		}
	}
	if s.words != nil {
		if err := s.words.CheckContent(title + " " + artist + " " + note); err != nil {
			return nil, err
		}
	}
	song := &model.Song{
		UserID:    u.ID,
		Title:     title,
		Artist:    artist,
		Note:      note,
		Status:    model.SongPending,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(song).Error; err != nil {
		return nil, errs.New(errs.CodeInternal, err)
	}
	song.Username = u.Username
	return song, nil
}

// SetStatus 状态流转（待播放/已播放/已取消）。
func (s *SongService) SetStatus(actor *model.User, id uint, status model.SongStatus) error {
	switch status {
	case model.SongPending, model.SongPlayed, model.SongCanceled:
	default:
		return errs.New(errs.ErrValidation, fmt.Errorf("非法状态"))
	}
	var song model.Song
	if err := s.db.First(&song, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if err := s.db.Model(&song).Update("status", status).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	if actor != nil {
		Audit(s.db, actor.ID, "song.status", "song", itoa(id), "status="+string(status))
	}
	return nil
}

// AdminDelete 删除点歌记录。
func (s *SongService) AdminDelete(actor *model.User, id uint) error {
	var song model.Song
	if err := s.db.First(&song, id).Error; err != nil {
		return errs.New(errs.ErrValidation, nil)
	}
	if err := s.db.Delete(&model.Song{}, id).Error; err != nil {
		return errs.New(errs.CodeInternal, err)
	}
	if actor != nil {
		Audit(s.db, actor.ID, "song.delete", "song", itoa(id), song.Title)
	}
	return nil
}
