package main

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

var (
	// nodeNameRegex validates node names (alphanumeric, dots, underscores, hyphens, 1-64 chars)
	// Must not start with hyphen to prevent SSH option injection
	nodeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

	// ipv4Regex validates IPv4 addresses
	ipv4Regex = regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)

	// ipv6Regex validates IPv6 addresses (simplified)
	ipv6Regex = regexp.MustCompile(`^[0-9a-fA-F:]+$`)
)

// sanitizeCorrelationID validates and sanitizes a correlation ID
// Returns a valid UUID, generating a new one if input is missing or invalid
func sanitizeCorrelationID(id string) string {
	if id == "" {
		return uuid.NewString()
	}
	if _, err := uuid.Parse(id); err != nil {
		return uuid.NewString()
	}
	return id
}

// validateNodeName checks if a node name is in valid format
func validateNodeName(name string) error {
	if !nodeNameRegex.MatchString(name) {
		return fmt.Errorf("invalid node name")
	}
	return nil
}
