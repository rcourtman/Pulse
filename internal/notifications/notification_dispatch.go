package notifications

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// SendAlert sends notifications for a newly fired or explicitly re-notified alert.
func (n *NotificationManager) SendAlert(alert *alerts.Alert) {
	n.mu.RLock()
	target := n.initialTarget
	n.mu.RUnlock()
	n.sendAlert(alert, alertSendOptions{target: target})
}

// SendEscalatedAlert sends a scheduled escalation notification. Escalation
// cadence is owned by the alert schedule, so delivery cooldown must not suppress
// a level that the alert manager has already decided is due.
func (n *NotificationManager) SendEscalatedAlert(alert *alerts.Alert, notifyTarget string) {
	n.SendEscalatedAlertToDestinations(alert, notifyTarget, nil)
}

// SendEscalatedAlertToDestinations sends one due escalation to exact logical
// destinations when provided. Exact selection is intentionally fail-closed:
// stale or unknown IDs never widen into a channel-wide or all-destination send.
func (n *NotificationManager) SendEscalatedAlertToDestinations(alert *alerts.Alert, notifyTarget string, destinationIDs []string) {
	n.sendAlert(alert, alertSendOptions{
		bypassCooldown: true,
		immediate:      true,
		target:         normalizeNotificationDeliveryTarget(notifyTarget),
		destinationIDs: normalizeEscalationDestinationIDs(destinationIDs),
	})
}

func (n *NotificationManager) sendAlert(alert *alerts.Alert, options alertSendOptions) {
	if alert == nil {
		return
	}
	alert = alert.Clone()
	if alert == nil {
		return
	}
	// Fresh firings cannot inherit local recovery-settlement disposition from
	// caller metadata. Clone owns this map; the alerts owner's state is intact.
	delete(alert.Metadata, metadataResolvedAt)
	delete(alert.Metadata, metadataReceiptRecovered)
	if options.target == "" {
		options.target = notificationDeliveryTargetAll
	}

	n.mu.Lock()

	log.Info().
		Str("alertID", alert.ID).
		Bool("enabled", n.enabled).
		Int("webhooks", len(n.webhooks)).
		Bool("emailEnabled", n.emailConfig.Enabled).
		Msg("send alert called")

	if !n.enabled || n.configurationError != nil {
		log.Debug().Msg("notifications disabled or configuration unavailable, skipping")
		configurationUnavailable := n.configurationError != nil
		n.mu.Unlock()
		if configurationUnavailable {
			n.rehearsalGate(alert, "firing", "unavailable", "configuration_unavailable", "Saved notification configuration is unavailable. No rehearsal delivery was admitted.")
		} else {
			n.rehearsalGate(alert, "firing", "suppressed", "notifications_disabled", "Notification delivery is paused. The rehearsal respects that pause.")
		}
		return
	}
	// The lease covers the queue read below until its write completes, so Stop
	// cannot close storage under an admitted alert.
	if !n.lifecycle.admit() {
		log.Debug().Str("alertID", alert.ID).Msg("notification manager stopping, alert not admitted")
		n.mu.Unlock()
		n.rehearsalGate(alert, "firing", "unavailable", "manager_stopping", "The notification manager stopped before admission.")
		return
	}
	defer n.lifecycle.release()

	n.mu.Unlock()
	var exists bool
	for {
		var revision uint64
		exists, revision = n.firingInCooldown(alert, options)
		n.mu.Lock()
		if !n.enabled {
			n.mu.Unlock()
			n.rehearsalGate(alert, "firing", "suppressed", "notifications_disabled", "Notification delivery was paused before admission.")
			return
		}
		if revision == n.destinationRevision {
			break
		}
		n.mu.Unlock()
	}
	if exists && !options.bypassCooldown {
		n.mu.Unlock()
		n.rehearsalGate(alert, "firing", "suppressed", "cooldown", "The occurrence is in its delivery cooldown.")
		return
	}
	queue := n.queue
	n.mu.Unlock()
	if queue != nil {
		for {
			err := queue.reopenReceiptOccurrence(alert.ID, alert.StartTime)
			if errors.Is(err, errNotificationDispositionChanged) {
				continue
			}
			if errors.Is(err, ErrNotificationDeliverySkipped) || errors.Is(err, errNotificationOccurrenceUnavailable) {
				return
			}
			if err != nil {
				log.Error().Err(err).Msg("Notification firing awaits occurrence lifecycle authority")
			}
			break
		}
	}
	n.mu.Lock()
	if !n.enabled {
		n.mu.Unlock()
		n.rehearsalGate(alert, "firing", "suppressed", "notifications_disabled", "Notification delivery was paused before admission.")
		return
	}

	if options.immediate || !n.groupingEnabled || n.groupWindow <= 0 {
		emailConfig := copyEmailConfig(n.emailConfig)
		webhooks := copyWebhookConfigs(n.webhooks)
		appriseConfig := copyAppriseConfig(n.appriseConfig)
		queue := n.queue
		n.mu.Unlock()

		n.rehearsalGate(alert, "firing", "passed", "admitted", "Notification delivery is enabled. The ordinary routing and queue path owns delivery.")
		n.dispatchFiringAlerts(emailConfig, webhooks, appriseConfig, []*alerts.Alert{alert}, options.target, options.destinationIDs, queue, options.bypassCooldown)
		return
	}

	log.Info().
		Str("alertID", alert.ID).
		Str("resourceName", alert.ResourceName).
		Str("type", alert.Type).
		Float64("value", alert.Value).
		Float64("threshold", alert.Threshold).
		Bool("inCooldown", exists).
		Msg("alert passed cooldown check - adding to pending notifications")

	// Group occurrences, not callback invocations. A repeat before delivery has
	// no cooldown receipt yet, but must not inflate the batch's count or payload.
	n.addPendingFiringAlertLocked(alert)

	// If this is the first alert in the group, start the timer
	if n.groupTimer == nil {
		n.groupDeadline = time.Now().Add(n.groupWindow)
		n.groupTimer = time.AfterFunc(time.Until(n.groupDeadline), func() {
			n.sendGroupedAlerts()
		})
		log.Debug().
			Int("pendingCount", len(n.pendingAlerts)).
			Dur("groupWindow", n.groupWindow).
			Msg("started alert grouping timer")
	}
	deadline := n.groupDeadline
	n.mu.Unlock()
	n.rehearsalGate(alert, "firing", "passed", "admitted", "Notification delivery is enabled. The configured grouping window applies.")
	if rehearsalID(alert) != "" && queue != nil {
		queue.rehearsalGrouping(alert, deadline)
	}
}

type pendingFiringOccurrence struct {
	id    string
	start time.Time
}

func pendingFiringKey(alert *alerts.Alert) (pendingFiringOccurrence, bool) {
	if alert == nil || alert.ID == "" || alert.StartTime.IsZero() {
		// Incomplete legacy identity cannot prove that two callbacks refer to
		// the same occurrence. Preserve both rather than silently lose one.
		return pendingFiringOccurrence{}, false
	}
	// time.Time's map equality includes location and monotonic clock data;
	// occurrence identity must use instant equality, as cancellation does.
	return pendingFiringOccurrence{alert.ID, alert.StartTime.UTC().Round(0)}, true
}

// addPendingFiringAlertLocked takes an already-cloned snapshot under n.mu.
// The index belongs only to the current grouping window; cancellation rebuilds
// it lazily, and every flush/disable discards it. No delivery is recorded here.
func (n *NotificationManager) addPendingFiringAlertLocked(alert *alerts.Alert) {
	if key, known := pendingFiringKey(alert); known {
		if n.pendingAlertIndex == nil {
			n.pendingAlertIndex = make(map[pendingFiringOccurrence]int, len(n.pendingAlerts)+1)
			for index, pending := range n.pendingAlerts {
				if key, known := pendingFiringKey(pending); known {
					n.pendingAlertIndex[key] = index
				}
			}
		}
		if index, exists := n.pendingAlertIndex[key]; exists {
			previous := n.pendingAlerts[index]
			// Keep the newest observation's complete snapshot, including its
			// operational links. Equal-time callbacks prefer higher severity;
			// unknown observation time cannot evict a known newer observation.
			if previous.LastSeen.After(alert.LastSeen) ||
				(previous.LastSeen.Equal(alert.LastSeen) && notificationSeverityRank(previous.Level) > notificationSeverityRank(alert.Level)) {
				return
			}
			n.pendingAlerts[index] = alert
			return
		}
		n.pendingAlertIndex[key] = len(n.pendingAlerts)
	}
	n.pendingAlerts = append(n.pendingAlerts, alert)
}

// Unknown levels cannot opt into the severity cooldown exception.
func notificationSeverityRank(level alerts.AlertLevel) int {
	switch level {
	case alerts.AlertLevelInfo:
		return 1
	case alerts.AlertLevelWarning:
		return 2
	case alerts.AlertLevelCritical:
		return 3
	default:
		return 0
	}
}

func (n *NotificationManager) markAlertsNotified(alertsToSend []*alerts.Alert, sentAt time.Time) {
	n.mu.Lock()
	if n.lastNotified == nil {
		n.lastNotified = make(map[string]notificationRecord)
	}
	for _, alert := range alertsToSend {
		if alert == nil {
			continue
		}
		level := alert.Level
		lastSent := sentAt
		if previous, ok := n.lastNotified[alert.ID]; ok {
			// Destinations and retries can complete out of order. An older
			// occurrence still owns its delivery receipt, but must not replace
			// the current occurrence's repeat-delivery cooldown. Unknown legacy
			// starts likewise cannot evict a known occurrence.
			if previous.alertStart.After(alert.StartTime) {
				continue
			}
			if previous.alertStart.Equal(alert.StartTime) {
				// A late warning cannot re-enable critical delivery, and an
				// earlier completion cannot shorten the same occurrence's hold.
				if notificationSeverityRank(previous.level) > notificationSeverityRank(level) {
					level = previous.level
				}
				if previous.lastSent.After(lastSent) {
					lastSent = previous.lastSent
				}
			}
		}
		n.lastNotified[alert.ID] = notificationRecord{
			level:      level,
			lastSent:   lastSent,
			alertStart: alert.StartTime,
		}
	}
	n.mu.Unlock()
}

func (n *NotificationManager) dispatchFiringAlerts(
	emailConfig EmailConfig,
	webhooks []WebhookConfig,
	appriseConfig AppriseConfig,
	alertsToSend []*alerts.Alert,
	target notificationDeliveryTarget,
	destinationIDs []string,
	queue *NotificationQueue,
	bypassCooldown ...bool,
) {
	jobs := buildNotificationDeliveryJobsForSelection(
		emailConfig,
		webhooks,
		appriseConfig,
		alertsToSend,
		eventAlert,
		time.Time{},
		target,
		destinationIDs,
	)
	before := jobs
	if len(bypassCooldown) == 0 || !bypassCooldown[0] {
		jobs = n.filterFiringJobsByCooldown(jobs)
	} else {
		jobs = n.bindDeliveryEpochs(jobs)
	}
	queue.rehearsalRouting(alertsToSend, emailConfig, webhooks, appriseConfig, target, before, jobs, "firing")
	if len(jobs) == 0 {
		return
	}
	if queue != nil {
		n.enqueueNotificationJobs(queue, jobs)
		return
	}
	n.dispatchNotificationJobsAsync(jobs)
}

func notificationDeliveryDestinationKey(job notificationDeliveryJob) string {
	var identity string
	switch job.Type {
	case "email":
		if job.EmailConfig == nil {
			return ""
		}
		recipients := append([]string(nil), job.EmailConfig.To...)
		if len(recipients) == 0 && strings.TrimSpace(job.EmailConfig.From) != "" {
			recipients = append(recipients, job.EmailConfig.From)
		}
		for i := range recipients {
			recipients[i] = strings.ToLower(strings.TrimSpace(recipients[i]))
		}
		sort.Strings(recipients)
		identity = strings.Join(recipients, "\x1f")
	case "webhook":
		if job.WebhookConfig == nil {
			return ""
		}
		// Include both the logical configuration ID and the actual endpoint.
		// Reusing an ID after changing its URL must not send a recovery to an
		// endpoint that never received the firing occurrence.
		webhookID := strings.TrimSpace(job.WebhookConfig.ID)
		webhookURL := strings.TrimSpace(job.WebhookConfig.URL)
		if webhookID == "" && webhookURL == "" {
			return ""
		}
		identity = strings.Join([]string{webhookID, webhookURL}, "\x1f")
		// A relay URL can route to different accounts. Include the receiver
		// fingerprint so each actual recipient owns independent authority.
		identity += "\x1f" + notificationDeliveryConfigurationFingerprint(job)
	case "apprise":
		if job.AppriseConfig == nil {
			return ""
		}
		targets := append([]string(nil), job.AppriseConfig.Targets...)
		for i := range targets {
			targets[i] = strings.TrimSpace(targets[i])
		}
		sort.Strings(targets)
		identity = strings.Join([]string{
			string(job.AppriseConfig.Mode),
			strings.TrimSpace(job.AppriseConfig.ServerURL),
			strings.TrimSpace(job.AppriseConfig.ConfigKey),
			strings.Join(targets, "\x1f"),
		}, "\x1e")
	default:
		return ""
	}
	if identity == "" {
		return ""
	}
	return hashDeliveryDestination(job.Type, identity)
}

func hashDeliveryDestination(kind, identity string) string {
	digest := sha256.Sum256([]byte(kind + "\x00" + identity))
	return kind + ":" + hex.EncodeToString(digest[:16])
}

func legacyWebhookDestination(hook WebhookConfig) (string, string) {
	identity := strings.Join([]string{strings.TrimSpace(hook.ID), strings.TrimSpace(hook.URL)}, "\x1f")
	return hashDeliveryDestination("webhook", identity), identity
}

func notificationDeliveryReceiptKey(alert *alerts.Alert, destinationKey string) string {
	if alert == nil || alert.ID == "" || alert.StartTime.IsZero() || destinationKey == "" {
		return ""
	}
	return alert.ID + "\x00" + strconv.FormatInt(alert.StartTime.UnixNano(), 10) + "\x00" + destinationKey
}

func (n *NotificationManager) recordSuccessfulDelivery(job notificationDeliveryJob, deliveredAt time.Time) error {
	destinationKey := notificationDeliveryDestinationKey(job)
	if destinationKey == "" {
		return nil
	}
	queue := n.GetQueue()
	if queue != nil && !job.ReceiptEpochBound {
		return fmt.Errorf("notification delivery has no admitted destination generation")
	}
	var recorded []*alerts.Alert
	var receiptErrors []error
	for _, alert := range job.Alerts {
		if alert == nil {
			continue
		}
		if alert.StartTime.IsZero() {
			if job.Event == eventAlert && alert.ID != "" {
				recorded = append(recorded, alert)
			}
			continue
		}
		if queue != nil {
			var err error
			if job.Event == eventResolved {
				err = queue.deleteDeliveryReceiptEpoch(alert.ID, alert.StartTime, destinationKey, job.ReceiptEpoch, job.QueueID)
			} else {
				if !job.ReceiptEpochBound {
					err = fmt.Errorf("notification delivery has no admitted destination generation")
				} else {
					err = queue.recordDeliveryReceipt(alert.ID, alert.StartTime, destinationKey, deliveredAt, notificationSeverityRank(alert.Level), job.ReceiptEpoch, job.QueueID)
				}
			}
			if err != nil {
				log.Error().Err(err).Str("alertID", alert.ID).Str("destination", queue.diagnosticDestinationID(notificationDestinationIDForJob(job))).Msg("Failed to update notification delivery receipt")
				receiptErrors = append(receiptErrors, err)
			} else {
				n.cacheDeliveryReceipt(alert, destinationKey, deliveredAt, job.Event)
				recorded = append(recorded, alert)
			}
			continue
		}

		key := notificationDeliveryReceiptKey(alert, destinationKey)
		if key == "" {
			continue
		}
		n.mu.Lock()
		if n.deliveryReceipts == nil {
			n.deliveryReceipts = make(map[string]struct{})
		}
		if job.Event == eventResolved {
			delete(n.deliveryReceipts, key)
		} else {
			n.deliveryReceipts[key] = struct{}{}
		}
		n.mu.Unlock()
		n.cacheDeliveryReceipt(alert, destinationKey, deliveredAt, job.Event)
		recorded = append(recorded, alert)
	}
	if job.Event == eventAlert {
		// Publish the cooldown/delivered marker only after the per-destination
		// receipts are durable. Resolution can race the delivery goroutine and
		// uses this marker as proof that receipt recording has completed.
		n.markAlertsNotified(recorded, deliveredAt)
	}
	return errors.Join(receiptErrors...)
}

func (n *NotificationManager) filterResolvedJobsByDeliveryReceipt(jobs []notificationDeliveryJob) []notificationDeliveryJob {
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	queue := n.GetQueue()
	n.mu.RLock()
	currentFingerprints := n.currentDeliveryFingerprintsLocked()
	n.mu.RUnlock()
	filtered := make([]notificationDeliveryJob, 0, len(jobs))
	for _, job := range jobs {
		key := notificationDeliveryDestinationKey(job)
		if fingerprint, configured := currentFingerprints[key]; configured && fingerprint != notificationDeliveryConfigurationFingerprint(job) {
			continue
		}
		if key == "" {
			continue
		}
		groups := make(map[string][]*alerts.Alert)
		var order []string
		for _, alert := range job.Alerts {
			if alert == nil || alert.StartTime.IsZero() {
				continue
			}
			eligible := false
			epoch := ""
			if queue != nil {
				var err error
				_, epoch, eligible, err = queue.deliveryReceiptWithEpoch(alert, key)
				if err != nil {
					log.Error().Err(err).Msg("Failed to read notification delivery receipt; suppressing recovery")
					queue.rehearsalReceiptUnavailable(alert, notificationDestinationIDForJob(job))
					continue
				}
			} else {
				n.mu.RLock()
				_, eligible = n.deliveryReceipts[notificationDeliveryReceiptKey(alert, key)]
				n.mu.RUnlock()
			}
			if !eligible {
				continue
			}
			if _, found := groups[epoch]; !found {
				order = append(order, epoch)
			}
			groups[epoch] = append(groups[epoch], alert)
		}
		for _, epoch := range order {
			copy := job
			copy.Alerts = groups[epoch]
			copy.ReceiptEpoch = epoch
			copy.ReceiptEpochBound = true
			filtered = append(filtered, copy)
		}
	}
	return filtered
}

// SendResolvedAlert applies the same grouping window to recoveries as firings.
// The two event types have separate buffers; a recovery can never be rendered
// as a firing alert. Normal operation persists the grouping window in the
// delivery queue; the in-memory buffer is only the queue-unavailable fallback.
func (n *NotificationManager) SendResolvedAlert(resolved *alerts.ResolvedAlert) {
	if resolved == nil || resolved.Alert == nil {
		return
	}

	// Clone the alert so downstream goroutines cannot mutate shared state.
	alertCopy := resolved.Alert.Clone()
	if alertCopy == nil {
		return
	}
	delete(alertCopy.Metadata, metadataReceiptRecovered)

	resolvedAt := resolved.ResolvedTime
	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}
	annotateResolvedMetadata(alertCopy, resolvedAt)

	n.mu.Lock()
	if !n.enabled || n.configurationError != nil || !n.notifyOnResolve || !n.lifecycle.accepting() {
		enabled, configured, recoveries := n.enabled, n.configurationError == nil, n.notifyOnResolve
		n.mu.Unlock()
		status, reason, message := "unavailable", "manager_stopping", "The notification manager stopped before recovery admission."
		if !configured {
			reason, message = "configuration_unavailable", "Saved notification configuration is unavailable."
		} else if !enabled {
			status, reason, message = "suppressed", "notifications_disabled", "Notification delivery is paused. Recovery was not sent."
		} else if !recoveries {
			status, reason, message = "suppressed", "recovery_disabled", "Recovery notifications are disabled in the saved alert policy."
		}
		n.rehearsalGate(alertCopy, "recovery", status, reason, message)
		return
	}
	if n.queue == nil && n.groupingEnabled && n.groupWindow > 0 {
		// Repeated reconciliation of one occurrence must not duplicate batch rows.
		for _, pending := range n.pendingResolved {
			if pending.ID == alertCopy.ID && pending.StartTime.Equal(alertCopy.StartTime) {
				n.mu.Unlock()
				return
			}
		}
		n.pendingResolved = append(n.pendingResolved, alertCopy)
		if n.resolvedGroupTimer == nil {
			n.resolvedGroupTimer = time.AfterFunc(n.groupWindow, n.sendGroupedResolvedAlerts)
		}
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()
	n.dispatchResolvedAlerts([]*alerts.Alert{alertCopy})
}

// takePendingResolvedLocked drains only the recovery buffer. Call with mu held.
func (n *NotificationManager) takePendingResolvedLocked() []*alerts.Alert {
	pending := n.pendingResolved
	n.pendingResolved = nil
	if n.resolvedGroupTimer != nil {
		n.resolvedGroupTimer.Stop()
		n.resolvedGroupTimer = nil
	}
	return pending
}

func (n *NotificationManager) sendGroupedResolvedAlerts() {
	n.mu.Lock()
	pending := n.takePendingResolvedLocked()
	n.mu.Unlock()
	n.dispatchResolvedAlerts(pending)
}

func (n *NotificationManager) dispatchResolvedAlerts(alertList []*alerts.Alert) {
	if len(alertList) == 0 {
		return
	}
	n.destinationMu.Lock()
	n.mu.RLock()
	enabled := n.enabled && n.notifyOnResolve
	emailConfig := copyEmailConfig(n.emailConfig)
	webhooks := copyWebhookConfigs(n.webhooks)
	appriseConfig := copyAppriseConfig(n.appriseConfig)
	queue := n.queue
	// The lease precedes the receipt read below, which already uses the queue.
	admitted := enabled && n.lifecycle.admit()
	n.mu.RUnlock()
	n.destinationMu.Unlock()
	if !admitted {
		return
	}
	defer n.lifecycle.release()
	// Synchronous occurrence cancellation owns resolution disposition. Dispatch
	// can be delayed past a refire and must never relabel that newer firing.
	jobs := buildNotificationDeliveryJobs(emailConfig, webhooks, appriseConfig, alertList, eventResolved, resolvedTimeFromAlerts(alertList))
	before := jobs
	// Filter each destination independently at dispatch, not merely when the
	// alert enters the window. Never announce a recovery for an undelivered firing.
	jobs = n.filterResolvedJobsByDeliveryReceipt(jobs)
	queue.rehearsalRouting(alertList, emailConfig, webhooks, appriseConfig, notificationDeliveryTargetAll, before, jobs, "recovery")
	if queue != nil {
		n.enqueueNotificationJobs(queue, jobs)
	} else {
		n.dispatchNotificationJobsAsync(jobs)
	}
}

// CancelAlert removes pending notifications for a resolved alert. It reports
// whether the alert's firing notification was cancelled before any delivery:
// true means the user never received the firing notification (it was still in
// the grouping window or waiting in the queue), so sending a recovery
// notification would reference an alert that was never announced.
func (n *NotificationManager) CancelAlert(alertID string) bool {
	return n.cancelAlert(alertID, nil)
}

// CancelResolvedAlert cancels firing work for one resolved occurrence, without
// deleting a newer occurrence's grouping entries or delivery cooldown.
// ID-wide cancellation remains available for explicit administrative callers.
func (n *NotificationManager) CancelResolvedAlert(alert *alerts.Alert) bool {
	if alert == nil || alert.ID == "" || alert.StartTime.IsZero() {
		return false
	}
	return n.cancelAlert(alert.ID, &alert.StartTime)
}

func (n *NotificationManager) cancelAlert(alertID string, start *time.Time) bool {
	n.mu.Lock()
	queue := n.queue
	// Cancellation still runs while Stop drains; its lease keeps the queue
	// open until the cancellation is committed.
	if queue != nil && !n.lifecycle.join() {
		queue = nil
	}
	record, firingDelivered := n.lastNotified[alertID]
	firingDelivered = firingDelivered && (start == nil || record.alertStart.Equal(*start))

	removed := 0
	if len(n.pendingAlerts) > 0 {
		filtered := n.pendingAlerts[:0]
		for _, pending := range n.pendingAlerts {
			if pending == nil {
				continue
			}
			if pending.ID == alertID && (start == nil || pending.StartTime.Equal(*start)) {
				removed++
				continue
			}
			filtered = append(filtered, pending)
		}

		for i := len(filtered); i < len(n.pendingAlerts); i++ {
			n.pendingAlerts[i] = nil
		}

		n.pendingAlerts = filtered
		if removed > 0 {
			n.pendingAlertIndex = nil
		}

		if len(n.pendingAlerts) == 0 && n.groupTimer != nil {
			if n.groupTimer.Stop() {
				log.Debug().Str("alertID", alertID).Msg("stopped grouping timer after alert cancellation")
			}
			n.groupTimer = nil
			n.groupDeadline = time.Time{}
		}
	}

	// Clean up cooldown record for resolved alert
	if start == nil || record.alertStart.Equal(*start) {
		delete(n.lastNotified, alertID)
	}
	remaining := len(n.pendingAlerts)
	n.mu.Unlock()

	// Cancel any queued notifications containing this alert
	cancelledPending := 0
	if queue != nil {
		defer n.lifecycle.release()

		var err error
		if start == nil {
			cancelledPending, err = queue.CancelByAlertIdentifiers([]string{alertID})
		} else {
			cancelledPending, err = queue.CancelByAlertOccurrence(alertID, *start)
		}
		if err != nil {
			log.Error().Err(err).Str("alertID", alertID).Msg("failed to cancel queued notifications")
		}
		n.destinationMu.Lock()
		n.mu.Lock()
		for key := range n.deliveryCooldown {
			if strings.HasPrefix(key, alertID+"\x00") {
				delete(n.deliveryCooldown, key)
			}
		}
		n.mu.Unlock()
		n.destinationMu.Unlock()
	}

	firingNeverDelivered := !firingDelivered && (removed > 0 || cancelledPending > 0)

	log.Debug().
		Str("alertID", alertID).
		Int("remaining", remaining).
		Bool("firingNeverDelivered", firingNeverDelivered).
		Msg("removed resolved alert from pending notifications and cooldown map")

	return firingNeverDelivered
}

// sendGroupedAlerts sends all pending alerts as a group
func (n *NotificationManager) sendGroupedAlerts() {
	n.destinationMu.Lock()
	n.mu.Lock()
	if len(n.pendingAlerts) == 0 {
		n.mu.Unlock()
		n.destinationMu.Unlock()
		return
	}
	// A timer racing Stop must leave the buffer for the shutdown checkpoint,
	// not dispatch it directly or lose it after admission has closed.
	if !n.lifecycle.admit() {
		n.mu.Unlock()
		n.destinationMu.Unlock()
		return
	}
	defer n.lifecycle.release()

	// Copy alerts to send
	alertsToSend := make([]*alerts.Alert, len(n.pendingAlerts))
	copy(alertsToSend, n.pendingAlerts)

	// Clear pending alerts
	clear(n.pendingAlerts)
	n.pendingAlerts = n.pendingAlerts[:0]
	n.pendingAlertIndex = nil
	if n.groupTimer != nil {
		n.groupTimer.Stop()
	}
	n.groupTimer = nil
	n.groupDeadline = time.Time{}

	log.Info().
		Int("alertCount", len(alertsToSend)).
		Msg("sending grouped alert notifications")

	// Snapshot configuration while holding the lock to avoid races with concurrent updates
	emailConfig := copyEmailConfig(n.emailConfig)
	webhooks := copyWebhookConfigs(n.webhooks)
	appriseConfig := copyAppriseConfig(n.appriseConfig)
	initialTarget := n.initialTarget
	queue := n.queue
	n.mu.Unlock()
	n.destinationMu.Unlock()

	n.dispatchFiringAlerts(emailConfig, webhooks, appriseConfig, alertsToSend, initialTarget, nil, queue)
}

func buildNotificationDeliveryJobs(emailConfig EmailConfig, webhooks []WebhookConfig, appriseConfig AppriseConfig, alertsToSend []*alerts.Alert, event notificationEvent, resolvedAt time.Time) []notificationDeliveryJob {
	return buildNotificationDeliveryJobsForTarget(
		emailConfig,
		webhooks,
		appriseConfig,
		alertsToSend,
		event,
		resolvedAt,
		notificationDeliveryTargetAll,
	)
}

func buildNotificationDeliveryJobsForTarget(
	emailConfig EmailConfig,
	webhooks []WebhookConfig,
	appriseConfig AppriseConfig,
	alertsToSend []*alerts.Alert,
	event notificationEvent,
	resolvedAt time.Time,
	target notificationDeliveryTarget,
) []notificationDeliveryJob {
	return buildNotificationDeliveryJobsForSelection(
		emailConfig,
		webhooks,
		appriseConfig,
		alertsToSend,
		event,
		resolvedAt,
		target,
		nil,
	)
}

func buildNotificationDeliveryJobsForSelection(
	emailConfig EmailConfig,
	webhooks []WebhookConfig,
	appriseConfig AppriseConfig,
	alertsToSend []*alerts.Alert,
	event notificationEvent,
	resolvedAt time.Time,
	target notificationDeliveryTarget,
	destinationIDs []string,
) []notificationDeliveryJob {
	var jobs []notificationDeliveryJob
	for _, batch := range partitionRehearsalAlerts(alertsToSend) {
		jobs = append(jobs, buildNotificationDeliveryJobsForSelectionBatch(emailConfig, webhooks, appriseConfig, batch, event, resolvedAt, target, destinationIDs)...)
	}
	return jobs
}

func buildNotificationDeliveryJobsForSelectionBatch(emailConfig EmailConfig, webhooks []WebhookConfig, appriseConfig AppriseConfig, alertsToSend []*alerts.Alert, event notificationEvent, resolvedAt time.Time, target notificationDeliveryTarget, destinationIDs []string) []notificationDeliveryJob {
	if target == "" {
		target = notificationDeliveryTargetAll
	}
	destinationIDs = normalizeEscalationDestinationIDs(destinationIDs)
	exactSelection := len(destinationIDs) > 0
	selected := make(map[string]struct{}, len(destinationIDs))
	for _, id := range destinationIDs {
		selected[id] = struct{}{}
	}
	isSelected := func(id string) bool {
		_, ok := selected[id]
		return ok
	}

	jobs := make([]notificationDeliveryJob, 0, 2+len(webhooks))
	if emailConfig.Enabled && ((exactSelection && isSelected("email")) || (!exactSelection && (target == notificationDeliveryTargetAll || target == notificationDeliveryTargetEmail))) {
		routedAlerts := routeNotificationAlerts(alertsToSend, emailConfig.TagFilter, emailConfig.TagMode, emailConfig.MinimumSeverity, event)
		if len(routedAlerts) > 0 {
			emailCopy := emailConfig
			jobs = append(jobs, notificationDeliveryJob{
				Type:        "email",
				Event:       event,
				Alerts:      routedAlerts,
				ResolvedAt:  resolvedAt,
				EmailConfig: &emailCopy,
			})
		}
	}
	if exactSelection || target == notificationDeliveryTargetAll || target == notificationDeliveryTargetWebhook {
		for _, webhook := range webhooks {
			if !webhook.Enabled || (exactSelection && !isSelected("webhook:"+strings.TrimSpace(webhook.ID))) {
				continue
			}
			routedAlerts := routeNotificationAlerts(alertsToSend, webhook.TagFilter, webhook.TagMode, webhook.MinimumSeverity, event)
			if len(routedAlerts) > 0 {
				webhookCopy := webhook
				// PagerDuty Events v2 resolves one dedup_key, not a list.
				// Never consume other occurrences' receipts for that request.
				if event == eventResolved && webhook.Service == "pagerduty" && strings.TrimSpace(webhook.Template) == "" {
					for _, alert := range routedAlerts {
						jobs = append(jobs, notificationDeliveryJob{Type: "webhook", Event: event, Alerts: []*alerts.Alert{alert}, ResolvedAt: resolvedAt, WebhookConfig: &webhookCopy})
					}
					continue
				}
				jobs = append(jobs, notificationDeliveryJob{
					Type:          "webhook",
					Event:         event,
					Alerts:        routedAlerts,
					ResolvedAt:    resolvedAt,
					WebhookConfig: &webhookCopy,
				})
			}
		}
	}
	if appriseConfig.Enabled && ((exactSelection && isSelected("apprise")) || (!exactSelection && (target == notificationDeliveryTargetAll || target == notificationDeliveryTargetApprise))) {
		routedAlerts := routeNotificationAlerts(alertsToSend, nil, "", appriseConfig.MinimumSeverity, event)
		if len(routedAlerts) > 0 {
			appriseCopy := appriseConfig
			jobs = append(jobs, notificationDeliveryJob{
				Type:          "apprise",
				Event:         event,
				Alerts:        routedAlerts,
				ResolvedAt:    resolvedAt,
				AppriseConfig: &appriseCopy,
			})
		}
	}
	return jobs
}

func normalizeEscalationDestinationIDs(destinationIDs []string) []string {
	return alerts.NormalizeEscalationDestinationIDs(destinationIDs)
}

func queueTypeForNotificationDeliveryJob(job notificationDeliveryJob) string {
	queueType := job.Type
	if job.Event == eventResolved {
		queueType += queueTypeSuffixResolved
	}
	return queueType
}

func normalizeNotificationDeliveryTarget(raw string) notificationDeliveryTarget {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(notificationDeliveryTargetAll):
		return notificationDeliveryTargetAll
	case string(notificationDeliveryTargetEmail):
		return notificationDeliveryTargetEmail
	case string(notificationDeliveryTargetWebhook), "webhooks":
		return notificationDeliveryTargetWebhook
	case string(notificationDeliveryTargetApprise):
		return notificationDeliveryTargetApprise
	default:
		return notificationDeliveryTargetAll
	}
}

func configJSONForNotificationDeliveryJob(job notificationDeliveryJob) ([]byte, error) {
	switch job.Type {
	case "email":
		if job.EmailConfig == nil {
			return nil, fmt.Errorf("missing email config")
		}
		return json.Marshal(*job.EmailConfig)
	case "webhook":
		if job.WebhookConfig == nil {
			return nil, fmt.Errorf("missing webhook config")
		}
		return json.Marshal(*job.WebhookConfig)
	case "apprise":
		if job.AppriseConfig == nil {
			return nil, FailfWithClass(NotificationFailureConfiguration, "missing apprise config")
		}
		return json.Marshal(*job.AppriseConfig)
	default:
		return nil, fmt.Errorf("unknown notification type: %s", job.Type)
	}
}

// notificationDestinationIDForJob supplies the existing queue/audit and
// rehearsal correlation ID. Project it through diagnosticDestinationID before
// presenting it to an operator. Delivery authority uses a separate key.
func notificationDestinationIDForJob(job notificationDeliveryJob) string {
	parts := []string{job.Type}
	switch job.Type {
	case "email":
		if job.EmailConfig != nil {
			recipients := append([]string(nil), job.EmailConfig.To...)
			sort.Strings(recipients)
			parts = append(
				parts,
				job.EmailConfig.Provider,
				job.EmailConfig.SMTPHost,
				strconv.Itoa(job.EmailConfig.SMTPPort),
				job.EmailConfig.From,
				strings.Join(recipients, "\x00"),
			)
		}
	case "webhook":
		if job.WebhookConfig != nil {
			if id := strings.TrimSpace(job.WebhookConfig.ID); id != "" {
				return "webhook:" + id
			}
			parts = append(
				parts,
				job.WebhookConfig.Name,
				job.WebhookConfig.Service,
				job.WebhookConfig.URL,
			)
		}
	case "apprise":
		if job.AppriseConfig != nil {
			targets := append([]string(nil), job.AppriseConfig.Targets...)
			sort.Strings(targets)
			parts = append(
				parts,
				string(job.AppriseConfig.Mode),
				job.AppriseConfig.ServerURL,
				job.AppriseConfig.ConfigKey,
				strings.Join(targets, "\x00"),
			)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return destinationCorrelationPrefix + hex.EncodeToString(sum[:16])
}

func (n *NotificationManager) enqueueNotificationJobs(queue *NotificationQueue, jobs []notificationDeliveryJob) bool {
	if queue == nil {
		return false
	}

	n.mu.RLock()
	groupWindow := n.groupWindow
	if !n.groupingEnabled {
		groupWindow = 0
	}
	n.mu.RUnlock()
	anyFailed := false
	for _, job := range jobs {
		if !job.ReceiptEpochBound {
			job = n.bindDeliveryEpochs([]notificationDeliveryJob{job})[0]
		}
		configJSON, err := configJSONForNotificationDeliveryJob(job)
		if err != nil {
			n.logNotificationJobError(job, err, "failed to marshal config for queue")
			continue
		}

		for _, bucket := range notificationQueueBucketsForJob(job, time.Now()) {
			notif := &QueuedNotification{
				Type:          queueTypeForNotificationDeliveryJob(bucket.job),
				DestinationID: notificationDestinationIDForJob(bucket.job),
				Alerts:        bucket.job.Alerts,
				Config:        configJSON,
				MaxAttempts:   3,
				NextRetryAt:   bucket.nextRetryAt,
				ReceiptEpoch:  deliveryJobEpoch(job),
			}
			var enqueueErr error
			if job.Event == eventResolved && groupWindow > 0 && !(job.WebhookConfig != nil && job.WebhookConfig.Service == "pagerduty" && strings.TrimSpace(job.WebhookConfig.Template) == "") {
				enqueueErr = queue.enqueueResolvedGroup(notif, groupWindow)
			} else {
				enqueueErr = queue.Enqueue(notif)
			}
			if err := enqueueErr; err != nil {
				anyFailed = true
				if len(bucket.job.Alerts) > 0 && rehearsalID(bucket.job.Alerts[0]) != "" {
					// A rehearsal cannot establish queue delivery by taking the
					// ordinary best-effort direct fallback around a failed queue.
					queue.rehearsalGate(bucket.job.Alerts[0], rehearsalPhase(job.Event), "unavailable", "queue_unavailable", "Queue admission failed. No direct rehearsal fallback was sent.")
					continue
				}
				if !bucket.job.ReceiptEpochBound {
					// Storage is unavailable. Preserve the established best-effort
					// direct fallback without giving its completion a current generation.
					bucket.job.ReceiptEpoch = uuid.NewString()
					bucket.job.ReceiptEpochBound = true
				}
				n.logNotificationJobError(bucket.job, err, "failed to enqueue notification - falling back to direct send")
				n.dispatchNotificationJobAsync(bucket.job, "failed to send notification after queue enqueue failure")
				continue
			}

			logger := log.Debug().
				Str("type", bucket.job.Type).
				Str("event", string(bucket.job.Event)).
				Int("alertCount", len(bucket.job.Alerts))
			if notif.NextRetryAt != nil {
				logger = logger.Time("nextRetryAt", *notif.NextRetryAt)
			}
			if bucket.job.WebhookConfig != nil {
				logger = logger.Str("webhookName", bucket.job.WebhookConfig.Name)
			}
			logger.Msg("enqueued notification delivery job")
		}
	}

	if anyFailed && len(jobs) > 0 && jobs[0].Event == eventResolved {
		log.Debug().Msg("at least one resolved notification enqueue failed; direct sends were triggered")
	}
	return anyFailed
}

func (n *NotificationManager) dispatchNotificationJobsAsync(jobs []notificationDeliveryJob) {
	for _, job := range jobs {
		if job.Type == "email" && job.EmailConfig != nil {
			log.Info().
				Int("alertCount", len(job.Alerts)).
				Str("smtpHost", job.EmailConfig.SMTPHost).
				Int("smtpPort", job.EmailConfig.SMTPPort).
				Strs("recipients", job.EmailConfig.To).
				Msg("email notifications enabled - dispatching delivery job")
		}
		n.dispatchNotificationJobAsync(job, "failed to send notification")
	}
}

func (n *NotificationManager) dispatchNotificationJobAsync(job notificationDeliveryJob, failureMessage string) {
	// Direct delivery stays owned until it finishes, so Stop waits for it and
	// nothing is sent, or recorded only in memory, after Stop returns.
	if !n.lifecycle.join() {
		log.Warn().
			Str("type", job.Type).
			Str("event", string(job.Event)).
			Int("alertCount", len(job.Alerts)).
			Msg("notification manager stopped; direct delivery refused")
		return
	}
	n.mu.RLock()
	queue := n.queue
	n.mu.RUnlock()
	spawnAsync(func() {
		defer n.lifecycle.release()
		if queue != nil {
			if job.Event == eventAlert {
				prepared, release, ok := n.prepareDirectFiringJob(queue, job)
				if !ok {
					return
				}
				job = prepared
				defer release()
			} else {
				release := queue.acquireAlertDeliveryGates(alertIdentifiersFromAlerts(job.Alerts), false)
				defer release()
			}
		}
		if !n.IsEnabled() {
			return
		}
		batches := [][]*alerts.Alert{job.Alerts}
		if job.WebhookConfig != nil && sizesTelegramDeliveries(*job.WebhookConfig) && len(job.Alerts) > 1 {
			batches = telegramAlertBatches(job.Alerts, job.Event)
		}
		for _, batch := range batches {
			job.Alerts = batch
			if err := n.deliverNotificationJob(job); err != nil {
				n.logNotificationJobError(job, err, failureMessage)
				continue
			}
			if err := n.recordSuccessfulDelivery(job, time.Now()); err != nil {
				n.logNotificationJobError(job, err, "failed to retain successful notification delivery")
			}
		}
	})
}

// Filter stale members independently. A concurrent newer disposition causes
// a fresh preflight, not loss of an otherwise current grouped direct delivery.
func (n *NotificationManager) prepareDirectFiringJob(queue *NotificationQueue, job notificationDeliveryJob) (notificationDeliveryJob, func(), bool) {
	for {
		var currentAlerts []*alerts.Alert
		tokens := make(map[pendingFiringOccurrence]int64)
		for _, alert := range job.Alerts {
			if alert == nil {
				continue
			}
			for {
				token, err := queue.reopenReceiptOccurrenceWithToken(alert.ID, alert.StartTime, false)
				if errors.Is(err, errNotificationDispositionChanged) {
					continue
				}
				if err != nil {
					n.logNotificationJobError(job, err, "direct firing member refused without current occurrence authority")
					break
				}
				currentAlerts = append(currentAlerts, alert)
				if key, ok := pendingFiringKey(alert); ok {
					tokens[key] = token
				}
				break
			}
		}
		job.Alerts = currentAlerts
		if len(job.Alerts) == 0 {
			return job, nil, false
		}
		release := queue.acquireAlertDeliveryGates(alertIdentifiersFromAlerts(job.Alerts), false)
		queue.mu.RLock()
		changed := false
		var readErr error
		for key, token := range tokens {
			_, current, err := queue.receiptResolutionLocked(key.id, key.start)
			if err != nil {
				readErr = err
				break
			}
			if current > token {
				changed = true
				break
			}
		}
		queue.mu.RUnlock()
		if readErr != nil {
			release()
			return job, nil, false
		}
		if changed {
			release()
			continue
		}
		return job, release, true
	}
}

func (n *NotificationManager) deliverNotificationJob(job notificationDeliveryJob) error {
	switch job.Type {
	case "email":
		if job.EmailConfig == nil {
			return fmt.Errorf("missing email config")
		}
		if job.Event == eventResolved {
			return n.sendResolvedEmail(*job.EmailConfig, job.Alerts, job.ResolvedAt)
		}
		return n.sendGroupedEmail(*job.EmailConfig, job.Alerts)
	case "webhook":
		if job.WebhookConfig == nil {
			return fmt.Errorf("missing webhook config")
		}
		if job.Event == eventResolved {
			return n.sendResolvedWebhook(*job.WebhookConfig, job.Alerts, job.ResolvedAt)
		}
		return n.sendGroupedWebhook(*job.WebhookConfig, job.Alerts)
	case "apprise":
		if job.AppriseConfig == nil {
			return FailfWithClass(NotificationFailureConfiguration, "missing apprise config")
		}
		if job.Event == eventResolved {
			return n.sendResolvedApprise(*job.AppriseConfig, job.Alerts, job.ResolvedAt)
		}
		return n.sendGroupedApprise(*job.AppriseConfig, job.Alerts)
	default:
		return fmt.Errorf("unknown notification type: %s", job.Type)
	}
}

func (n *NotificationManager) logNotificationJobError(job notificationDeliveryJob, err error, message string) {
	logger := log.Error().
		Err(err).
		Str("type", job.Type).
		Str("event", string(job.Event))
	if job.WebhookConfig != nil {
		logger = logger.Str("webhookName", job.WebhookConfig.Name)
	}
	logger.Msg(message)
}
