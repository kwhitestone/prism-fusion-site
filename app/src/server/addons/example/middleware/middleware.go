package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"whitestone.top/prism-fusion/global"
)

// ExampleMiddleware 示例插件作用域中间件
// 框架会自动限定作用域，仅对本插件的路由生效，无需手动判断路径
func ExampleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录插件请求日志
		latency := time.Since(start)
		global.PRISM_LOG.Info("[ExamplePlugin] Request processed",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
		)
	}
}

// ExampleGlobalMiddleware 示例插件全局中间件
// 为所有请求添加 X-Request-ID 响应头，对所有路由生效
func ExampleGlobalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)

		global.PRISM_LOG.Info("[ExamplePlugin][Global] Request received",
			zap.String("request_id", requestID),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
		)

		c.Next()
	}
}

// generateRequestID 生成简单的请求ID
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
