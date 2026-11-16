package api

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
)

// Keep the original fresh-slice algorithm independent of the storage helper.
// Optimization must preserve decisions, Retry-After, order and key isolation,
// including timestamps sampled out of order before the limiter lock is taken.
func freshSliceRateLimitReference(attempts []time.Time, limit int, window time.Duration, now time.Time) ([]time.Time, bool, time.Duration) {
	var valid []time.Time
	for _, attempt := range attempts {
		if attempt.After(now.Add(-window)) {
			valid = append(valid, attempt)
		}
	}
	if len(valid) >= limit {
		retry := valid[0].Add(window).Sub(now)
		if retry < time.Second {
			retry = time.Second
		}
		return valid, false, retry
	}
	return append(valid, now), true, 0
}

func TestRateLimiterWindowReferenceEquivalence(t *testing.T) {
	for _, limit := range []int{1, 3, 10, 500} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			rl := NewRateLimiter(limit, time.Minute)
			rl.Stop() // deterministic clock; cleanup is tested separately
			reference := make(map[string][]time.Time)
			keys := []string{"", "ip-a", "ip-b", "org-a", "org-b"}
			base := time.Unix(1_700_000_000, 0)
			// An expired timestamp in the middle rules out prefix-only pruning.
			seed := []time.Time{base.Add(-time.Second), base.Add(-time.Minute), base.Add(time.Second), base.Add(-2 * time.Minute)}
			rl.attempts[keys[0]] = slices.Clone(seed)
			reference[keys[0]] = slices.Clone(seed)
			rng := rand.New(rand.NewSource(42))
			now := base
			for i := 0; i < 4000; i++ {
				if i%251 == 0 && i != 0 {
					now = now.Add(2 * rl.window)
				} else {
					now = now.Add(time.Millisecond)
				}
				observed := now
				if i%7 == 0 {
					observed = now.Add(-rl.window / 2)
				}
				key := keys[rng.Intn(len(keys))]
				if i == 0 {
					key, observed = keys[0], base
				}
				wantAttempts, wantAllowed, wantRetry := freshSliceRateLimitReference(reference[key], limit, rl.window, observed)
				reference[key] = wantAttempts
				allowed, retry := rl.allowAt(key, observed)
				if allowed != wantAllowed || retry != wantRetry {
					t.Fatalf("step %d: allowed/retry = %v/%v, want %v/%v", i, allowed, retry, wantAllowed, wantRetry)
				}
				for _, caller := range keys {
					if !slices.Equal(rl.attempts[caller], reference[caller]) {
						t.Fatalf("step %d: caller %q timestamp inventory differs", i, caller)
					}
				}
			}
		})
	}
}

func TestRateLimiterWindowExpiryAndDeniedRetries(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	rl.Stop()
	base := time.Unix(1_700_000_000, 0)
	for _, offset := range []time.Duration{0, 10 * time.Second} {
		if allowed, _ := rl.allowAt("caller", base.Add(offset)); !allowed {
			t.Fatal("initial budget refused")
		}
	}
	for _, tc := range []struct {
		offset time.Duration
		allow  bool
		retry  time.Duration
	}{
		{20 * time.Second, false, 40 * time.Second},
		{59*time.Second + 999*time.Millisecond, false, time.Second},
		{time.Minute, true, 0},                 // exactly at cutoff expires, not one tick later
		{time.Minute, false, 10 * time.Second}, // denied requests did not extend the window
		{70 * time.Second, true, 0},
	} {
		allowed, retry := rl.allowAt("caller", base.Add(tc.offset))
		if allowed != tc.allow || retry != tc.retry {
			t.Fatalf("at %v: allowed/retry = %v/%v, want %v/%v", tc.offset, allowed, retry, tc.allow, tc.retry)
		}
	}
	if !rl.Allow("other") {
		t.Fatal("one caller consumed another caller's budget")
	}
}

func TestRateLimiterWindowCleanupRetainsAllLiveEntries(t *testing.T) {
	rl := NewRateLimiter(5, time.Hour)
	rl.Stop()
	now := time.Now()
	live := []time.Time{now, now.Add(time.Minute)}
	rl.attempts["mixed"] = []time.Time{live[0], now.Add(-2 * time.Hour), live[1]}
	rl.attempts["expired"] = []time.Time{now.Add(-2 * time.Hour)}
	rl.attempts["empty"] = nil
	rl.cleanup()
	if len(rl.attempts) != 1 || !slices.Equal(rl.attempts["mixed"], live) {
		t.Fatalf("cleanup did not preserve only the ordered live inventory: %v", rl.attempts)
	}
}

func TestRateLimiterWindowReusesStorage(t *testing.T) {
	for _, limit := range []int{10, 500, 2000} {
		for _, mode := range []string{"denied", "sliding", "cleanup"} {
			t.Run(fmt.Sprintf("%d/%s", limit, mode), func(t *testing.T) {
				rl, now, step := newWindowStorageFixture(limit, mode)
				allocs := testing.AllocsPerRun(100, func() {
					if mode == "cleanup" {
						rl.cleanup()
						return
					}
					now = now.Add(step)
					allowed, _ := rl.allowAt("caller", now)
					if allowed != (mode == "sliding") {
						t.Fatal("storage fixture changed enforcement")
					}
				})
				if allocs != 0 {
					t.Fatalf("steady %s checks allocate %g times per call, want zero", mode, allocs)
				}
			})
		}
	}
}

func TestRateLimiterWindowConcurrentCleanup(t *testing.T) {
	rl := NewRateLimiter(64, time.Hour)
	defer rl.Stop()
	var wg sync.WaitGroup
	start := make(chan struct{})
	counts := make([]int, 8)
	for worker := range counts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("caller-%d", worker%2)
			<-start
			for range 200 {
				if rl.Allow(key) {
					counts[worker]++
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for range 200 {
			rl.cleanup()
		}
	}()
	close(start)
	wg.Wait()
	total := 0
	for _, n := range counts {
		total += n
	}
	if total != 128 {
		t.Fatalf("concurrent cleanup allowed %d requests, want exactly 128 across two keys", total)
	}
}

func newWindowStorageFixture(limit int, mode string) (*RateLimiter, time.Time, time.Duration) {
	window, step := time.Hour, time.Duration(0)
	now := time.Now()
	if mode == "sliding" {
		window, step = time.Duration(limit)*time.Millisecond, time.Millisecond
	}
	rl := NewRateLimiter(limit, window)
	rl.Stop() // avoid timer scheduling in measurements
	for i := 0; i < limit; i++ {
		rl.allowAt("caller", now.Add(time.Duration(i-limit+1)*step))
	}
	return rl, now, step
}

// The identical benchmark is also run on the parent via a hash-bound proof input.
func BenchmarkRateLimiterWindowStorage(b *testing.B) {
	for _, limit := range []int{10, 500, 2000} {
		for _, mode := range []string{"denied", "sliding", "cleanup"} {
			b.Run(fmt.Sprintf("%d/%s", limit, mode), func(b *testing.B) {
				rl, now, step := newWindowStorageFixture(limit, mode)
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					if mode == "cleanup" {
						rl.cleanup()
					} else {
						now = now.Add(step)
						rl.allowAt("caller", now)
					}
				}
			})
		}
	}
}

func BenchmarkRateLimiterMiddlewareWindow(b *testing.B) {
	for _, limit := range []int{10, 500, 2000} {
		b.Run(fmt.Sprint(limit), func(b *testing.B) {
			rl := NewRateLimiter(limit, time.Hour)
			rl.Stop()
			for range limit {
				rl.Allow("192.0.2.17")
			}
			handler := rl.Middleware(func(http.ResponseWriter, *http.Request) {
				b.Fatal("exhausted caller reached downstream handler")
			})
			req := httptest.NewRequest(http.MethodGet, "/api/login", nil)
			req.RemoteAddr = "192.0.2.17:12345"
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				if w.Code != http.StatusTooManyRequests {
					b.Fatalf("status = %d, want 429", w.Code)
				}
			}
		})
	}
}
