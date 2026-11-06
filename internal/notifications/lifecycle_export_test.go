package notifications

import (
	"database/sql"
	"time"
)

// External-package integration tests need the real cleanup and persisted facts
// without importing ai/config into this package's internal tests (a cycle).
func ReceiptLifecycleCleanupForTest(q *NotificationQueue)    { q.performCleanup() }
func ReceiptLifecycleDBForTest(q *NotificationQueue) *sql.DB { return q.db }
func ReceiptLifecycleSeedForTest(q *NotificationQueue, id string, start, at time.Time) error {
	return recordFixtureDeliveryReceipt(q, id, start, "webhook:fixture", at, 2)
}
