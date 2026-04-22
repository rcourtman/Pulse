package license

import (
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func GenerateLicenseForTesting(email string, tier Tier, expiresIn time.Duration) (string, error) {
	return pkglicensing.GenerateLicenseForTesting(email, tier, expiresIn)
}
