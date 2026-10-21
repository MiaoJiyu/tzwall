package moderation

import (
	"context"
)

// noneMod 未配置审核时的直通实现：一律通过，转人工审核兜底。
type noneMod struct{}

func NewNone() Moderation { return &noneMod{} }

func (m *noneMod) Configured() bool { return false }

func (m *noneMod) SubmitText(ctx context.Context, bizID, text string) (Verdict, error) {
	return Verdict{Pass: true}, nil
}

func (m *noneMod) SubmitImage(ctx context.Context, bizID, key string) (Verdict, error) {
	return Verdict{Pass: true}, nil
}

func (m *noneMod) QueryJob(ctx context.Context, jobID, kind string) (Verdict, error) {
	return Verdict{Pass: true}, nil
}
