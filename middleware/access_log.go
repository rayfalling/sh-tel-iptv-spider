package middleware

import (
	"strings"
	"time"

	"iptv-spider-sh/global"
	"iptv-spider-sh/modules/reqlog"

	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
)

// AccessLog 访问日志中间件
//   - 写入 zap 访问日志（log/access/）
//   - 同时记录到内存环形缓冲，供 GET /api/requests 与状态页面展示
func AccessLog(ctx iris.Context) {
	method := ctx.Method()
	path := ctx.Path()
	clientIP := ctx.RemoteAddr()
	start := time.Now()

	ctx.Next()

	statusCode := ctx.GetStatusCode()
	durationMs := time.Since(start).Milliseconds()

	if global.ACCESS_LOG != nil {
		global.ACCESS_LOG.Info(
			"access",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.String("client_ip", clientIP),
			zap.Int64("duration_ms", durationMs),
		)
	}

	// SSE 是长连接：连接结束时记录会严重失真，也只会污染「最近请求」列表
	if strings.HasPrefix(path, "/api/log/stream") {
		return
	}

	reqlog.Add(reqlog.Record{
		Time:       time.Now().Format("2006-01-02 15:04:05"),
		Method:     method,
		Path:       path,
		Status:     statusCode,
		ClientIP:   clientIP,
		DurationMs: durationMs,
	})
}
