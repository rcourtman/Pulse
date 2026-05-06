package alerts

import "time"

const (
	StaleTrackingThreshold              = 24 * time.Hour
	RateLimitCleanupWindow              = 1 * time.Hour
	alertsDirPerm                       = 0o700
	alertsFilePerm                      = 0o600
	offlineRecoveryConfirmationsDefault = 3
	offlineRecoveryConfirmationsStorage = 2
)
