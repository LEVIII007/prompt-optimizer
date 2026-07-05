package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Zlog *zap.Logger

// InitLogger sets up a JSON-to-stdout zap logger at the given level and
// returns a cleanup func that should be deferred to flush on exit.
func InitLogger(logLevel string) func() {
	if logLevel == "" {
		logLevel = "info"
	}

	var lvl zapcore.Level
	_ = lvl.Set(logLevel)

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	stdoutCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(os.Stdout),
		lvl,
	)

	Zlog = zap.New(stdoutCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return func() { _ = Zlog.Sync() }
}

func Logger() *zap.Logger {
	if Zlog != nil {
		return Zlog
	}
	return zap.NewNop()
}
