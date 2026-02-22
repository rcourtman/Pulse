package metering

import pkgmetering "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/metering"

const (
	MaxCardinalityPerTenant     = pkgmetering.MaxCardinalityPerTenant
	MaxIdempotencyKeysPerWindow = pkgmetering.MaxIdempotencyKeysPerWindow
)

var (
	ErrCardinalityExceeded         = pkgmetering.ErrCardinalityExceeded
	ErrDuplicateEvent              = pkgmetering.ErrDuplicateEvent
	ErrIdempotencyKeyLimitExceeded = pkgmetering.ErrIdempotencyKeyLimitExceeded
)

type WindowedAggregator = pkgmetering.WindowedAggregator

func NewWindowedAggregator() *WindowedAggregator {
	return pkgmetering.NewWindowedAggregator()
}
