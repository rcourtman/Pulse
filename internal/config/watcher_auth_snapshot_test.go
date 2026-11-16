package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// The watcher mutex serializes reloads, not API setup, password changes or
// token mutations. Those writers share config.Mu with the reload transaction.
func TestConfigWatcher_ReloadConfigConcurrentAuthorityWriter(t *testing.T) {
	passwordHash, err := hashPasswordFn("synthetic-watcher-password")
	if err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("PULSE_AUTH_USER=file-admin\nPULSE_AUTH_PASS='"+passwordHash+"'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{AuthUser: "initial-admin", AuthPass: passwordHash}
	watcher := &ConfigWatcher{config: cfg, envPath: envPath}
	stop := make(chan struct{})
	started := make(chan struct{})
	done := make(chan struct{})
	var writes atomic.Int64
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			Mu.Lock()
			cfg.AuthUser = "api-admin"
			cfg.AuthPass = passwordHash
			cfg.APITokens = []APITokenRecord{{ID: "synthetic-watcher-token"}}
			if writes.Load()%2 == 1 {
				cfg.AuthUser = "other-api-admin"
				cfg.AuthPass = ""
				cfg.APITokens = nil
			}
			Mu.Unlock()
			if writes.Add(1) == 1 {
				close(started)
			}
			runtime.Gosched()
		}
	}()
	stopWriter := sync.OnceFunc(func() { close(stop); <-done })
	t.Cleanup(stopWriter)
	<-started
	for range 64 {
		watcher.reloadConfig()
	}
	stopWriter()
	if writes.Load() < 2 {
		t.Fatal("the concurrent configuration writer did not run repeatedly")
	}

	// With the independent writer stopped, an ordinary reload still applies
	// the complete credential pair without replacing the saved token inventory.
	Mu.RLock()
	tokenCount := cfg.APITokenCount()
	Mu.RUnlock()
	watcher.reloadConfig()
	Mu.RLock()
	defer Mu.RUnlock()
	if cfg.AuthUser != "file-admin" || cfg.AuthPass != passwordHash {
		t.Error("final reload did not apply the file's credential pair")
	}
	if cfg.APITokenCount() != tokenCount {
		t.Error("credential reload replaced the token inventory")
	}
}
