package license

import (
	"crypto/ed25519"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// Compatibility-only mirror for existing internal tests.
var (
	publicKeyMu sync.RWMutex
	publicKey   ed25519.PublicKey
)

func SetPublicKey(key ed25519.PublicKey) {
	publicKeyMu.Lock()
	if len(key) == 0 {
		publicKey = nil
	} else {
		keyCopy := make(ed25519.PublicKey, len(key))
		copy(keyCopy, key)
		publicKey = keyCopy
	}
	publicKeyMu.Unlock()

	pkglicensing.SetPublicKey(key)
}

func currentPublicKey() ed25519.PublicKey {
	publicKeyMu.RLock()
	defer publicKeyMu.RUnlock()
	if len(publicKey) == 0 {
		return nil
	}
	keyCopy := make(ed25519.PublicKey, len(publicKey))
	copy(keyCopy, publicKey)
	return keyCopy
}

var (
	ErrInvalidLicense     = pkglicensing.ErrInvalidLicense
	ErrExpiredLicense     = pkglicensing.ErrExpiredLicense
	ErrMalformedLicense   = pkglicensing.ErrMalformedLicense
	ErrSignatureInvalid   = pkglicensing.ErrSignatureInvalid
	ErrFeatureNotIncluded = pkglicensing.ErrFeatureNotIncluded
	ErrNoPublicKey        = pkglicensing.ErrNoPublicKey
)

type SubscriptionState = entitlements.SubscriptionState

const (
	SubStateTrial     = entitlements.SubStateTrial
	SubStateActive    = entitlements.SubStateActive
	SubStateGrace     = entitlements.SubStateGrace
	SubStateExpired   = entitlements.SubStateExpired
	SubStateSuspended = entitlements.SubStateSuspended
	SubStateCanceled  = entitlements.SubStateCanceled
)

type LimitCheckResult = entitlements.LimitCheckResult

const (
	LimitAllowed   = entitlements.LimitAllowed
	LimitSoftBlock = entitlements.LimitSoftBlock
	LimitHardBlock = entitlements.LimitHardBlock
)

type Claims = pkglicensing.Claims
type License = pkglicensing.License
type LicenseState = pkglicensing.LicenseState
type LicenseStatus = pkglicensing.LicenseStatus

const (
	LicenseStateNone        = pkglicensing.LicenseStateNone
	LicenseStateActive      = pkglicensing.LicenseStateActive
	LicenseStateExpired     = pkglicensing.LicenseStateExpired
	LicenseStateGracePeriod = pkglicensing.LicenseStateGracePeriod
)

const (
	DefaultGracePeriod = pkglicensing.DefaultGracePeriod

	// Compatibility constants retained for existing tests.
	maxLicenseKeyLength     = 16 << 10 // 16 KiB
	maxLicenseSegmentLength = 8 << 10  // 8 KiB
	maxLicensePayloadSize   = 8 << 10  // 8 KiB decoded JSON
)

type Service struct {
	*pkglicensing.Service
}

func NewService() *Service {
	return &Service{Service: pkglicensing.NewService()}
}

func ValidateLicense(licenseKey string) (*License, error) {
	return pkglicensing.ValidateLicense(licenseKey)
}

func GenerateLicenseForTesting(email string, tier Tier, expiresIn time.Duration) (string, error) {
	return pkglicensing.GenerateLicenseForTesting(email, tier, expiresIn)
}

func isLicenseValidationDevMode() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_LICENSE_DEV_MODE")), "true")
}
