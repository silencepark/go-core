package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MaxBodySize 限制请求体大小，防止 OOM。超过限制返回 413。
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cl := c.GetHeader("Content-Length"); cl != "" {
			if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > maxBytes {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
					"code":    41300,
					"message": "request body too large",
				})
				return
			}
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
