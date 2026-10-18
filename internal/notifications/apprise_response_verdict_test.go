package notifications

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestAppriseInterruptedResponseRetainsVerdict(t *testing.T) {
	for _, kind := range []string{"firing", "resolved", "test"} {
		for _, code := range []int{200, 401, 429, 503} {
			t.Run(kind+"/"+strconv.Itoa(code), func(t *testing.T) {
				captured := captureAppriseLogs(t)
				server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "1000")
					w.WriteHeader(code)
					_, _ = fmt.Fprint(w, "provider-private-content ", appriseSecret)
				}))
				defer server.Close()
				manager := &NotificationManager{}
				require.NoError(t, manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
				err := sendAppriseForConfidentiality(manager, AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: server.URL}, kind)
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
				want := ClassFromHTTPStatus(code)
				if code == 200 {
					want = NotificationFailureConnectivity
				}
				require.Equal(t, want, ClassifyNotificationFailureError(err))
				assertAppriseConfidential(t, err.Error())
				assertAppriseConfidential(t, captured.String())
			})
		}
	}
}

func TestQueuedAppriseInterruptedResponseDoesNotInventReceipt(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(strconv.FormatBool(complete), func(t *testing.T) {
			var requests atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if !complete {
					w.Header().Set("Content-Length", "1000")
				}
				_, _ = fmt.Fprint(w, "provider-private-content")
			}))
			defer server.Close()
			dir := t.TempDir()
			manager := NewNotificationManagerWithDeferredQueue("", dir)
			t.Cleanup(manager.Stop)
			require.NoError(t, manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
			cfg := NormalizeAppriseConfig(AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: server.URL})
			manager.SetAppriseConfig(cfg)
			config, err := json.Marshal(cfg)
			require.NoError(t, err)
			alert := &alerts.Alert{ID: "response-verdict", ResourceName: "synthetic-node", Message: "synthetic alert", Level: alerts.AlertLevelWarning, StartTime: time.Now()}
			// Admit one retained job with a one-attempt budget to observe an
			// exhausted verdict through the actual processor and durable queue.
			require.NoError(t, manager.queue.Enqueue(&QueuedNotification{ID: "response-verdict", Type: "apprise", Config: config, MaxAttempts: 1, Alerts: []*alerts.Alert{alert}}))
			manager.StartQueueProcessing()
			waitVerdictAudit(t, manager.queue, "response-verdict")
			manager.Stop()
			reopened := NewNotificationManagerWithDeferredQueue("", dir)
			t.Cleanup(reopened.Stop)
			var status, class string
			var attempts, success int
			require.NoError(t, reopened.queue.db.QueryRow(`SELECT status, attempts FROM notification_queue WHERE id = 'response-verdict'`).Scan(&status, &attempts))
			require.NoError(t, reopened.queue.db.QueryRow(`SELECT success, failure_class FROM notification_audit WHERE notification_id = 'response-verdict'`).Scan(&success, &class))
			var receipts int
			require.NoError(t, reopened.queue.db.QueryRow(`SELECT count(*) FROM notification_delivery_receipts`).Scan(&receipts))
			stats, err := reopened.GetTelemetryStats(time.Now().Add(-time.Hour))
			require.NoError(t, err)
			require.Equal(t, int32(1), requests.Load())
			require.Equal(t, 1, attempts)
			require.Equal(t, 1, stats.Attempts)
			if complete {
				require.Equal(t, string(QueueStatusSent), status)
				require.Equal(t, 1, success)
				require.Empty(t, class)
				require.Equal(t, 1, receipts)
				require.Equal(t, 1, stats.Deliveries)
				require.Zero(t, stats.Failures)
			} else {
				require.Equal(t, string(QueueStatusDLQ), status)
				require.Zero(t, success)
				require.Equal(t, string(NotificationFailureConnectivity), class)
				require.Zero(t, receipts)
				require.Zero(t, stats.Deliveries)
				require.Equal(t, 1, stats.Failures)
				require.Equal(t, 1, stats.FailureClasses.Connectivity)
			}
			require.Zero(t, stats.FailureClasses.Unknown)
		})
	}
}
