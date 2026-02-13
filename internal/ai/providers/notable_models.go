// Package providers contains AI provider client implementations
package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ModelsDevInfo represents model data from models.dev API
type ModelsDevInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Family      string `json:"family"`
	ReleaseDate string `json:"release_date"`
	LastUpdated string `json:"last_updated"`
	Reasoning   bool   `json:"reasoning"`
}

// ModelsDevProvider represents a provider entry from models.dev
type ModelsDevProvider struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	Models map[string]ModelsDevInfo `json:"models"`
}

// NotableModelsCache caches model metadata from models.dev
type NotableModelsCache struct {
	mu          sync.RWMutex
	data        map[string]ModelsDevInfo // keyed by "provider:model_id"
	lastFetched time.Time
	ttl         time.Duration
	apiURL      string
}

// notableCutoffMonths defines how many months old a model can be to still be "notable"
const notableCutoffMonths = 3

// cacheDefaultTTL is the default cache time-to-live
const cacheDefaultTTL = 24 * time.Hour

// modelsDevAPIURL is the public API endpoint
const modelsDevAPIURL = "https://models.dev/api.json"

var (
	defaultCache     *NotableModelsCache
	defaultCacheOnce sync.Once
)

// GetNotableCache returns the singleton NotableModelsCache instance
func GetNotableCache() *NotableModelsCache {
	defaultCacheOnce.Do(func() {
		defaultCache = NewNotableModelsCache(modelsDevAPIURL, cacheDefaultTTL)
	})
	return defaultCache
}

// NewNotableModelsCache creates a new cache with the specified API URL and TTL
func NewNotableModelsCache(apiURL string, ttl time.Duration) *NotableModelsCache {
	return &NotableModelsCache{
		data:   make(map[string]ModelsDevInfo),
		ttl:    ttl,
		apiURL: apiURL,
	}
}

// providerMapping maps our internal provider names to models.dev provider IDs
var providerMapping = map[string][]string{
	"anthropic": {"anthropic"},
	"openai":    {"openai", "firmware", "github-copilot", "abacus"},
	"google":    {"google", "google-vertex"},
	"gemini":    {"google", "google-vertex"},
	"xai":       {"xai"},
	"mistral":   {"mistral"},
	"deepseek":  {"deepseek"},
	"cohere":    {"cohere"},
}

// Refresh fetches the latest data from models.dev API
func (c *NotableModelsCache) Refresh(ctx context.Context) (retErr error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Skip if cache is still fresh
	if time.Since(c.lastFetched) < c.ttl && len(c.data) > 0 {
		return nil
	}

	log.Debug().Str("url", c.apiURL).Msg("Refreshing notable models cache from models.dev")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return fmt.Errorf("providers.NotableModelsCache.Refresh: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Pulse/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch models.dev API, using fallback")
		return fmt.Errorf("providers.NotableModelsCache.Refresh: execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			closeWrapped := fmt.Errorf("providers.NotableModelsCache.Refresh: close response body: %w", closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, closeWrapped)
				return
			}
			retErr = closeWrapped
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Msg("models.dev API returned non-200 status")
		return nil
	}

	// Parse the response - it's a map of provider ID to provider info
	var providers map[string]ModelsDevProvider
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		log.Warn().Err(err).Msg("Failed to decode models.dev API response")
		return fmt.Errorf("providers.NotableModelsCache.Refresh: decode response: %w", err)
	}

	// Build the lookup map
	newData := make(map[string]ModelsDevInfo)
	for providerID, provider := range providers {
		for modelID, modelInfo := range provider.Models {
			// Store with provider prefix for lookup
			key := normalizeKey(providerID, modelID)
			newData[key] = modelInfo

			// Also store just the model ID for fuzzy matching
			keyNoProvider := normalizeModelID(modelID)
			if _, exists := newData[keyNoProvider]; !exists {
				newData[keyNoProvider] = modelInfo
			}
		}
	}

	c.data = newData
	c.lastFetched = time.Now()
	log.Info().Int("models", len(newData)).Msg("Notable models cache refreshed")

	return nil
}

// IsNotable determines if a model should be marked as notable
// Priority:
// 1. Ollama models are always notable (user explicitly pulled them)
// 2. Check models.dev data for release_date
// 3. Fall back to createdAt timestamp
func (c *NotableModelsCache) IsNotable(provider, modelID string, createdAt int64) bool {
	// Ollama models are always notable - user explicitly pulled them
	if strings.EqualFold(provider, "ollama") {
		return true
	}

	// Try to refresh cache if needed (non-blocking, best effort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := c.Refresh(ctx)
	if err != nil {
		log.Debug().Err(err).Str("provider", provider).Str("model", modelID).Msg("Cache refresh failed in IsNotable")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Log cache size for debugging
	log.Debug().Int("cache_size", len(c.data)).Str("provider", provider).Str("model", modelID).Msg("Checking IsNotable")

	// Try exact match with provider prefix
	for _, providerID := range getProviderIDs(provider) {
		key := normalizeKey(providerID, modelID)
		if info, found := c.data[key]; found {
			isNotable := isRecentlyReleased(info.ReleaseDate, info.LastUpdated)
			log.Debug().Str("key", key).Str("release", info.ReleaseDate).Bool("notable", isNotable).Msg("Found model in cache")
			return isNotable
		}
	}

	// Try fuzzy match on just model ID
	key := normalizeModelID(modelID)
	if info, found := c.data[key]; found {
		isNotable := isRecentlyReleased(info.ReleaseDate, info.LastUpdated)
		log.Debug().Str("key", key).Str("release", info.ReleaseDate).Bool("notable", isNotable).Msg("Found model via fuzzy match")
		return isNotable
	}

	// Fallback: Use createdAt timestamp if available
	if createdAt > 0 {
		createdTime := time.Unix(createdAt, 0)
		cutoff := time.Now().AddDate(0, -notableCutoffMonths, 0)
		return createdTime.After(cutoff)
	}

	// Check if this model family has a newer release that's notable
	// This handles Anthropic API returning old-style names like "claude-3-5-haiku-20241022"
	// when there's a newer "claude-haiku-4-5" in models.dev
	if aliases := getModelFamilyAliases(modelID); len(aliases) > 0 {
		for _, alias := range aliases {
			for _, providerID := range getProviderIDs(provider) {
				key := normalizeKey(providerID, alias)
				if info, found := c.data[key]; found {
					if isRecentlyReleased(info.ReleaseDate, info.LastUpdated) {
						log.Debug().Str("original", modelID).Str("alias", alias).Str("release", info.ReleaseDate).Msg("Model family has notable newer release")
						return true
					}
				}
			}
		}
	}

	// Default: not notable
	log.Debug().Str("provider", provider).Str("model", modelID).Msg("Model not found in cache, defaulting to not notable")
	return false
}

// getModelFamilyAliases returns alternative model names to check for notable status
// This maps old-style API names to their newer model.dev equivalents
func getModelFamilyAliases(modelID string) []string {
	modelID = strings.ToLower(modelID)

	// Anthropic model family mappings
	if strings.Contains(modelID, "haiku") {
		return []string{"claude-haiku-4-5", "claude-3-5-haiku-latest"}
	}
	if strings.Contains(modelID, "sonnet") {
		return []string{"claude-sonnet-4-5", "claude-3-5-sonnet-latest"}
	}
	if strings.Contains(modelID, "opus") {
		return []string{"claude-opus-4-5", "claude-opus-4-1", "claude-opus-4-0"}
	}

	// OpenAI model family mappings
	if strings.Contains(modelID, "gpt-4o") {
		return []string{"gpt-4o", "gpt-4o-2024-11-20"}
	}
	if strings.Contains(modelID, "o1") || strings.Contains(modelID, "o3") {
		return []string{"o1", "o1-pro", "o3-mini"}
	}

	return nil
}

// getProviderIDs returns the models.dev provider IDs for our internal provider name
func getProviderIDs(provider string) []string {
	provider = strings.ToLower(provider)
	if ids, found := providerMapping[provider]; found {
		return ids
	}
	return []string{provider}
}

// normalizeKey creates a normalized lookup key
func normalizeKey(provider, modelID string) string {
	return strings.ToLower(provider) + ":" + normalizeModelID(modelID)
}

// normalizeModelID normalizes a model ID for comparison
func normalizeModelID(modelID string) string {
	// Remove common prefixes
	modelID = strings.ToLower(modelID)
	modelID = strings.TrimPrefix(modelID, "models/")
	modelID = strings.TrimPrefix(modelID, "gemini-")

	// Strip date suffixes like -20251022 or -20240620 from model IDs
	// This helps match "claude-3-5-sonnet-20241022" to "claude-3-5-sonnet"
	if len(modelID) > 9 {
		// Check if last 8 chars look like a date (YYYYMMDD)
		suffix := modelID[len(modelID)-8:]
		if len(suffix) == 8 {
			// All digits and starts with 20XX
			if suffix[0] == '2' && suffix[1] == '0' {
				allDigits := true
				for _, c := range suffix {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits && len(modelID) > 9 && modelID[len(modelID)-9] == '-' {
					modelID = modelID[:len(modelID)-9]
				}
			}
		}
	}

	return modelID
}

// isRecentlyReleased checks if a model was released within the notable cutoff
func isRecentlyReleased(releaseDate, lastUpdated string) bool {
	cutoff := time.Now().AddDate(0, -notableCutoffMonths, 0)

	// Try release_date first
	if releaseDate != "" {
		if t, err := parseFlexibleDate(releaseDate); err == nil {
			return t.After(cutoff)
		}
	}

	// Fall back to last_updated
	if lastUpdated != "" {
		if t, err := parseFlexibleDate(lastUpdated); err == nil {
			return t.After(cutoff)
		}
	}

	return false
}

// parseFlexibleDate parses dates in various formats used by models.dev
func parseFlexibleDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01",
		"2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid date format: %s", dateStr)
}
