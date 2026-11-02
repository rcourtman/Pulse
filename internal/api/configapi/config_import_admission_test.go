package configapi

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigImportAdmissionScopeLifetimeAndCapacity(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	req := ImportConfigRequest{Data: "synthetic-ciphertext", Passphrase: "synthetic-passphrase"}
	t.Run("default aliases, other org and different passphrase", func(t *testing.T) {
		var guard configImportRequests
		first, rejection := guard.begin("", req, now)
		if rejection != nil {
			t.Fatal(rejection)
		}
		if _, rejection := guard.begin("default", req, now); rejection == nil || rejection.status != http.StatusConflict {
			t.Fatal("default alias admitted overlapping import")
		}
		other, rejection := guard.begin("other", req, now)
		if rejection != nil {
			t.Fatal("another org inherited active import")
		}
		guard.finish(other, true, now)
		guard.finish(first, true, now)
		if _, rejection := guard.begin("default", req, now.Add(configImportReplayWindow-time.Nanosecond)); rejection == nil || rejection.status != http.StatusConflict {
			t.Fatal("unexpired completed attempt was evicted")
		}
		wrong := req
		wrong.Passphrase += "-different"
		attempt, rejection := guard.begin("default", wrong, now)
		if rejection != nil {
			t.Fatal("another passphrase reused the completed admission")
		}
		guard.finish(attempt, false, now)
		if _, rejection := guard.begin("default", req, now.Add(configImportReplayWindow)); rejection != nil {
			t.Fatal("expired attempt prevented an intentional later import")
		}
	})
	t.Run("in-flight attempts never expire and completion starts window", func(t *testing.T) {
		var guard configImportRequests
		first, _ := guard.begin("default", req, now)
		later := now.Add(2 * configImportReplayWindow)
		if _, rejection := guard.begin("default", req, later); rejection == nil {
			t.Fatal("long-running import expired")
		}
		guard.finish(first, true, later)
		if _, rejection := guard.begin("default", req, later.Add(configImportReplayWindow-time.Nanosecond)); rejection == nil {
			t.Fatal("window did not start at completion")
		}
	})
	t.Run("bounded capacity fails closed without evicting attempts", func(t *testing.T) {
		var guard configImportRequests
		for i := 0; i < configImportRequestLimit; i++ {
			attempt, rejection := guard.begin(fmt.Sprintf("org-%d", i), req, now)
			if rejection != nil {
				t.Fatal(rejection)
			}
			guard.finish(attempt, true, now)
		}
		if _, rejection := guard.begin("overflow", req, now); rejection == nil || rejection.status != http.StatusServiceUnavailable {
			t.Fatal("full registry did not fail closed")
		}
		if len(guard.attempts) != configImportRequestLimit {
			t.Fatal("capacity changed")
		}
		if _, rejection := guard.begin("org-0", req, now); rejection == nil || rejection.status != http.StatusConflict {
			t.Fatal("capacity overflow evicted original replay protection")
		}
		if _, rejection := guard.begin("overflow", req, now.Add(configImportReplayWindow)); rejection != nil {
			t.Fatal("expired registry did not reclaim capacity")
		}
	})
	t.Run("failed transaction releases its admission", func(t *testing.T) {
		var guard configImportRequests
		attempt, _ := guard.begin("default", req, now)
		guard.finish(attempt, false, now)
		if _, rejection := guard.begin("default", req, now); rejection != nil {
			t.Fatal("uncommitted request could not retry")
		}
	})
}

func TestConfigImportAdmissionConcurrentRequestsHaveOneOwner(t *testing.T) {
	var guard configImportRequests
	var admitted atomic.Int32
	var requests sync.WaitGroup
	start := make(chan struct{})
	now := time.Now()
	for i := 0; i < 64; i++ {
		requests.Add(1)
		go func() {
			defer requests.Done()
			<-start
			if _, rejection := guard.begin("default", ImportConfigRequest{Data: "synthetic", Passphrase: "synthetic"}, now); rejection == nil {
				admitted.Add(1)
			}
		}()
	}
	close(start)
	requests.Wait()
	if admitted.Load() != 1 || len(guard.attempts) != 1 {
		t.Fatalf("concurrent owners = %d, want one", admitted.Load())
	}
}

func TestConfigImportAdmissionFingerprintsAreGuardScoped(t *testing.T) {
	now := time.Now()
	req := ImportConfigRequest{Data: "synthetic-ciphertext", Passphrase: "synthetic-passphrase"}
	var firstGuard, secondGuard configImportRequests
	first, rejection := firstGuard.begin("default", req, now)
	if rejection != nil {
		t.Fatal(rejection)
	}
	second, rejection := secondGuard.begin("default", req, now)
	if rejection != nil {
		t.Fatal(rejection)
	}
	if first.key.fingerprint == second.key.fingerprint {
		t.Fatal("independent guards retained the same request fingerprint")
	}
	firstGuard.finish(first, true, now)
	if _, rejection := firstGuard.begin("default", req, now); rejection == nil || rejection.status != http.StatusConflict {
		t.Fatal("keyed fingerprint did not recognise a completed request")
	}
}

func TestConfigImportAdmissionFingerprintsPreserveFieldBoundaries(t *testing.T) {
	now := time.Now()
	var guard configImportRequests
	// The concatenated bytes are equal, but archive and passphrase differ.
	for _, req := range []ImportConfigRequest{
		{Data: "synthetic-a", Passphrase: "bc"},
		{Data: "synthetic-ab", Passphrase: "c"},
	} {
		attempt, rejection := guard.begin("default", req, now)
		if rejection != nil {
			t.Fatal("distinct fields reused a completed request fingerprint", rejection)
		}
		guard.finish(attempt, true, now)
	}
}
