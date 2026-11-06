package lan

import (
	"net/http"
	"time"
)

// 限速档位，单位 bytes/sec。0 表示不限。
//
// 只给档位不给自由输入：共享方要的是「别把我上传占满」，不是精确的 QoS 配置。
const (
	RateLimitOff    = 0
	RateLimit1MBps  = 1 * 1024 * 1024
	RateLimit5MBps  = 5 * 1024 * 1024
	RateLimit20MBps = 20 * 1024 * 1024
)

// throttle 让「已发送 N 字节」的实际耗时不少于 N/rate。
//
// 做法是按累计量排队而不是按单块限流：每次要写一块之前，先算出「发完这一块
// 总共应该用掉多少时间」，如果实际用时还不够就补睡差额。这样平均速率被钉在
// rate 上，也不会出现「前几块瞬间发完、然后长时间卡住」的锯齿。
//
// now / sleep 是字段而不是直接调 time 包，测试里塞假实现就能断言速率而不用
// 真的等 5 秒。
type throttle struct {
	rate  int
	now   func() time.Time
	sleep func(time.Duration)

	start time.Time
	sent  int64
}

func newThrottle(rate int) *throttle {
	return &throttle{
		rate:  rate,
		now:   time.Now,
		sleep: time.Sleep,
		start: time.Now(),
	}
}

// wait 在「发完 total 字节之后」这个假设下，补齐应该等待的时间。
func (t *throttle) wait(total int64) {
	if t.rate <= 0 || total <= 0 {
		return
	}

	// 期望耗时 = total / rate 秒。用整数运算避免浮点误差累积。
	want := time.Duration(float64(total) / float64(t.rate) * float64(time.Second))
	elapsed := t.now().Sub(t.start)
	if want > elapsed {
		t.sleep(want - elapsed)
	}
}

// rateLimitedWriter 把限速挂在写出这一层，所以对桌面端和浏览器端同时生效——
// 它们走的是同一条下载路径。
type rateLimitedWriter struct {
	http.ResponseWriter
	t *throttle
}

func (w *rateLimitedWriter) Write(p []byte) (int, error) {
	// 先按「写完这一块之后」的累计量排队，再真正写出。
	w.t.wait(w.t.sent + int64(len(p)))
	n, err := w.ResponseWriter.Write(p)
	w.t.sent += int64(n)
	return n, err
}

// wrapRateLimit 在限速开启时套一层 writer；不限速时原样返回，避免多一层
// 无谓的间接调用。
func (s *Server) wrapRateLimit(w http.ResponseWriter) http.ResponseWriter {
	s.mu.RLock()
	rate := s.rateLimit
	s.mu.RUnlock()

	if rate <= 0 {
		return w
	}
	return &rateLimitedWriter{ResponseWriter: w, t: newThrottle(rate)}
}
