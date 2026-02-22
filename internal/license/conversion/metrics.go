package conversion

import pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"

type ConversionMetrics = pkglicensing.ConversionMetrics

// GetConversionMetrics returns the singleton conversion metrics instance.
func GetConversionMetrics() *ConversionMetrics {
	return pkglicensing.GetConversionMetrics()
}
