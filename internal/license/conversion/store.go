package conversion

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

type ConversionStore = pkglicensing.ConversionStore
type StoredConversionEvent = pkglicensing.StoredConversionEvent
type FunnelSummary = pkglicensing.FunnelSummary

func NewConversionStore(dbPath string) (*ConversionStore, error) {
	return pkglicensing.NewConversionStore(dbPath)
}
