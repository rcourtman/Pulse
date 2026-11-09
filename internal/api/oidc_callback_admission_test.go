package api

import (
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestOIDCCallbackPolicyLocalCommitOrdering(t *testing.T) {
	r, _ := setupTestRouter(t)
	p := newRecoveryTestProvider("callback-ordering", true, "")
	if err := r.ssoConfig.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	provider, revision := r.currentOIDCCallbackPolicy(p.ID)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var completionErr error
	go func() {
		defer close(done)
		completionErr = r.commitOIDCCallback(p.ID, revision, provider, func() error { close(entered); <-release; return nil })
	}()
	awaitOIDCInitialization(t, entered)
	// The final local effects own a shared publication fence, not an IdP-wide
	// lock. Immutable configuration reads are still available at this point.
	got := r.currentSSOConfig().GetProvider(p.ID)
	if got == nil || !got.Enabled {
		t.Error("local callback completion blocked valid config reads")
	}
	if r.ssoMu.TryLock() {
		r.ssoMu.Unlock()
		t.Error("policy publication can interleave with final local callback effects")
	}
	unblock()
	awaitOIDCInitialization(t, done)
	if completionErr != nil {
		t.Fatal(completionErr)
	}
	apply, save := r.commitSSOConfig(func(cfg *config.SSOConfig) error {
		changed := *cfg.GetProvider(p.ID)
		changed.Enabled = false
		return cfg.UpdateProvider(changed)
	})
	if apply != nil || save != nil {
		t.Fatalf("withdrawal: %v %v", apply, save)
	}
	called := false
	if err := r.commitOIDCCallback(p.ID, revision, provider, func() error { called = true; return nil }); err != errOIDCCallbackPolicyChanged || called {
		t.Fatalf("superseded local completion ran: called=%v err=%v", called, err)
	}
}
