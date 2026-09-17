package loghub

import (
	"runtime"
	"sync"
	"testing"
)

// TestPushCancelRace 回归测试：日志投递与订阅者取消并发时的
// "send on closed channel" panic。
//
// 打挂路径：SSE 客户端断开 → cancel() 在锁内 close(ch)；
// 与此同时日志写入路径 push() 已经拿到订阅者快照并在锁外发送 → panic → 整个进程退出
// （日志写入发生在任何一次 LOG.xxx 调用里，所以这是随时可触发的）。
//
// 参数说明（在 1 核虚拟机上实测标定）：
//   - 该窗口只在真正并行（P > 1）时才可能命中。单核机器上 Go 默认 GOMAXPROCS=1，
//     push 从 Unlock 到 send 之间不会出现调度点，测试就永远抓不到旧实现的问题，
//     所以这里显式把 GOMAXPROCS 抬到 4。
//   - 2 个写入 goroutine + 500 次订阅/取消：旧实现 5/5 稳定 panic，
//     新实现约 1~2 秒通过；再少就抓不到了（1 写入+300 次时旧实现 0/3）。
//
// go test -short 可跳过（该用例是并发压力测试，比较吃 CPU）。
func TestPushCancelRace(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: 跳过并发压力测试")
	}
	prev := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(prev)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
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
	}

	// 反复订阅/取消，模拟管理面板 SSE 频繁连接与断开
	for i := 0; i < 500; i++ {
		ch, cancel := Subscribe()
		runtime.Gosched()
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

// TestPushRingBounded 环形缓冲：对外最多暴露 ringCap 条，内部也不能无限增长。
// （原实现每写一条日志就重新分配并复制整个缓冲区；现在改为长到 2*ringCap 才搬移一次。）
func TestPushRingBounded(t *testing.T) {
	for i := 0; i < 5*ringCap; i++ {
		push(Entry{Time: "t", Level: "info", Message: "x"})
	}
	if got := len(Recent(0)); got != ringCap {
		t.Fatalf("Recent(0) = %d, want %d", got, ringCap)
	}
	if got := len(Recent(10 * ringCap)); got != ringCap {
		t.Fatalf("Recent(huge) = %d, want %d", got, ringCap)
	}
	mu.RLock()
	backing := len(ring)
	mu.RUnlock()
	if backing >= 2*ringCap {
		t.Fatalf("backing ring grew to %d, want < %d", backing, 2*ringCap)
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
