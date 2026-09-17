// Package utils 中的 panic 兜底工具。
//
// 背景：Iris 的 recover 中间件只覆盖 HTTP handler 链。下面这些执行路径没有任何 recover：
//   - robfig/cron 的定时任务（cron 自己起 goroutine）
//   - main / API 里手动起的后台 goroutine
//   - zap 的日志写入路径
//
// 任何一处 panic 都会直接终止整个进程（专网抖动、响应体异常等都可能触发）。
// 这里统一兜底：记录堆栈后继续运行，避免「一次坏响应 = 服务下线」。
package utils

import (
	"fmt"
	"runtime/debug"

	"iptv-spider-sh/global"
)

// SafeRun 执行 fn 并捕获其中的 panic（同步调用）
func SafeRun(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("[panic] %s: %v\n%s", name, r, debug.Stack())
			if global.LOG != nil {
				global.LOG.Error(msg)
			} else {
				fmt.Println(msg)
			}
		}
	}()
	fn()
}

// SafeGo 在独立 goroutine 中执行 fn 并捕获其中的 panic
func SafeGo(name string, fn func()) {
	go SafeRun(name, fn)
}
