package bmc

import (
	"github.com/nsqio/go-nsq"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type nsqZapLogger struct {
	log *zap.SugaredLogger
}

func (n nsqZapLogger) Output(calldepth int, s string) error {
	n.log.Infow(s)
	return nil
}

func nsqMapLevel(log *zap.SugaredLogger) nsq.LogLevel {
	switch log.Desugar().Level() {
	case zapcore.DebugLevel, zapcore.DPanicLevel, zapcore.InvalidLevel:
		return nsq.LogLevelDebug
	case zapcore.InfoLevel:
		return nsq.LogLevelInfo
	case zapcore.WarnLevel:
		return nsq.LogLevelWarning
	case zapcore.ErrorLevel, zapcore.FatalLevel:
		return nsq.LogLevelError
	case zapcore.PanicLevel:
		return nsq.LogLevelMax
	default:
		return nsq.LogLevelDebug
	}
}
