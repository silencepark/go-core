// Package log 基于 Zap 的结构化日志封装，支持 trace_id 全链路追踪。
// Logger 通过依赖注入使用，不使用全局单例。
package log

import (
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/silencepark/go-core/config"
)

type ctxKey struct{}

// Logger 日志封装，聚合 zap.Logger 并提供 trace_id 上下文传递能力。
// 通过 New 创建，由 Wire 注入到各层。
type Logger struct {
	*zap.Logger
}

// New 根据日志配置创建 Logger。调用方应在启动早期调用一次，通过 DI 传递。
func New(cfg *config.Config) (*Logger, error) {
	logCfg := cfg.Log
	level := zapcore.InfoLevel
	if logCfg.Level != "" {
		if err := level.Set(logCfg.Level); err != nil {
			return nil, err
		}
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	logPath := logCfg.Path
	if logPath == "" {
		logPath = "logs/app.log"
	}
	_ = os.MkdirAll(filepath.Dir(logPath), 0755)

	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    logCfg.MaxSize,
		MaxBackups: logCfg.MaxBackups,
		MaxAge:     logCfg.MaxAge,
		Compress:   true,
	})

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), writer),
		level,
	)

	z := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return &Logger{Logger: z}, nil
}

// FromContext 从 context 提取 trace_id 并附加到 logger；无 trace 时返回自身。
func (l *Logger) FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}
	if traceID, ok := ctx.Value(ctxKey{}).(string); ok && traceID != "" {
		return &Logger{Logger: l.With(zap.String("trace_id", traceID))}
	}
	return l
}

// ContextWithTrace 将 trace_id 写入 context，供后续 FromContext 提取。
// 这是无状态的工具函数，保持为包级函数。
func ContextWithTrace(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, traceID)
}
