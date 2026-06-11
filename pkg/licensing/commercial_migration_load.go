package licensing

// Kept in a separate file from ClassifyLegacyExchangeError so the
// load-failure contract can evolve with persistence (not exchange) concerns.

const (
	// CommercialMigrationReasonPersistedUnreadable marks a persisted v5
	// license file that exists but cannot be read or decrypted on this
	// system (e.g. license.enc sealed under key material this install can
	// no longer derive).
	CommercialMigrationReasonPersistedUnreadable CommercialMigrationReason = "persisted_license_unreadable"
)

// ClassifyPersistedLicenseLoadError converts a failure to read or decrypt the
// persisted v5 license into the commercial-migration contract. Without this,
// an undecryptable license.enc degraded a paid install to Community with
// nothing but a log line. Re-running the exchange cannot fix an unreadable
// file, so the state is terminal and the remedy is re-entering the v5 key.
func ClassifyPersistedLicenseLoadError(err error) *CommercialMigrationStatus {
	if err == nil {
		return nil
	}
	return &CommercialMigrationStatus{
		Source:            CommercialMigrationSourceV5License,
		State:             CommercialMigrationStateFailed,
		Reason:            CommercialMigrationReasonPersistedUnreadable,
		RecommendedAction: CommercialMigrationActionEnterSupportedV5,
	}
}
