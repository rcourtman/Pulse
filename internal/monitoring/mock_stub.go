// +build production

package monitoring

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// getMockState returns nil in production builds
func getMockState() *models.StateSnapshot {
	return nil
}