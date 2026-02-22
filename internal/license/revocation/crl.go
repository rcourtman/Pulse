package revocation

import (
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

const DefaultStaleTTL = pkglicensing.DefaultStaleTTL

type CRLCache = pkglicensing.CRLCache

func NewCRLCache(staleTTL time.Duration) *CRLCache {
	return pkglicensing.NewCRLCache(staleTTL)
}
