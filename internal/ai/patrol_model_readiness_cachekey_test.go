package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The readiness cache key is written to disk in ai_patrol_model_readiness.json.
// It must still change when credentials change, but it must not be a digest an
// attacker holding that file can brute-force back to the Ollama Basic Auth
// password, which is chosen by a human and therefore low entropy.

func readinessCredentialTestConfig(password string) *config.AIConfig {
	cfg := readinessTestConfig()
	cfg.OllamaUsername = "pulse"
	cfg.OllamaPassword = password
	return cfg
}

func TestPatrolModelReadinessCacheKeyStillTracksCredentialChanges(t *testing.T) {
	service := NewService(config.NewConfigPersistence(t.TempDir()), nil)

	first := service.patrolModelReadinessCacheKey(readinessCredentialTestConfig("hunter2"), config.AIProviderOllama, "test-model")
	second := service.patrolModelReadinessCacheKey(readinessCredentialTestConfig("hunter3"), config.AIProviderOllama, "test-model")

	if first == "" || second == "" {
		t.Fatalf("cache key must be derivable, got %q and %q", first, second)
	}
	if first == second {
		t.Fatal("changing the Ollama password must change the cache key, or stale readiness evidence survives a credential rotation")
	}
}

func TestPatrolModelReadinessCacheKeyIsNotAnUnkeyedPasswordDigest(t *testing.T) {
	const password = "hunter2"
	cfg := readinessCredentialTestConfig(password)
	service := NewService(config.NewConfigPersistence(t.TempDir()), nil)

	key := service.patrolModelReadinessCacheKey(cfg, config.AIProviderOllama, "test-model")
	if key == "" {
		t.Fatal("expected a cache key")
	}

	// The pre-fix construction embedded hex(sha256(username \n password))
	// directly in the hashed material, so anyone holding the evidence file
	// could confirm a guessed password with two SHA-256 operations. Rebuild
	// that derivation and require the key to have moved off it.
	unkeyed := sha256.Sum256([]byte(cfg.OllamaUsername + "\n" + password))
	legacyMaterial := fmt.Sprintf("%s\n%s\n%s\n%x\n%d\n%d\n%d",
		config.AIProviderOllama,
		"test-model",
		cfg.OllamaBaseURL,
		unkeyed,
		cfg.GetRequestTimeout().Milliseconds(),
		cfg.GetPatrolInvestigationBudget(),
		cfg.GetPatrolInvestigationTimeout().Milliseconds(),
	)
	legacySum := sha256.Sum256([]byte(legacyMaterial))
	if key == hex.EncodeToString(legacySum[:]) {
		t.Fatal("cache key is still the unkeyed SHA-256 derivation of the Ollama credentials")
	}

	// Two installs with the same credentials must not agree, which is what
	// proves the fingerprint is salted rather than a global constant.
	other := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	if otherKey := other.patrolModelReadinessCacheKey(cfg, config.AIProviderOllama, "test-model"); otherKey == key {
		t.Fatal("identical credentials produced identical cache keys across installs; the fingerprint is not salted")
	}
}

func TestPatrolModelReadinessSaltPersistsAtRestAndIsPrivate(t *testing.T) {
	dir := t.TempDir()
	cfg := readinessCredentialTestConfig("hunter2")

	first := NewService(config.NewConfigPersistence(dir), nil)
	key := first.patrolModelReadinessCacheKey(cfg, config.AIProviderOllama, "test-model")

	// A restart must reproduce the key or persisted evidence is worthless.
	reloaded := NewService(config.NewConfigPersistence(dir), nil)
	if got := reloaded.patrolModelReadinessCacheKey(cfg, config.AIProviderOllama, "test-model"); got != key {
		t.Fatalf("cache key changed across restart: %q -> %q", key, got)
	}

	saltPath := filepath.Join(dir, patrolModelReadinessSaltFilename)
	info, err := os.Stat(saltPath)
	if err != nil {
		t.Fatalf("stat readiness salt: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("readiness salt permissions = %o, want 600", perm)
	}
	salt, err := os.ReadFile(saltPath)
	if err != nil {
		t.Fatalf("read readiness salt: %v", err)
	}
	if len(salt) != patrolModelReadinessSaltBytes {
		t.Fatalf("readiness salt length = %d, want %d", len(salt), patrolModelReadinessSaltBytes)
	}
}

func TestPatrolModelReadinessCacheKeyWorksWithoutPersistence(t *testing.T) {
	// A Service with no persistence falls back to a process-local salt. The
	// key must still be stable within the process so in-memory caching works.
	service := &Service{}
	cfg := readinessCredentialTestConfig("hunter2")

	key := service.patrolModelReadinessCacheKey(cfg, config.AIProviderOllama, "test-model")
	if key == "" {
		t.Fatal("expected a cache key without persistence")
	}
	if again := service.patrolModelReadinessCacheKey(cfg, config.AIProviderOllama, "test-model"); again != key {
		t.Fatalf("cache key unstable within a process: %q -> %q", key, again)
	}
}
