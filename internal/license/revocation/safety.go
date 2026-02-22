package revocation

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type SafeEvaluator = pkglicensing.SafeEvaluator
type EnrollmentRateLimit = pkglicensing.EnrollmentRateLimit

var DefaultEnrollmentRateLimit = pkglicensing.DefaultEnrollmentRateLimit

func NewSafeEvaluator(inner *entitlements.Evaluator) *SafeEvaluator {
	return pkglicensing.NewSafeEvaluator(inner)
}
