package api

import "testing"

func rateLimitFixtureWorkers(cfg *EndpointRateLimitConfig) []*RateLimiter {
	if cfg == nil {
		return nil
	}
	return []*RateLimiter{cfg.AuthEndpoints, cfg.ConfigEndpoints, cfg.ExportEndpoints,
		cfg.RecoveryEndpoints, cfg.UpdateEndpoints, cfg.WebSocketEndpoints, cfg.GeneralAPI, cfg.PublicEndpoints}
}

func stopRateLimitFixture(cfg *EndpointRateLimitConfig) {
	for _, limiter := range rateLimitFixtureWorkers(cfg) {
		limiter.Stop()
	}
}

// These fixtures mutate a package global and must remain sequential. Each
// replacement belongs to the fixture, not to the previous global owner.
func isolateRateLimitFixture(t *testing.T) {
	t.Helper()
	saved := globalRateLimitConfig
	globalRateLimitConfig = nil
	t.Cleanup(func() {
		stopRateLimitFixture(globalRateLimitConfig)
		globalRateLimitConfig = saved
	})
}

// Exercise the actual fixtures rather than infer worker ownership from total
// process goroutine counts. No router, database or endpoint timing is involved.
func TestRateLimitInitializationFixturesStopWorkers(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"endpoint", TestGetRateLimiterForEndpoint_InitializesIfNeeded},
		{"middleware", TestUniversalRateLimitMiddleware_InitializesIfNeeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := globalRateLimitConfig
			var created *EndpointRateLimitConfig
			t.Run("fixture", func(t *testing.T) {
				tc.run(t)
				created = globalRateLimitConfig
			})
			if created == nil {
				t.Fatal("fixture did not initialize rate limiters")
			}
			defer stopRateLimitFixture(created) // The failing proof must not leak either.
			if globalRateLimitConfig != saved {
				t.Error("fixture did not restore the previous configuration")
			}
			for i, limiter := range rateLimitFixtureWorkers(created) {
				select {
				case <-limiter.stopCleanup:
				default:
					t.Errorf("fixture left cleanup worker %d active", i)
				}
			}
		})
	}
}
