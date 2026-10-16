package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rs/zerolog/log"
)

func (r routerChartMonitorResolver) RecordSuccessfulExtendedHistoryRead() {
	if r.router == nil || r.router.persistence == nil || mock.IsMockEnabled() {
		return
	}
	// This install-wide tally deliberately has no tenant or actor dimension.
	if err := r.router.persistence.RecordSuccessfulExtendedHistoryRead(time.Now().UTC()); err != nil {
		log.Debug().Msg("Unable to retain aggregate extended history read count")
	}
}
