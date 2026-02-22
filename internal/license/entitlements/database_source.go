package entitlements

import (
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type DatabaseSource = pkglicensing.DatabaseSource

func NewDatabaseSource(store BillingStore, orgID string, cacheTTL time.Duration) *DatabaseSource {
	return pkglicensing.NewDatabaseSource(store, orgID, cacheTTL)
}
