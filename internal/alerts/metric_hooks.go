package alerts

// Metric hooks for integrating with Prometheus
var (
	recordAlertFired        func(*Alert)
	recordAlertResolved     func(*Alert)
	recordAlertSuppressed   func(string)
	recordAlertAcknowledged func()
)

// SetMetricHooks registers callbacks for recording alert metrics.
// - fired: called when an alert is dispatched (in dispatchAlert)
// - resolved: called when an alert is cleared (in clearAlertNoLock)
// - suppressed: called when an alert is suppressed due to flapping
// - acknowledged: called when an alert is acknowledged
func SetMetricHooks(fired func(*Alert), resolved func(*Alert), suppressed func(string), acknowledged func()) {
	recordAlertFired = fired
	recordAlertResolved = resolved
	recordAlertSuppressed = suppressed
	recordAlertAcknowledged = acknowledged
}
