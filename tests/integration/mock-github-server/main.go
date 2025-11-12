package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ReleaseInfo matches the GitHub API release structure
type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Rate limiting tracker
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
	}
	// Cleanup old entries every minute
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()
	return rl
}

func getenvDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func isChecksumFilename(name string) bool {
	switch strings.ToLower(name) {
	case "checksums.txt", "sha256sums", "sha256sums.txt":
		return true
	default:
		return false
	}
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-1 * time.Minute)
	for ip := range rl.requests {
		filtered := []time.Time{}
		for _, t := range rl.requests[ip] {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = filtered
		}
	}
}

func (rl *rateLimiter) check(ip string, limit int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	// Filter to last minute
	recent := []time.Time{}
	for _, t := range rl.requests[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= limit {
		return false
	}

	recent = append(recent, now)
	rl.requests[ip] = recent
	return true
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	limiter := newRateLimiter()
	baseURL := getenvDefault("MOCK_BASE_URL", fmt.Sprintf("http://mock-github:%s", port))

	// Environment-controlled behavior
	checksumError := os.Getenv("MOCK_CHECKSUM_ERROR") == "true"
	networkError := os.Getenv("MOCK_NETWORK_ERROR") == "true"
	enableRateLimit := os.Getenv("MOCK_RATE_LIMIT") == "true"
	staleRelease := os.Getenv("MOCK_STALE_RELEASE") == "true"

	log.Printf("Mock GitHub Server starting on port %s", port)
	log.Printf("Config: checksumError=%v networkError=%v rateLimit=%v staleRelease=%v",
		checksumError, networkError, enableRateLimit, staleRelease)

	// In-memory storage for tarballs and checksums
	tarballs := make(map[string][]byte)
	checksums := make(map[string]string)
	checksumEntries := make(map[string][]string)

	latestTag := getenvDefault("MOCK_LATEST_VERSION", "v99.0.0")
	prevTag := getenvDefault("MOCK_PREVIOUS_VERSION", "v98.5.0")
	rcTag := getenvDefault("MOCK_RC_VERSION", "v99.1.0-rc.1")

	// Generate test releases
	releases := []ReleaseInfo{
		{
			TagName:     latestTag,
			Name:        fmt.Sprintf("Pulse %s", latestTag),
			Prerelease:  false,
			PublishedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		},
		{
			TagName:     prevTag,
			Name:        fmt.Sprintf("Pulse %s", prevTag),
			Prerelease:  false,
			PublishedAt: time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
		},
		{
			TagName:     rcTag,
			Name:        fmt.Sprintf("Pulse %s", rcTag),
			Prerelease:  true,
			PublishedAt: time.Now().Add(-12 * time.Hour).Format(time.RFC3339),
		},
	}

	// Generate tarballs and checksums for each release
	for i := range releases {
		rel := &releases[i]
		version := strings.TrimPrefix(rel.TagName, "v")
		filename := fmt.Sprintf("pulse-%s-linux-amd64.tar.gz", version)

		// Create dummy tarball
		tarball := createDummyTarball(version)
		tarballs[filename] = tarball

		// Calculate checksum
		hash := sha256.Sum256(tarball)
		checksum := hex.EncodeToString(hash[:])

		// Optionally corrupt checksum for testing
		if checksumError {
			checksum = "0000000000000000000000000000000000000000000000000000000000000000"
		}

		checksums[filename] = checksum
		entry := fmt.Sprintf("%s  %s", checksum, filename)
		checksumEntries[version] = append(checksumEntries[version], entry)
		checksumEntries["v"+version] = append(checksumEntries["v"+version], entry)

		// Add download URLs to release
		rel.Assets = []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{
				Name:               filename,
				BrowserDownloadURL: fmt.Sprintf("%s/download/%s/%s", baseURL, version, filename),
			},
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: fmt.Sprintf("%s/download/%s/checksums.txt", baseURL, version),
			},
		}
	}

	// Releases endpoint
	http.HandleFunc("/repos/rcourtman/Pulse/releases", func(w http.ResponseWriter, r *http.Request) {
		// Rate limiting
		if enableRateLimit {
			ip := r.RemoteAddr
			if !limiter.check(ip, 3) { // Very aggressive: 3 requests per minute
				w.Header().Set("X-RateLimit-Limit", "3")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{
					"message": "API rate limit exceeded",
				})
				log.Printf("Rate limited: %s", ip)
				return
			}
		}

		// Network error simulation
		if networkError {
			time.Sleep(5 * time.Second)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
		log.Printf("Served releases list")
	})

	// Latest release endpoint
	http.HandleFunc("/repos/rcourtman/Pulse/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return first non-prerelease
		for _, rel := range releases {
			if !rel.Prerelease {
				json.NewEncoder(w).Encode(rel)
				log.Printf("Served latest release: %s", rel.TagName)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// Download tarball
	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/download/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		version := parts[0]
		file := parts[1]

		if isChecksumFilename(file) {
			// Generate checksums.txt
			var buf bytes.Buffer
			entries, ok := checksumEntries[version]
			if !ok {
				trimmed := strings.TrimPrefix(version, "v")
				entries = checksumEntries[trimmed]
			}
			if len(entries) == 0 {
				w.WriteHeader(http.StatusNotFound)
				log.Printf("No checksums found for version %s", version)
				return
			}
			for _, line := range entries {
				buf.WriteString(line)
				if !strings.HasSuffix(line, "\n") {
					buf.WriteByte('\n')
				}
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write(buf.Bytes())
			log.Printf("Served checksums for version %s (requested %s)", version, file)
			return
		}

		// Serve tarball (strictly match requested filename first)
		tarball, ok := tarballs[file]
		if !ok {
			// Fallback to canonical filename derived from version (without leading v)
			trimmedVersion := strings.TrimPrefix(version, "v")
			canonical := fmt.Sprintf("pulse-%s-linux-amd64.tar.gz", trimmedVersion)
			tarball, ok = tarballs[canonical]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				log.Printf("Tarball not found: %s (canonical %s)", file, canonical)
				return
			}
			file = canonical
		}

		// Mark as stale if requested
		if staleRelease {
			w.Header().Set("X-Release-Status", "stale")
			w.Header().Set("X-Release-Warning", "This release has known issues and should not be installed")
		}

		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(tarball)))
		w.Write(tarball)
		log.Printf("Served tarball: %s (version %s)", file, version)
	})

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func createDummyTarball(version string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Create a dummy binary with version info
	content := []byte(fmt.Sprintf("#!/bin/sh\necho 'Pulse version %s'\n", version))

	hdr := &tar.Header{
		Name: "pulse",
		Mode: 0755,
		Size: int64(len(content)),
	}

	tw.WriteHeader(hdr)
	tw.Write(content)

	// Add a VERSION file
	versionContent := []byte(version)
	versionHdr := &tar.Header{
		Name: "VERSION",
		Mode: 0644,
		Size: int64(len(versionContent)),
	}
	tw.WriteHeader(versionHdr)
	tw.Write(versionContent)

	tw.Close()
	gw.Close()

	return buf.Bytes()
}
