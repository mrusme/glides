package runtime

import (
	"log/slog"
)

type AsyncLogger struct {
	logger *slog.Logger
}

func NewAsyncLogger(logger *slog.Logger) (l AsyncLogger) {
	l = AsyncLogger{}

	l.logger = logger

	return l
}

func (l AsyncLogger) fixArgs(args ...any) []any {
	if len(args)%2 != 0 {
		args = append([]any{"args"}, args...)
	}
	return args
}

func (l AsyncLogger) Debug(args ...any) {
	l.logger.Debug("Worker.Async", l.fixArgs(args)...)
}

func (l AsyncLogger) Info(args ...any) {
	l.logger.Info("Worker.Async", l.fixArgs(args)...)
}

func (l AsyncLogger) Warn(args ...any) {
	l.logger.Warn("Worker.Async", l.fixArgs(args)...)
}

func (l AsyncLogger) Error(args ...any) {
	l.logger.Error("Worker.Async", l.fixArgs(args)...)
}

func (l AsyncLogger) Fatal(args ...any) {
	l.logger.Error("Worker.Async", l.fixArgs(args)...)
}
