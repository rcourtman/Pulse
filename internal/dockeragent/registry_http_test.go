package dockeragent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestRegistryChecker_CheckImageUpdate_Behavior(t *testing.T) {
	logger := zerolog.Nop()
	checker := NewRegistryChecker(logger)

	t.Run("disabled checker returns nil", func(t *testing.T) {
		checker.SetEnabled(false)
		result := checker.CheckImageUpdate(context.Background(), "nginx:latest", "sha256:current")
		if result != nil {
			t.Error("Expected nil result when checker is disabled")
		}
		checker.SetEnabled(true)
	})

	t.Run("digest-pinned image skipped", func(t *testing.T) {
		result := checker.CheckImageUpdate(context.Background(), "nginx@sha256:abc123", "sha256:abc123")
		if result == nil {
			t.Fatal("Expected result for digest-pinned image")
		}
		if result.UpdateAvailable {
			t.Error("Expected no update available for digest-pinned image")
		}
		if result.Error != "digest-pinned image" {
			t.Errorf("Expected error 'digest-pinned image', got %q", result.Error)
		}
	})

	t.Run("empty image name", func(t *testing.T) {
		result := checker.CheckImageUpdate(context.Background(), "", "sha256:current")
		if result == nil {
			t.Fatal("Expected result for empty image")
		}
		// Should have an error since we can't parse empty image
	})
}

func TestRegistryChecker_Caching(t *testing.T) {
	logger := zerolog.Nop()
	checker := NewRegistryChecker(logger)

	// Test caching behavior
	cacheKey := "test-key"

	t.Run("cache miss returns nil", func(t *testing.T) {
		entry := checker.getCached(cacheKey)
		if entry != nil {
			t.Error("Expected nil for cache miss")
		}
	})

	t.Run("cache hit for digest", func(t *testing.T) {
		checker.cacheDigest(cacheKey, "sha256:testdigest")
		entry := checker.getCached(cacheKey)
		if entry == nil {
			t.Fatal("Expected cache hit")
		}
		if entry.latestDigest != "sha256:testdigest" {
			t.Errorf("Expected digest 'sha256:testdigest', got %q", entry.latestDigest)
		}
	})

	t.Run("cache hit for error", func(t *testing.T) {
		errorKey := "error-key"
		checker.cacheError(errorKey, "test error")
		entry := checker.getCached(errorKey)
		if entry == nil {
			t.Fatal("Expected cache hit for error")
		}
		if entry.err != "test error" {
			t.Errorf("Expected error 'test error', got %q", entry.err)
		}
	})

	t.Run("cleanup removes expired entries", func(t *testing.T) {
		// Add an expired entry manually
		checker.cache.mu.Lock()
		checker.cache.entries["expired-key"] = cacheEntry{
			latestDigest: "sha256:old",
			expiresAt:    time.Now().Add(-1 * time.Hour), // Already expired
		}
		checker.cache.mu.Unlock()

		checker.CleanupCache()

		entry := checker.getCached("expired-key")
		if entry != nil {
			t.Error("Expected expired entry to be removed")
		}
	})
}

func TestRegistryChecker_ShouldCheck(t *testing.T) {
	logger := zerolog.Nop()
	checker := NewRegistryChecker(logger)

	t.Run("should check when never checked", func(t *testing.T) {
		if !checker.ShouldCheck() {
			t.Error("Expected ShouldCheck to return true when never checked")
		}
	})

	t.Run("should not check immediately after mark", func(t *testing.T) {
		checker.MarkChecked()
		if checker.ShouldCheck() {
			t.Error("Expected ShouldCheck to return false immediately after MarkChecked")
		}
	})

	t.Run("should not check when disabled", func(t *testing.T) {
		checker.SetEnabled(false)
		if checker.ShouldCheck() {
			t.Error("Expected ShouldCheck to return false when disabled")
		}
		checker.SetEnabled(true)
	})
}

func TestParseImageReference_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantReg  string
		wantRepo string
		wantTag  string
	}{
		{
			name:     "quay.io image",
			image:    "quay.io/prometheus/prometheus:v2.45.0",
			wantReg:  "quay.io",
			wantRepo: "prometheus/prometheus",
			wantTag:  "v2.45.0",
		},
		{
			name:     "ecr image",
			image:    "123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:prod",
			wantReg:  "123456789.dkr.ecr.us-east-1.amazonaws.com",
			wantRepo: "myapp",
			wantTag:  "prod",
		},
		{
			name:     "gcr.io image",
			image:    "gcr.io/google-containers/pause:3.2",
			wantReg:  "gcr.io",
			wantRepo: "google-containers/pause",
			wantTag:  "3.2",
		},
		{
			name:     "multi-level path",
			image:    "registry.example.com/org/team/project/app:v1",
			wantReg:  "registry.example.com",
			wantRepo: "org/team/project/app",
			wantTag:  "v1",
		},
		{
			name:     "image with sha256 in name (not pinned)",
			image:    "myimage-sha256:latest",
			wantReg:  "registry-1.docker.io",
			wantRepo: "library/myimage-sha256",
			wantTag:  "latest",
		},
		{
			name:     "mcr.io image",
			image:    "mcr.microsoft.com/dotnet/sdk:7.0",
			wantReg:  "mcr.microsoft.com",
			wantRepo: "dotnet/sdk",
			wantTag:  "7.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReg, gotRepo, gotTag := parseImageReference(tt.image)
			if gotReg != tt.wantReg {
				t.Errorf("registry = %q, want %q", gotReg, tt.wantReg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("repository = %q, want %q", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Errorf("tag = %q, want %q", gotTag, tt.wantTag)
			}
		})
	}
}

// TestRegistryChecker_FetchDigest documents the fetchDigest behavior
// Note: This function requires HTTPS, so we can't easily mock it with httptest
// The function is tested via integration tests with real registries
func TestRegistryChecker_FetchDigest_Documentation(t *testing.T) {
	// fetchDigest:
	// - Always uses HTTPS
	// - Sets Accept headers for manifest types (OCI, Docker V2, V1)
	// - Uses Docker-Content-Digest header if available
	// - Falls back to Etag header if Docker-Content-Digest is missing
	// - Returns error for 4xx/5xx status codes
	// - Caches errors for rate limiting scenarios

	t.Run("documents expected behavior", func(t *testing.T) {
		logger := zerolog.Nop()
		checker := NewRegistryChecker(logger)

		// Attempting to fetch from an invalid registry should fail
		ctx := context.Background()
		_, err := checker.fetchDigest(ctx, "invalid.registry.test", "library/nginx", "latest")
		if err == nil {
			t.Error("Expected error when fetching from invalid registry")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		logger := zerolog.Nop()
		checker := NewRegistryChecker(logger)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before fetching

		_, err := checker.fetchDigest(ctx, "registry-1.docker.io", "library/nginx", "latest")
		if err == nil {
			t.Error("Expected error when context is cancelled")
		}
	})
}

// TestRegistryChecker_AuthToken_Documentation documents the auth token behavior
// Note: We can't easily test this with a mock server since the auth URL is hardcoded
// The auth flow is tested via integration tests with real registries
func TestRegistryChecker_AuthToken_Documentation(t *testing.T) {
	t.Run("auth token documentation", func(t *testing.T) {
		// Auth token flow for Docker Hub:
		// 1. Request manifest without token
		// 2. Receive 401 with Www-Authenticate header
		// 3. Request token from auth.docker.io
		// 4. Retry manifest request with Bearer token

		// For GHCR:
		// 1. Request token from ghcr.io/token
		// 2. Use token for manifest request

		// This behavior is tested via integration tests
		t.Log("Auth token flow is verified via integration tests with real registries")
	})
}

func TestImageUpdateResult_Fields(t *testing.T) {
	result := ImageUpdateResult{
		Image:           "nginx:latest",
		CurrentDigest:   "sha256:current",
		LatestDigest:    "sha256:latest",
		UpdateAvailable: true,
		CheckedAt:       time.Now(),
		Error:           "",
	}

	if !result.UpdateAvailable {
		t.Error("Expected UpdateAvailable to be true")
	}

	if result.Image != "nginx:latest" {
		t.Errorf("Expected image 'nginx:latest', got %q", result.Image)
	}
}

func BenchmarkParseImageReference(b *testing.B) {
	images := []string{
		"nginx",
		"nginx:latest",
		"myrepo/myapp:v1",
		"ghcr.io/owner/repo:tag",
		"registry.example.com:5000/app:v2",
		"nginx@sha256:abc123def456",
	}

	for i := 0; i < b.N; i++ {
		for _, img := range images {
			parseImageReference(img)
		}
	}
}

func BenchmarkDigestsDiffer(b *testing.B) {
	checker := &RegistryChecker{}
	current := "sha256:a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd"
	latest := "sha256:f6e5d4c3b2a1987654321098765432109876543210987654321098765432fedc"

	for i := 0; i < b.N; i++ {
		checker.digestsDiffer(current, latest)
	}
}

// TestConcurrentCacheAccess verifies thread-safety of the cache
func TestConcurrentCacheAccess(t *testing.T) {
	logger := zerolog.Nop()
	checker := NewRegistryChecker(logger)

	// Spawn multiple goroutines accessing the cache
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				checker.cacheDigest(key, fmt.Sprintf("digest-%d", j))
				checker.getCached(key)
			}
			done <- true
		}(i)
	}

	// Also run cleanup concurrently
	go func() {
		for i := 0; i < 50; i++ {
			checker.CleanupCache()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 11; i++ {
		<-done
	}
}
