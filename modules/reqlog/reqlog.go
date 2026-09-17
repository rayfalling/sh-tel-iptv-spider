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
	if len(ring) > capacity {
		ring = append([]Record(nil), ring[len(ring)-capacity:]...)
	}
}

// Recent 返回全部记录，最新的在前
func Recent() []Record {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Record, 0, len(ring))
	for i := len(ring) - 1; i >= 0; i-- {
		out = append(out, ring[i])
	}
	return out
}
