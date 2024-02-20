package bmc

import (
	"context"
	"log/slog"

	"github.com/nsqio/go-nsq"
)

type nsqLogger struct {
	log *slog.Logger
}

func (n nsqLogger) Output(calldepth int, s string) error {
	n.log.Info(s)
	return nil
}

func nsqMapLevel(log *slog.Logger) nsq.LogLevel {
	ctx := context.Background()
	if log.Enabled(ctx, slog.LevelDebug) {
		return nsq.LogLevelDebug
	}
	if log.Enabled(ctx, slog.LevelInfo) {
		return nsq.LogLevelInfo
	}
	if log.Enabled(ctx, slog.LevelError) {
		return nsq.LogLevelError
	}
	if log.Enabled(ctx, slog.LevelWarn) {
		return nsq.LogLevelWarning
	}
	return nsq.LogLevelInfo
}
