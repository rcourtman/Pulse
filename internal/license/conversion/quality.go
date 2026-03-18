package conversion

import (
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type HealthStatus = pkglicensing.HealthStatus
type PipelineHealth = pkglicensing.PipelineHealth
type PipelineHealthOption = pkglicensing.PipelineHealthOption

var knownConversionEventTypes = pkglicensing.KnownConversionEventTypes()

// NewPipelineHealth creates a health tracker initialized at current time.
func NewPipelineHealth(opts ...PipelineHealthOption) *PipelineHealth {
	return pkglicensing.NewPipelineHealth(opts...)
}

func WithPipelineHealthNow(now func() time.Time) PipelineHealthOption {
	return pkglicensing.WithPipelineHealthNow(now)
}

func WithPipelineHealthStaleThreshold(threshold time.Duration) PipelineHealthOption {
	return pkglicensing.WithPipelineHealthStaleThreshold(threshold)
}
