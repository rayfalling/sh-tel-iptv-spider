package loghub

import (
	"sync"
	"testing"
)

// TestPushCancelRace 回归测试：日志投递与订阅者取消并发时，
// 原实现会向已关闭的 channel 发送（"send on closed channel" panic，
// 整个进程随之退出）。必须配合 -race 运行才有意义。
func TestPushCancelRace(t *testing.T) {
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// 持续写日志
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				push(Entry{Time: "t", Level: "info", Message: "hello"})
			}
		}
	}()

	// 反复订阅/取消，模拟管理面板 SSE 频繁连接与断开
	for i := 0; i < 500; i++ {
		ch, cancel := Subscribe()
		select {
		case <-ch:
		default:
		}
		cancel()
	}

	close(stop)
	wg.Wait()

	if n := SubscriberCount(); n != 0 {
		t.Fatalf("subscribers leaked: %d", n)
	}
}

// TestSubscribeCancelIdempotent cancel 重复调用不应 panic（重复 close）
func TestSubscribeCancelIdempotent(t *testing.T) {
	_, cancel := Subscribe()
	cancel()
	cancel()
}

// TestRecentBounds Recent 在各容量下不越界
func TestRecentBounds(t *testing.T) {
	for i := 0; i < 20; i++ {
		push(Entry{Time: "t", Level: "info", Message: "x"})
	}
	if got := len(Recent(5)); got != 5 {
		t.Fatalf("Recent(5) = %d", got)
	}
	if got := len(Recent(0)); got == 0 {
		t.Fatalf("Recent(0) should return all")
	}
	if got := len(Recent(1 << 20)); got == 0 {
		t.Fatalf("Recent(huge) should return all")
	}
}

func TestSetLevel(t *testing.T) {
	defer SetLevel("info")
	for _, s := range []string{"debug", "info", "warn", "warning", "error", " DEBUG "} {
		if err := SetLevel(s); err != nil {
			t.Fatalf("SetLevel(%q) = %v", s, err)
		}
	}
	if err := SetLevel("nope"); err == nil {
		t.Fatalf("SetLevel(nope) should fail")
	}
	if _, err := ParseLevel("debug"); err != nil {
		t.Fatalf("ParseLevel(debug) = %v", err)
	}
	if _, err := ParseLevel("nope"); err == nil {
		t.Fatalf("ParseLevel(nope) should fail")
	}
}
