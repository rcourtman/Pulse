// +build !mock

package mock

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// IsMockEnabled returns false in production builds
func IsMockEnabled() bool {
	return false
}

// GetMockState returns empty state in production builds
func GetMockState() models.StateSnapshot {
	return models.StateSnapshot{}
}
