package conversion

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

type ConversionStore = pkglicensing.ConversionStore
type StoredConversionEvent = pkglicensing.StoredConversionEvent
type FunnelStageCounts = pkglicensing.FunnelStageCounts
type FunnelSummary = pkglicensing.FunnelSummary
type FunnelDayBreakdown = pkglicensing.FunnelDayBreakdown
type FunnelDimensionBreakdown = pkglicensing.FunnelDimensionBreakdown
type FunnelReport = pkglicensing.FunnelReport

func NewConversionStore(dbPath string) (*ConversionStore, error) {
	return pkglicensing.NewConversionStore(dbPath)
}
