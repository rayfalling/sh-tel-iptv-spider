// Package loghub 承担两件事：
//  1. 持有可在运行期调整的 zap 日志级别（供 /api/admin/log-level 与 EPG 配置热更新使用）
//  2. 把结构化日志广播给 SSE 订阅者，并保留最近若干条供新连接回填
//
// 实现方式：包装 zapcore.Core，在编码落盘之前拿到 Entry，因此不需要解析日志文本。
package loghub

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ringCap      = 500
	subChanSize  = 256
	timeLayout   = "2006-01-02 15:04:05.000"
	LevelInvalid = "日志级别无效，可选值: debug/info/warn/error"
)

// Entry 一条日志（SSE 事件体）
type Entry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

var (
	mu          sync.RWMutex
	ring        = make([]Entry, 0, ringCap)
	subscribers = make(map[int]chan Entry)
	nextSubID   int
	level       = zap.NewAtomicLevelAt(zapcore.InfoLevel)
)

// AtomicLevel 供 zap 构建 core 时使用，使级别可在运行期生效
func AtomicLevel() zap.AtomicLevel { return level }

// Level 当前级别
func Level() zapcore.Level { return level.Level() }

// LevelString 当前级别名
func LevelString() string { return level.Level().String() }

// SetLevel 按名称设置级别（热生效）
func SetLevel(s string) error {
	var l zapcore.Level
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		l = zapcore.DebugLevel
	case "info":
		l = zapcore.InfoLevel
	case "warn", "warning":
		l = zapcore.WarnLevel
	case "error":
		l = zapcore.ErrorLevel
	default:
		return fmt.Errorf("%s", LevelInvalid)
	}
	level.SetLevel(l)
	return nil
}

// ParseLevel 解析级别名并返回对应级别（不改变当前级别）
func ParseLevel(s string) (zapcore.Level, error) {
	return parseLevel(s)
}

func parseLevel(s string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	}
	return zapcore.InfoLevel, fmt.Errorf("%s", LevelInvalid)
}

func push(e Entry) {
	mu.Lock()
	ring = append(ring, e)
	if len(ring) > ringCap {
		ring = append([]Entry(nil), ring[len(ring)-ringCap:]...)
	}
	targets := make([]chan Entry, 0, len(subscribers))
	for _, ch := range subscribers {
		targets = append(targets, ch)
	}
	mu.Unlock()

	for _, ch := range targets {
		// 订阅者积压时丢弃该条，绝不阻塞日志写入
		select {
		case ch <- e:
		default:
		}
	}
}

// Recent 返回最近 n 条（时间正序）
func Recent(n int) []Entry {
	mu.RLock()
	defer mu.RUnlock()
	if n <= 0 || n > len(ring) {
		n = len(ring)
	}
	out := make([]Entry, n)
	copy(out, ring[len(ring)-n:])
	return out
}

// Subscribe 订阅实时日志；返回的 cancel 必须被调用
func Subscribe() (<-chan Entry, func()) {
	mu.Lock()
	id := nextSubID
	nextSubID++
	ch := make(chan Entry, subChanSize)
	subscribers[id] = ch
	mu.Unlock()

	cancel := func() {
		mu.Lock()
		if c, ok := subscribers[id]; ok {
			delete(subscribers, id)
			close(c)
		}
		mu.Unlock()
	}
	return ch, cancel
}

// SubscriberCount 当前订阅者数量（供健康检查观察）
func SubscriberCount() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(subscribers)
}

// Core 包装 zapcore.Core，在落盘前把日志广播出去
type Core struct {
	zapcore.Core
}

// Wrap 用广播能力包装已有 core
func Wrap(c zapcore.Core) zapcore.Core {
	return &Core{Core: c}
}

func (c *Core) With(fields []zapcore.Field) zapcore.Core {
	return &Core{Core: c.Core.With(fields)}
}

func (c *Core) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *Core) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	t := ent.Time
	if t.IsZero() {
		t = time.Now()
	}
	push(Entry{
		Time:    t.Format(timeLayout),
		Level:   ent.Level.String(),
		Message: ent.Message,
	})
	return c.Core.Write(ent, fields)
}
