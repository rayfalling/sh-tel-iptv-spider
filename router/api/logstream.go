package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"iptv-spider-sh/modules/loghub"

	"github.com/kataras/iris/v12"
)

// logStream GET /api/log/stream?level=info
// SSE 实时日志流：先回填最近 100 条，再推送实时日志，20 秒一次心跳。
func logStream(ctx iris.Context) {
	minLevel, err := loghub.ParseLevel(ctx.URLParamDefault("level", "info"))
	if err != nil {
		jsonErr(ctx, iris.StatusBadRequest, err.Error())
		return
	}

	ctx.Header("Content-Type", "text/event-stream; charset=utf-8")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Compress(false)

	w := ctx.ResponseWriter()
	flusher, ok := w.(interface{ Flush() })
	if !ok {
		jsonErr(ctx, iris.StatusInternalServerError, "当前连接不支持流式输出")
		return
	}

	// 回填历史
	for _, e := range loghub.Recent(100) {
		if !levelEnabled(e.Level, minLevel.String()) {
			continue
		}
		if err := writeSSE(w, e); err != nil {
			return
		}
	}
	flusher.Flush()

	ch, cancel := loghub.Subscribe()
	defer cancel()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	done := ctx.Request().Context().Done()
	for {
		select {
		case <-done:
			return
		case e, open := <-ch:
			if !open {
				return
			}
			if !levelEnabled(e.Level, minLevel.String()) {
				continue
			}
			if err := writeSSE(w, e); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w interface{ Write([]byte) (int, error) }, e loghub.Entry) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: log\ndata: %s\n\n", body)
	return err
}

// levelEnabled 按级别过滤（level 名称来自 zap）
func levelEnabled(entryLevel, minLevel string) bool {
	rank := func(s string) int {
		switch strings.ToLower(s) {
		case "debug":
			return 0
		case "info":
			return 1
		case "warn", "warning":
			return 2
		case "error", "dpanic", "panic", "fatal":
			return 3
		}
		return 1
	}
	return rank(entryLevel) >= rank(minLevel)
}
