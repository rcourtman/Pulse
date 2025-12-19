package ai

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// MetadataProvider provides access to resource metadata stores
// This allows the AI to update resource URLs when it discovers web services
type MetadataProvider interface {
	// SetGuestURL sets the custom URL for a Proxmox guest (VM/container)
	SetGuestURL(guestID, customURL string) error

	// SetDockerURL sets the custom URL for a Docker container/service
	SetDockerURL(resourceID, customURL string) error

	// SetHostURL sets the custom URL for a host
	SetHostURL(hostID, customURL string) error
}

// SetMetadataProvider sets the metadata provider for URL updates
func (s *Service) SetMetadataProvider(mp MetadataProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadataProvider = mp
	log.Info().Msg("AI service: metadata provider configured for URL discovery")
}

// SetResourceURL updates the URL for a resource in Pulse
// This is called by the AI when it discovers a web service
func (s *Service) SetResourceURL(resourceType, resourceID, customURL string) error {
	s.mu.RLock()
	mp := s.metadataProvider
	s.mu.RUnlock()

	if mp == nil {
		return fmt.Errorf("metadata provider not configured")
	}

	// Validate and normalize the URL
	if customURL != "" {
		// If no scheme, default to http
		if !strings.Contains(customURL, "://") {
			customURL = "http://" + customURL
		}

		// Try to parse the URL
		parsedURL, err := url.Parse(customURL)
		if err != nil {
			return fmt.Errorf("invalid URL format: %w", err)
		}

		// Validate scheme
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("URL must use http:// or https:// scheme")
		}

		// Ensure host is present
		if parsedURL.Host == "" {
			return fmt.Errorf("URL must include a host")
		}
	}

	// Route to the appropriate metadata store based on resource type
	switch strings.ToLower(resourceType) {
	case "guest", "vm", "container", "lxc", "qemu":
		if err := mp.SetGuestURL(resourceID, customURL); err != nil {
			return fmt.Errorf("failed to set guest URL: %w", err)
		}
		log.Info().
			Str("resourceType", resourceType).
			Str("resourceID", resourceID).
			Str("url", customURL).
			Msg("AI set guest URL")

	case "docker", "docker_container", "docker_service":
		if err := mp.SetDockerURL(resourceID, customURL); err != nil {
			return fmt.Errorf("failed to set Docker URL: %w", err)
		}
		log.Info().
			Str("resourceType", resourceType).
			Str("resourceID", resourceID).
			Str("url", customURL).
			Msg("AI set Docker URL")

	case "host":
		if err := mp.SetHostURL(resourceID, customURL); err != nil {
			return fmt.Errorf("failed to set host URL: %w", err)
		}
		log.Info().
			Str("resourceType", resourceType).
			Str("resourceID", resourceID).
			Str("url", customURL).
			Msg("AI set host URL")

	default:
		return fmt.Errorf("unknown resource type: %s (use 'guest', 'docker', or 'host')", resourceType)
	}

	return nil
}
