// Package reqlog 保存最近若干条 API 访问记录，供 GET /api/requests 与状态页面展示。
package reqlog

import "sync"

const capacity = 50

// Record 一条访问记录
type Record struct {
	Time       string `json:"time"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	ClientIP   string `json:"client_ip"`
	DurationMs int64  `json:"duration_ms"`
}

var (
	mu   sync.RWMutex
	ring = make([]Record, 0, capacity)
)

// Add 追加一条记录（超出容量丢弃最旧的）
func Add(r Record) {
	mu.Lock()
	defer mu.Unlock()
	ring = append(ring, r)
	// 与 loghub.push 同理：只在长到容量两倍时搬移一次，
	// 避免缓冲区满之后每来一个请求都复制整个切片（内存上限 2*capacity）
	if len(ring) >= 2*capacity {
		ring = append([]Record(nil), ring[len(ring)-capacity:]...)
	}
}

// Recent 返回全部记录，最新的在前（最多 capacity 条）
func Recent() []Record {
	mu.RLock()
	defer mu.RUnlock()
	n := len(ring)
	if n > capacity {
		n = capacity
	}
	out := make([]Record, 0, n)
	for i := len(ring) - 1; i >= len(ring)-n; i-- {
		out = append(out, ring[i])
	}
	return out
}
