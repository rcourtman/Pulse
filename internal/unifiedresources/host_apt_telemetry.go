package unifiedresources

import (
	"encoding/hex"
	"strings"
	"time"
)

const HostAPTTelemetryMaxClockSkew = 5 * time.Minute

func ValidHostAPTDigest(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && value == strings.ToLower(value)
}

func hostAPTTelemetryFresh(checkedAt, observedAt, now time.Time, maxAge time.Duration) bool {
	if checkedAt.IsZero() || observedAt.IsZero() || maxAge <= 0 {
		return false
	}
	checkedAt = checkedAt.UTC()
	observedAt = observedAt.UTC()
	now = now.UTC()
	if checkedAt.After(now.Add(HostAPTTelemetryMaxClockSkew)) || observedAt.After(now.Add(HostAPTTelemetryMaxClockSkew)) {
		return false
	}
	if observedAt.Before(checkedAt.Add(-HostAPTTelemetryMaxClockSkew)) {
		return false
	}
	return now.Sub(checkedAt) <= maxAge && now.Sub(observedAt) <= maxAge
}

func HostPackageUpdateTelemetryFresh(status *AgentPackageUpdateMeta, now time.Time) bool {
	return status != nil && hostAPTTelemetryFresh(status.CheckedAt, status.ObservedAt, now, HostPackageUpdateFreshness)
}

func HostStorageCleanupTelemetryFresh(status *AgentStorageCleanupMeta, now time.Time) bool {
	return status != nil && hostAPTTelemetryFresh(status.CheckedAt, status.ObservedAt, now, HostStorageCleanupFreshness)
}
