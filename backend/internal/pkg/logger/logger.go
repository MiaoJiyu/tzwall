package logger

import (
	"log/slog"
	"os"
)

// New 返回结构化 JSON 日志器，输出到 stdout 由 systemd journal 接管。
func New() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
