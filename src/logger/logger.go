package logger

import (
	"log/slog"
	"os"
)

var l *slog.Logger

func init() {
	l = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(l)
}

func Create(msg string, args ...any) {
	l.Info(msg, append([]any{"event", "create"}, args...)...)
}

func Fail(msg string, args ...any) {
	l.Error(msg, append([]any{"event", "fail"}, args...)...)
}

func Confirm(msg string, args ...any) {
	l.Info(msg, append([]any{"event", "confirm"}, args...)...)
}
