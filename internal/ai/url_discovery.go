package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// URLDiscoveryResult represents the result of URL discovery for a single resource
type URLDiscoveryResult struct {
	ResourceType string `json:"resource_type"` // "guest", "docker", "host"
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name,omitempty"`
	Status       string `json:"status"` // "found", "not_found", "skipped", "error"
	URL          string `json:"url,omitempty"`
	Error        string `json:"error,omitempty"`
}

// URLDiscoveryProgress represents progress of bulk discovery
type URLDiscoveryProgress struct {
	Total     int                  `json:"total"`
	Completed int                  `json:"completed"`
	Found     int                  `json:"found"`
	Errors    int                  `json:"errors"`
	Results   []URLDiscoveryResult `json:"results"`
	Running   bool                 `json:"running"`
	StartedAt time.Time            `json:"started_at,omitempty"`
}

// URLDiscoveryService handles bulk URL discovery operations
type URLDiscoveryService struct {
	mu            sync.RWMutex
	aiService     *Service
	progress      *URLDiscoveryProgress
	cancelFunc    context.CancelFunc
}

// NewURLDiscoveryService creates a new URL discovery service
func NewURLDiscoveryService(aiService *Service) *URLDiscoveryService {
	return &URLDiscoveryService{
		aiService: aiService,
	}
}

// GetProgress returns the current discovery progress
func (s *URLDiscoveryService) GetProgress() *URLDiscoveryProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.progress == nil {
		return &URLDiscoveryProgress{Running: false}
	}
	// Return a copy
	copy := *s.progress
	return &copy
}

// IsRunning returns true if discovery is in progress
func (s *URLDiscoveryService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress != nil && s.progress.Running
}

// Cancel stops the current discovery operation
func (s *URLDiscoveryService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}
	if s.progress != nil {
		s.progress.Running = false
	}
}

// DiscoverURLs starts bulk URL discovery for resources without URLs
func (s *URLDiscoveryService) DiscoverURLs(ctx context.Context, resourceType string, skipExisting bool) error {
	s.mu.Lock()
	if s.progress != nil && s.progress.Running {
		s.mu.Unlock()
		return fmt.Errorf("discovery already in progress")
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel

	s.progress = &URLDiscoveryProgress{
		Running:   true,
		StartedAt: time.Now(),
		Results:   []URLDiscoveryResult{},
	}
	s.mu.Unlock()

	// Run discovery in background
	go s.runDiscovery(ctx, resourceType, skipExisting)
	return nil
}

// runDiscovery performs the actual discovery work
func (s *URLDiscoveryService) runDiscovery(ctx context.Context, resourceType string, skipExisting bool) {
	defer func() {
		s.mu.Lock()
		s.progress.Running = false
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	// Get resources to scan from the resource provider
	s.aiService.mu.RLock()
	rp := s.aiService.resourceProvider
	mp := s.aiService.metadataProvider
	s.aiService.mu.RUnlock()

	if rp == nil {
		log.Error().Msg("URL discovery: resource provider not available")
		return
	}
	if mp == nil {
		log.Error().Msg("URL discovery: metadata provider not available")
		return
	}

	resources := rp.GetAll()
	if resources == nil {
		log.Error().Msg("URL discovery: no resources available")
		return
	}

	// Filter resources
	var toScan []struct {
		Type string
		ID   string
		Name string
		IP   string
	}

	for _, r := range resources {
		// Only scan workloads (VMs, containers) and hosts
		if !r.IsWorkload() && !r.IsInfrastructure() {
			continue
		}

		// Filter by type if specified
		if resourceType != "" && resourceType != "all" {
			typeMatches := false
			switch strings.ToLower(resourceType) {
			case "guest":
				typeMatches = r.Type == "vm" || r.Type == "container"
			case "docker":
				typeMatches = r.Type == "docker-container" || r.Type == "docker-service"
			case "host":
				typeMatches = r.Type == "host" || r.Type == "node" || r.Type == "docker-host"
			default:
				typeMatches = strings.EqualFold(string(r.Type), resourceType)
			}
			if !typeMatches {
				continue
			}
		}

		// Need an IP to scan
		ip := ""
		if r.Identity != nil && len(r.Identity.IPs) > 0 {
			ip = r.Identity.IPs[0]
		}

		toScan = append(toScan, struct {
			Type string
			ID   string
			Name string
			IP   string
		}{
			Type: string(r.Type),
			ID:   r.ID,
			Name: r.Name,
			IP:   ip,
		})
	}

	// Update total
	s.mu.Lock()
	s.progress.Total = len(toScan)
	s.mu.Unlock()

	log.Info().
		Int("total", len(toScan)).
		Str("resourceType", resourceType).
		Bool("skipExisting", skipExisting).
		Msg("Starting bulk URL discovery")

	// Process each resource
	for _, res := range toScan {
		select {
		case <-ctx.Done():
			log.Info().Msg("URL discovery cancelled")
			return
		default:
		}

		result := s.discoverSingleResource(ctx, res.Type, res.ID, res.Name, res.IP)

		s.mu.Lock()
		s.progress.Completed++
		s.progress.Results = append(s.progress.Results, result)
		if result.Status == "found" {
			s.progress.Found++
		} else if result.Status == "error" {
			s.progress.Errors++
		}
		s.mu.Unlock()
	}

	log.Info().
		Int("total", len(toScan)).
		Int("found", s.progress.Found).
		Int("errors", s.progress.Errors).
		Msg("Bulk URL discovery completed")
}

// discoverSingleResource attempts to discover URL for a single resource
func (s *URLDiscoveryService) discoverSingleResource(ctx context.Context, resType, resID, resName, ip string) URLDiscoveryResult {
	result := URLDiscoveryResult{
		ResourceType: resType,
		ResourceID:   resID,
		ResourceName: resName,
		Status:       "not_found",
	}

	if ip == "" {
		result.Status = "skipped"
		result.Error = "no IP address"
		return result
	}

	// Common web ports to check
	ports := []int{80, 443, 8080, 8443, 8096, 8920, 3000, 5000, 9000, 8081, 8000, 7878, 8989, 9117}

	for _, port := range ports {
		select {
		case <-ctx.Done():
			result.Status = "error"
			result.Error = "cancelled"
			return result
		default:
		}

		scheme := "http"
		if port == 443 || port == 8443 {
			scheme = "https"
		}

		url := fmt.Sprintf("%s://%s:%d/", scheme, ip, port)

		// Try to fetch the URL using the AI service's fetch method
		checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, err := s.aiService.fetchURL(checkCtx, url)
		cancel()

		if err == nil {
			// Found a responding service!
			result.Status = "found"
			result.URL = url

			// Save it to metadata
			if err := s.aiService.SetResourceURL(resType, resID, url); err != nil {
				log.Warn().
					Err(err).
					Str("resourceID", resID).
					Str("url", url).
					Msg("Failed to save discovered URL")
			} else {
				log.Info().
					Str("resourceType", resType).
					Str("resourceID", resID).
					Str("resourceName", resName).
					Str("url", url).
					Msg("Discovered and saved URL")
			}
			return result
		}
	}

	return result
}
