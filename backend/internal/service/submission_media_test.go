package service

import (
	"testing"

	"tzwall/backend/internal/model"
)

// TestMediaNeedsManualReview 回归：仅「内容可疑(review)」才转人工；
// 审核通过、基础设施异常(failed)、未审(pending/none) 都不应误伤合规稿件。
func TestMediaNeedsManualReview(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want bool
	}{
		{"全部通过", []string{model.ModerationPass}, false},
		{"通过+审核失败(文件过大等)", []string{model.ModerationPass, model.ModerationFailed}, false},
		{"通过+未审", []string{model.ModerationPass, model.ModerationNone, model.ModerationPending}, false},
		{"仅审核失败", []string{model.ModerationFailed}, false},
		{"无媒体", nil, false},
		{"内容可疑", []string{model.ModerationPass, model.ModerationReview}, true},
		{"仅内容可疑", []string{model.ModerationReview}, true},
	}
	for _, c := range cases {
		if got := mediaNeedsManualReview(c.in); got != c.want {
			t.Fatalf("%s: mediaNeedsManualReview(%v)=%v, want %v", c.name, c.in, got, c.want)
		}
	}
}

// TestMediaHasBlock 回归：只有明确违规(block)才判定为需驳回。
func TestMediaHasBlock(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want bool
	}{
		{"全部通过", []string{model.ModerationPass}, false},
		{"审核失败不算违规", []string{model.ModerationFailed}, false},
		{"内容可疑不算违规", []string{model.ModerationReview}, false},
		{"明确违规", []string{model.ModerationPass, model.ModerationBlock}, true},
		{"无媒体", nil, false},
	}
	for _, c := range cases {
		if got := mediaHasBlock(c.in); got != c.want {
			t.Fatalf("%s: mediaHasBlock(%v)=%v, want %v", c.name, c.in, got, c.want)
		}
	}
}
