package notifications

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	destinationCorrelationPrefix = "destination:"
	destinationAliasPrefix       = "destination:v2:"
	destinationAliasKeySize      = 32
)

// initDestinationAliasKey keeps one private key with the queue whose records
// it describes. It is independent of delivery credentials and receipt
// generations. The random in-memory key is also a safe fallback: failure to
// persist diagnostic aliases must not interrupt delivery or expose a raw ID.
func (q *NotificationQueue) initDestinationAliasKey() error {
	key := make([]byte, destinationAliasKeySize)
	rand.Read(key)
	q.destinationAliasKey = key
	if _, err := q.db.Exec(`CREATE TABLE IF NOT EXISTS notification_diagnostic_identity (
		singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
		alias_key BLOB NOT NULL CHECK(length(alias_key) = 32)
	)`); err != nil {
		return err
	}
	if _, err := q.db.Exec(`INSERT INTO notification_diagnostic_identity(singleton, alias_key)
		VALUES(1, ?) ON CONFLICT(singleton) DO NOTHING`, key); err != nil {
		return err
	}
	var saved []byte
	if err := q.db.QueryRow(`SELECT alias_key FROM notification_diagnostic_identity WHERE singleton = 1`).Scan(&saved); err != nil {
		return err
	}
	if len(saved) != destinationAliasKeySize {
		return errors.New("invalid notification alias key length")
	}
	q.destinationAliasKey = saved
	return nil
}

// diagnosticDestinationID is a one-way projection, not a routing key or
// retry handle. Preserve configured webhook IDs so the operator can still
// see their names. Do not publish configuration-derived correlation IDs.
func (q *NotificationQueue) diagnosticDestinationID(id string) string {
	suffix, ok := strings.CutPrefix(id, destinationCorrelationPrefix)
	if !ok || len(suffix) != 32 {
		return id
	}
	if len(q.destinationAliasKey) != destinationAliasKeySize {
		return ""
	}
	mac := hmac.New(sha256.New, q.destinationAliasKey)
	mac.Write([]byte("pulse:notification-diagnostics:v2\x00"))
	mac.Write([]byte(id))
	return destinationAliasPrefix + hex.EncodeToString(mac.Sum(nil)[:16])
}
