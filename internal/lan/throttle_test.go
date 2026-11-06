package lan

import (
	"net/http/httptest"
	"testing"
	"time"
)

// fakeClock 让限速断言不用真的等。
type fakeClock struct {
	t      time.Time
	slept  time.Duration
	sleeps int
}

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) sleep(d time.Duration) {
	c.slept += d
	c.sleeps++
	c.t = c.t.Add(d)
}

func newFakeThrottle(rate int) (*throttle, *fakeClock) {
	c := &fakeClock{t: time.Unix(0, 0)}
	return &throttle{rate: rate, now: c.now, sleep: c.sleep, start: c.t}, c
}

/**
 * 限速的核心不变量：发完 N 字节至少要花 N/rate 秒。
 *
 * 用假时钟断言，不真的 sleep——否则测一次 1 MB/s 就要等好几秒。
 */
func TestThrottle_WaitsForFullRate(t *testing.T) {
	th, clock := newFakeThrottle(1000) // 1000 bytes/s

	// 发 500 字节 → 应等 0.5s
	th.wait(500)
	if clock.slept != 500*time.Millisecond {
		t.Errorf("发 500 字节应等 500ms，实际 %v", clock.slept)
	}

	// 再发 500 字节（累计 1000）→ 总共应等 1s，已等 0.5s，再补 0.5s
	th.wait(1000)
	if clock.slept != time.Second {
		t.Errorf("累计 1000 字节应总共等 1s，实际 %v", clock.slept)
	}
}

/**
 * 不限速时一次都不该睡——这是「默认不限」的行为保证。
 */
func TestThrottle_OffNeverSleeps(t *testing.T) {
	th, clock := newFakeThrottle(0)
	th.wait(1 << 30)
	if clock.sleeps != 0 {
		t.Errorf("不限速时不该有任何等待，实际睡了 %d 次", clock.sleeps)
	}
}

/**
 * 时间已经过去了足够久时不该再补睡（比如客户端读得慢，天然就慢）。
 */
func TestThrottle_NoSleepWhenAlreadySlow(t *testing.T) {
	th, clock := newFakeThrottle(1000)
	// 假装已经过去 2 秒，此时发 500 字节（只需 0.5s）
	clock.t = clock.t.Add(2 * time.Second)
	th.wait(500)
	if clock.sleeps != 0 {
		t.Errorf("已经够慢了不该再睡，实际睡了 %v", clock.slept)
	}
}

/**
 * rateLimitedWriter 按累计写出量排队，且不改变写出内容。
 */
func TestRateLimitedWriter_ThrottlesByCumulativeBytes(t *testing.T) {
	rec := httptest.NewRecorder()
	th, clock := newFakeThrottle(1000)
	w := &rateLimitedWriter{ResponseWriter: rec, t: th}

	for i := 0; i < 4; i++ {
		if _, err := w.Write(make([]byte, 250)); err != nil {
			t.Fatal(err)
		}
	}

	if got := rec.Body.Len(); got != 1000 {
		t.Errorf("应写出 1000 字节，实际 %d", got)
	}
	// 4 × 250 字节 @ 1000 B/s = 1 秒
	if clock.slept != time.Second {
		t.Errorf("累计 1000 字节应等 1s，实际 %v", clock.slept)
	}
}

/**
 * 不限速时 wrapRateLimit 原样返回，不多套一层间接调用。
 */
func TestWrapRateLimit_OffReturnsSameWriter(t *testing.T) {
	srv := NewServer()
	srv.SetRateLimit(RateLimitOff)

	rec := httptest.NewRecorder()
	if got := srv.wrapRateLimit(rec); got != rec {
		t.Error("不限速时应原样返回原 writer")
	}
}

/**
 * 开限速之后返回的是包了一层的 writer，而且能正常写出。
 */
func TestWrapRateLimit_OnWrapsWriter(t *testing.T) {
	srv := NewServer()
	srv.SetRateLimit(RateLimit20MBps)

	rec := httptest.NewRecorder()
	w := srv.wrapRateLimit(rec)
	if _, ok := w.(*rateLimitedWriter); !ok {
		t.Fatalf("开限速时应返回 rateLimitedWriter，实际 %T", w)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("写出内容被改变了: %q", rec.Body.String())
	}
}

/** 负数被当成「不限」，不制造一个反向的限速。 */
func TestSetRateLimit_NegativeBecomesOff(t *testing.T) {
	srv := NewServer()
	srv.SetRateLimit(-1)
	if got := srv.GetRateLimit(); got != 0 {
		t.Errorf("负数应被归一成 0，实际 %d", got)
	}
}
