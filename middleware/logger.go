package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/silencepark/go-core/log"
)

// LoggerMiddleware 返回 Gin 请求日志中间件，logger 由 DI 注入。
func LoggerMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Header("X-Trace-Id", traceID)
		c.Request = c.Request.WithContext(log.ContextWithTrace(c.Request.Context(), traceID))

		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		size := c.Writer.Size()
		reqLog := logger.FromContext(c.Request.Context())

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("ip", c.ClientIP()),
			zap.Duration("latency", latency),
			zap.Int("size", size),
			zap.String("user_agent", c.Request.UserAgent()),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
			reqLog.Error("request", fields...)
			return
		}
		if status >= 500 {
			reqLog.Error("request", fields...)
		} else if status >= 400 {
			reqLog.Warn("request", fields...)
		} else {
			reqLog.Info("request", fields...)
		}
	}
}
