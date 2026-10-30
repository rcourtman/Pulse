package dockeragent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestRegistryImageChangeRefreshIsBounded(t *testing.T) {
	checker := NewRegistryChecker(zerolog.Nop())
	const ref = "example.test/service:1"
	const key = ref + "|amd64/linux/"
	remote := "sha256:first"
	calls := 0
	checker.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return newStringResponse(http.StatusOK, map[string]string{"Docker-Content-Digest": remote}, ""), nil
	})}
	check := func(local string) *ImageUpdateResult {
		return checker.CheckImageUpdate(context.Background(), ref, local, "amd64", "linux", "")
	}
	if got := check(remote); got.Error != "" || got.UpdateAvailable || calls != 1 {
		t.Fatalf("initial: %+v calls=%d", got, calls)
	}
	remote = "sha256:second"
	if got := check(remote); got.Error != "" || got.UpdateAvailable || calls != 2 {
		t.Fatalf("replacement: %+v calls=%d", got, calls)
	}
	// Old/new replicas, reordered/duplicate digests and repeated inspections
	// share the same latest result instead of invalidating each other.
	expiry := checker.getCached(key).expiresAt
	for _, local := range []string{"sha256:first", "sha256:second", " second , sha256:first,sha256:second"} {
		got := check(local)
		if got.Error != "" || got.LatestDigest != remote || calls != 2 {
			t.Fatalf("replica %s: %+v calls=%d", local, got, calls)
		}
	}
	if !checker.getCached(key).expiresAt.Equal(expiry) {
		t.Fatal("cache hits slide expiry")
	}
	remote = "sha256:fourth"
	for _, local := range []string{"sha256:third", "sha256:fifth", "sha256:third"} {
		got := check(local)
		if got.Error != imageChangeRefreshPending || got.UpdateAvailable || got.LatestDigest != "" || calls != 2 {
			t.Fatalf("unseen rapid replacement: %+v calls=%d", got, calls)
		}
	}
	// Move only the synthetic refresh deadline, not wall time or the TTL.
	checker.cache.mu.Lock()
	entry := checker.cache.entries[key]
	entry.refreshAfter = time.Now().Add(-time.Second)
	checker.cache.entries[key] = entry
	checker.cache.mu.Unlock()
	got := check("sha256:third")
	if got.Error != "" || !got.UpdateAvailable || got.LatestDigest != remote || calls != 3 {
		t.Fatalf("bounded refresh: %+v calls=%d", got, calls)
	}
	for _, local := range []string{"sha256:first", "sha256:second", "sha256:third", remote} {
		got := check(local)
		if got.Error != "" || got.UpdateAvailable != (local != remote) || calls != 3 {
			t.Fatalf("shared refreshed result: %+v calls=%d", got, calls)
		}
	}
	// A reference, architecture, OS or variant is still a separate cache key.
	for _, args := range [][5]string{{"example.test/other:1", "amd64", "linux", "", "sha256:other"}, {ref, "arm", "linux", "v7", "sha256:arm7"}, {ref, "arm", "linux", "v6", "sha256:arm6"}, {ref, "amd64", "windows", "", "sha256:windows"}} {
		before := calls
		got := checker.CheckImageUpdate(context.Background(), args[0], args[4], args[1], args[2], args[3])
		if got.Error != "" || !got.UpdateAvailable || calls != before+1 {
			t.Fatalf("isolated key: %+v calls=%d", got, calls)
		}
	}
}

func TestRegistryImageChangePreservesErrorBackoff(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			checker := NewRegistryChecker(zerolog.Nop())
			const ref = "example.test/service:1"
			const key = ref + "|amd64/linux/"
			calls := 0
			fail := false
			checker.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				if fail {
					return newStringResponse(status, nil, ""), nil
				}
				return newStringResponse(200, map[string]string{"Docker-Content-Digest": "sha256:current"}, ""), nil
			})}
			check := func(local string) *ImageUpdateResult {
				return checker.CheckImageUpdate(context.Background(), ref, local, "amd64", "linux", "")
			}
			check("sha256:current")
			fail = true
			got := check("sha256:new-local")
			if got.Error == "" || got.UpdateAvailable || got.LatestDigest != "" || calls != 2 {
				t.Fatalf("refresh failure: %+v calls=%d", got, calls)
			}
			expiry := checker.getCached(key).expiresAt
			ttl := errorCacheTTL
			if status == http.StatusTooManyRequests {
				ttl = rateLimitCacheTTL
			}
			if d := time.Until(expiry); d > ttl || d < ttl-time.Second {
				t.Fatalf("wrong backoff: %s want %s", d, ttl)
			}
			for _, local := range []string{"sha256:current", "sha256:newer-local", "sha256:yet-another"} {
				next := check(local)
				if next.Error != got.Error || next.UpdateAvailable || next.LatestDigest != "" || calls != 2 {
					t.Fatalf("local change bypasses error backoff: %+v calls=%d", next, calls)
				}
			}
			if !checker.getCached(key).expiresAt.Equal(expiry) {
				t.Fatal("errors slide backoff")
			}
			checker.cache.mu.Lock()
			entry := checker.cache.entries[key]
			entry.expiresAt = time.Now().Add(-time.Second)
			checker.cache.entries[key] = entry
			checker.cache.mu.Unlock()
			fail = false
			if next := check("sha256:current"); next.Error != "" || next.UpdateAvailable || calls != 3 {
				t.Fatalf("expiry recovery: %+v calls=%d", next, calls)
			}
		})
	}
}

func TestRegistryImageChangeRefreshReservation(t *testing.T) {
	checker := NewRegistryChecker(zerolog.Nop())
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	checker.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return newStringResponse(200, map[string]string{"Docker-Content-Digest": "sha256:old"}, ""), nil
		}
		close(started)
		<-release
		return newStringResponse(200, map[string]string{"Docker-Content-Digest": "sha256:new"}, ""), nil
	})}
	const ref = "example.test/service:1"
	check := func() *ImageUpdateResult {
		return checker.CheckImageUpdate(context.Background(), ref, "sha256:new", "amd64", "linux", "")
	}
	checker.CheckImageUpdate(context.Background(), ref, "sha256:old", "amd64", "linux", "")
	done := make(chan *ImageUpdateResult, 1)
	go func() { done <- check() }()
	<-started
	concurrent := check()
	close(release)
	got := <-done
	if concurrent.Error != imageChangeRefreshPending || concurrent.UpdateAvailable || concurrent.LatestDigest != "" || calls.Load() != 2 {
		t.Fatalf("concurrent refresh escaped reservation: %+v calls=%d", concurrent, calls.Load())
	}
	if got.Error != "" || got.UpdateAvailable || got.LatestDigest != "sha256:new" {
		t.Fatalf("reserved result: %+v", got)
	}
	if got := check(); got.Error != "" || got.UpdateAvailable || calls.Load() != 2 {
		t.Fatalf("settled cache: %+v calls=%d", got, calls.Load())
	}
}

func TestRegistryImageChangeHistoryBound(t *testing.T) {
	var history string
	for i := 0; i < 1000; i++ {
		history = mergeObservedLocalDigests(fmt.Sprintf("sha256:%d", i), history)
	}
	digests := strings.Split(history, ",")
	if len(digests) != maxObservedLocalDigests || digests[0] != "999" || digests[len(digests)-1] != "936" {
		t.Fatalf("unbounded/nonrecent history: %q", history)
	}
	if got := mergeObservedLocalDigests(" sha256:NEW, new, , sha256:old", "old,new"); got != "new,old" {
		t.Fatalf("digest normalization/dedup: %q", got)
	}
}
