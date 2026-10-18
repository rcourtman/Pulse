package api

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func routerOwnedLimiters(r *Router) []*RateLimiter {
	limiters := []*RateLimiter{r.exportLimiter, r.downloadLimiter, r.signupRateLimiter,
		r.handoffExchangeRateLimiter, r.bootstrapTokenValidationLimiter}
	limiters = append(limiters, rateLimitFixtureWorkers(r.endpointRateLimitConfig)...)
	if r.tenantRateLimiter != nil {
		limiters = append(limiters, r.tenantRateLimiter.limiter)
	}
	return limiters
}

func requireLimiterJoined(t *testing.T, limiter *RateLimiter) {
	t.Helper()
	select {
	case <-limiter.cleanupDone:
	default:
		t.Error("Stop returned with a cleanup worker still running")
	}
}

func TestRateLimiterConcurrentStopJoinsWithoutResettingBudget(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	t.Cleanup(limiter.Stop)
	if !limiter.Allow("192.0.2.1") || limiter.Allow("192.0.2.1") {
		t.Fatal("fixture did not consume its budget")
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Stop()
			limiter.Allow("192.0.2.2") // In-flight requests retain normal accounting.
		}()
	}
	wg.Wait()
	requireLimiterJoined(t, limiter)
	if limiter.Allow("192.0.2.1") {
		t.Error("stopping cleanup reset an exhausted request budget")
	}
	(*RateLimiter)(nil).Stop()
	(&RateLimiter{}).Stop()
}

func TestRouterShutdownJoinsEveryOwnedLimiter(t *testing.T) {
	for _, hosted := range []string{"false", "true"} {
		t.Run(hosted, func(t *testing.T) {
			t.Setenv("PULSE_HOSTED_MODE", hosted)
			for i := 0; i < 3; i++ {
				r := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "test")
				cleanupTestRouter(t, r)
				owned := routerOwnedLimiters(r)
				want := 13
				if hosted == "true" {
					want++
				}
				if len(owned) != want {
					t.Fatalf("owned limiters = %d, want %d", len(owned), want)
				}
				r.ShutdownBackgroundWorkers()
				r.ShutdownBackgroundWorkers()
				for _, limiter := range owned {
					requireLimiterJoined(t, limiter)
				}
			}
		})
	}
}

func TestRouterShutdownPreservesOtherRouterAndGlobalBudgets(t *testing.T) {
	isolateRateLimitFixture(t)
	InitializeRateLimiters()
	global := globalRateLimitConfig
	first := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, first)
	second := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, second)
	login := getRateLimiterForEndpoint(second.endpointRateLimitConfig, "/api/login", http.MethodPost)
	for i := 0; i < login.limit; i++ {
		if !login.Allow("192.0.2.1") {
			t.Fatal("other router budget exhausted early")
		}
	}
	first.ShutdownBackgroundWorkers()
	if login.Allow("192.0.2.1") {
		t.Error("another router's shutdown reset login enforcement")
	}
	for _, limiter := range append(routerOwnedLimiters(second), rateLimitFixtureWorkers(global)...) {
		select {
		case <-limiter.stopCleanup:
			t.Error("shutdown stopped a limiter owned by another router/global configuration")
		default:
		}
	}
	if !second.exportLimiter.Allow("192.0.2.2") {
		t.Error("other router no longer admits its independent export budget")
	}
}

func TestRouterShutdownSealsAndJoinsConcurrentWorkerAdmission(t *testing.T) {
	r := NewRouter(&config.Config{DataPath: t.TempDir()}, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, r)
	var started, exited atomic.Int64
	var callers sync.WaitGroup
	for i := 0; i < 32; i++ {
		callers.Add(2)
		go func() {
			defer callers.Done()
			r.startLifecycleWorker(func() {
				started.Add(1)
				<-r.lifecycleCtx.Done()
				exited.Add(1)
			})
		}()
		go func() {
			defer callers.Done()
			r.ShutdownBackgroundWorkers()
		}()
	}
	callers.Wait()
	if started.Load() != exited.Load() {
		t.Fatalf("shutdown returned before joining admitted workers: started %d, exited %d", started.Load(), exited.Load())
	}
	r.startLifecycleWorker(func() { t.Error("worker admitted after shutdown") })
	r.StartBackgroundWorkers() // Late application start cannot reopen admission.
	r.ShutdownBackgroundWorkers()
	for _, limiter := range routerOwnedLimiters(r) {
		requireLimiterJoined(t, limiter)
	}
	(*Router)(nil).ShutdownBackgroundWorkers()
	(&Router{}).ShutdownBackgroundWorkers()
}
