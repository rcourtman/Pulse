package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// The watcher mutex serializes reloads, not API setup, password changes or
// token mutations. Those writers share config.Mu with the reload transaction.
func TestConfigWatcher_ReloadConfigConcurrentAuthorityWriter(t *testing.T) {
	const tokenID = "synthetic-watcher-token"
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
	firstWrite := make(chan struct{})
	secondWrite := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			Mu.Lock()
			if i%2 == 0 {
				cfg.AuthUser = "api-admin"
				cfg.AuthPass = passwordHash
				cfg.APITokens = []APITokenRecord{{ID: tokenID}}
			} else {
				cfg.AuthUser = "other-api-admin"
				cfg.AuthPass = ""
				cfg.APITokens = nil
			}
			Mu.Unlock()
			switch i {
			case 0:
				close(firstWrite)
			case 1:
				close(secondWrite)
			}
			runtime.Gosched()
		}
	}()
	stopWriter := sync.OnceFunc(func() { close(stop); <-done })
	t.Cleanup(stopWriter)
	<-firstWrite
	// Don't order both updates before the first reload: the second update
	// must remain concurrent with it for the race detector. Wait for the
	// writer's progress before continuing, rather than assuming it ran again.
	watcher.reloadConfig()
	<-secondWrite
	for range 63 {
		watcher.reloadConfig()
	}
	stopWriter()

	// Once the writer has joined, check the ordinary reload against a known
	// different pair and non-empty inventory, regardless of who wrote last.
	cfg.AuthUser = "api-admin"
	cfg.AuthPass = ""
	cfg.APITokens = []APITokenRecord{{ID: tokenID}}
	watcher.reloadConfig()
	if cfg.AuthUser != "file-admin" || cfg.AuthPass != passwordHash {
		t.Error("final reload did not apply the file's credential pair")
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != tokenID {
		t.Error("credential reload replaced the token inventory")
	}
}
