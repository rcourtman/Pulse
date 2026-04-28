package license

import (
	"time"

	licensetestsupport "github.com/rcourtman/pulse-go-rewrite/pkg/licensing/testsupport"
)

func GenerateLicenseForTesting(email string, tier Tier, expiresIn time.Duration) (string, error) {
	return licensetestsupport.GenerateLicenseForTesting(email, tier, expiresIn)
}
