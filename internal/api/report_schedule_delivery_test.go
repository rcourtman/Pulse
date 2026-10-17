package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/stretchr/testify/require"
)

func reportDeliveryFixture(t *testing.T) (*ReportingHandlers, *config.ConfigPersistence, *stubReportingEngine, context.Context, config.ReportSchedule) {
	t.Helper()
	h, p := newTestReportingScheduleHandlers(t)
	engine := &stubReportingEngine{data: []byte("cpu\n42\n"), contentType: "text/csv"}
	previous := reporting.GetEngine()
	reporting.SetEngine(engine)
	t.Cleanup(func() { reporting.SetEngine(previous) })
	s := validReportSchedulePayload()
	s.ID = "delivery"
	s.Cadence = config.ReportScheduleCadence{Type: "weekly", Weekday: "monday", Time: "09:00", Timezone: "UTC"}
	s.Format = "csv"
	s.Delivery = config.ReportScheduleDelivery{Method: "email", To: []string{"client@example.test"}, Attach: true, SaveToDisk: true}
	return h, p, engine, context.WithValue(context.Background(), OrgIDContextKey, "default"), s
}

// SMTP receives real report bytes and supplies structured completed-message
// verdicts. Zero drops the connection after DATA, making acceptance uncertain.
func reportSMTP(t *testing.T, replies []int, onMessage func()) (notifications.EmailConfig, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	var sessions sync.WaitGroup
	attempts := &atomic.Int32{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			wire, err := listener.Accept()
			if err != nil {
				return
			}
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				defer wire.Close()
				_ = wire.SetDeadline(time.Now().Add(10 * time.Second))
				conn := textproto.NewConn(wire)
				if conn.PrintfLine("220 localhost") != nil {
					return
				}
				for {
					line, err := conn.ReadLine()
					if err != nil {
						return
					}
					switch strings.Fields(line)[0] {
					case "EHLO", "HELO", "MAIL", "RCPT":
						err = conn.PrintfLine("250 OK")
					case "DATA":
						if conn.PrintfLine("354 Send message") != nil {
							return
						}
						body, readErr := conn.ReadDotBytes()
						if readErr != nil {
							return
						}
						if !strings.Contains(string(body), "Content-Disposition: attachment") {
							t.Error("report has no MIME attachment")
						}
						attempt := int(attempts.Add(1)) - 1
						if onMessage != nil {
							onMessage()
						}
						code := replies[len(replies)-1]
						if attempt < len(replies) {
							code = replies[attempt]
						}
						if code == 0 {
							return
						}
						err = conn.PrintfLine("%d provider-verdict-do-not-persist", code)
					case "QUIT":
						_ = conn.PrintfLine("221 Bye")
						return
					default:
						t.Errorf("unexpected SMTP command %q", line)
						return
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done; sessions.Wait() })
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	n, err := strconv.Atoi(port)
	require.NoError(t, err)
	return notifications.EmailConfig{Enabled: true, SMTPHost: "127.0.0.1", SMTPPort: n, From: "pulse@example.test", To: []string{"client@example.test"}}, attempts
}

func storedReport(t *testing.T, p *config.ConfigPersistence) config.ReportSchedule {
	t.Helper()
	store, err := p.LoadReportScheduleStore()
	require.NoError(t, err)
	require.Len(t, store.Schedules, 1)
	return store.Schedules[0]
}

func runDeliveryRequest(h *ReportingHandlers, ctx context.Context, retry bool) *httptest.ResponseRecorder {
	url := "/api/admin/reports/schedules/delivery/run"
	if retry {
		url += "?retry=true"
	}
	r := httptest.NewRequest(http.MethodPost, url, nil).WithContext(ctx)
	r.SetPathValue("id", "delivery")
	w := httptest.NewRecorder()
	h.HandleRunReportSchedule(w, r)
	return w
}

func TestReportScheduleDeliveryPreflightAndServerOwnedState(t *testing.T) {
	h, p, engine, ctx, s := reportDeliveryFixture(t)
	now := time.Now().UTC()
	s.LastRunStatus = "ok"
	s.LastReportPath = "/untrusted/path"
	s.LastSuccessAt = &now
	s.RetryAt = &now
	s.RunWindowEnd = &now
	s.RunAttempts = 99
	r := httptest.NewRequest(http.MethodPost, "/api/admin/reports/schedules", encodeScheduleBody(t, s)).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleCreateReportSchedule(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "Enable email notifications")
	s.Enabled = false
	r = httptest.NewRequest(http.MethodPost, "/api/admin/reports/schedules", encodeScheduleBody(t, s)).WithContext(ctx)
	w = httptest.NewRecorder()
	h.HandleCreateReportSchedule(w, r)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := storedReport(t, p)
	require.Empty(t, created.LastRunStatus)
	require.Empty(t, created.LastReportPath)
	require.Nil(t, created.LastSuccessAt)
	require.Nil(t, created.RetryAt)
	require.Nil(t, created.RunWindowEnd)
	require.Zero(t, created.RunAttempts)
	created.ID = "delivery"
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{created}}))
	w = runDeliveryRequest(h, ctx, false)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Empty(t, engine.lastMulti.Resources, "missing email config must fail before generation")
	failed := storedReport(t, p)
	require.Equal(t, "failed", failed.LastRunStatus)
	require.Nil(t, failed.RetryAt)
	require.Empty(t, failed.LastReportPath)
	list := httptest.NewRecorder()
	h.HandleListReportSchedules(list, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	var payload reportScheduleListResponse
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &payload))
	require.False(t, payload.DeliveryChecks["delivery"].Ready)
}

func TestReportScheduleDeliveryRetriesSurviveRestartAndKeepWindow(t *testing.T) {
	h, p, engine, ctx, s := reportDeliveryFixture(t)
	cfg, attempts := reportSMTP(t, []int{451, 451, 250}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	now := time.Date(2026, 10, 19, 9, 0, 1, 0, time.UTC)
	h.runDueReportSchedulesForOrg(ctx, "default", now)
	first := storedReport(t, p)
	require.Equal(t, "retrying", first.LastRunStatus)
	require.Equal(t, 1, first.RunAttempts)
	require.NotNil(t, first.RetryAt)
	window := engine.lastMulti.End
	require.NotContains(t, first.LastError, "provider-verdict")
	// A fresh handler reads only the persisted state, with no in-memory retry job.
	restarted := NewReportingHandlers(h.reportMonitorManager(), nil)
	restarted.runDueReportSchedulesForOrg(ctx, "default", now.Add(30*time.Second))
	require.EqualValues(t, 1, attempts.Load())
	restarted.runDueReportSchedulesForOrg(ctx, "default", first.RetryAt.Add(time.Second))
	second := storedReport(t, p)
	require.Equal(t, "retrying", second.LastRunStatus)
	require.Equal(t, 2, second.RunAttempts)
	require.True(t, second.RetryAt.After(first.RetryAt.Add(4*time.Minute)))
	restarted.runDueReportSchedulesForOrg(ctx, "default", second.RetryAt.Add(time.Second))
	finished := storedReport(t, p)
	require.Equal(t, "ok", finished.LastRunStatus)
	require.Equal(t, "sent_with_attachment", finished.LastDeliveryStatus)
	require.Equal(t, 3, finished.RunAttempts)
	require.Nil(t, finished.RetryAt)
	require.NotNil(t, finished.LastSuccessAt)
	require.FileExists(t, finished.LastReportPath)
	require.Equal(t, window, engine.lastMulti.End)
	restarted.runDueReportSchedulesForOrg(ctx, "default", now.Add(time.Hour))
	require.EqualValues(t, 3, attempts.Load())
}

func TestReportScheduleDeliveryStopsAtBudgetRejectionAndUncertainty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     int
		want     string
		attempts int32
	}{{"exhausted", 451, "failed", 3}, {"rejected", 550, "failed", 1}, {"unconfirmed", 0, "unconfirmed", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			h, p, _, ctx, s := reportDeliveryFixture(t)
			cfg, attempts := reportSMTP(t, []int{tc.code}, nil)
			require.NoError(t, p.SaveEmailConfig(cfg))
			require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
			now := time.Date(2026, 10, 19, 9, 0, 1, 0, time.UTC)
			for i := 0; i < 5; i++ {
				h.runDueReportSchedulesForOrg(ctx, "default", now.Add(time.Duration(i)*10*time.Minute))
			}
			out := storedReport(t, p)
			require.Equal(t, tc.want, out.LastRunStatus)
			require.Nil(t, out.RetryAt)
			require.Equal(t, tc.attempts, attempts.Load())
			require.NotContains(t, out.LastError, "provider-verdict")
			if tc.code == 0 {
				require.Contains(t, out.LastError, "duplicate")
			}
		})
	}
}

func TestReportScheduleDeliveryRecoversInterruptedEmailWithoutResending(t *testing.T) {
	h, p, _, ctx, s := reportDeliveryFixture(t)
	cfg, attempts := reportSMTP(t, []int{250}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	now := time.Date(2026, 10, 19, 9, 0, 1, 0, time.UTC)
	s.LastRunStatus = "running"
	s.RunWindowEnd = &now
	s.RunAttempts = 1
	s.LastOccurrenceKey = currentReportScheduleOccurrenceKey(s, now)
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	h.runDueReportSchedulesForOrg(ctx, "default", now.Add(time.Minute))
	require.Equal(t, "unconfirmed", storedReport(t, p).LastRunStatus)
	require.Zero(t, attempts.Load())
	w := runDeliveryRequest(h, ctx, true)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.EqualValues(t, 1, attempts.Load())
	require.Equal(t, "ok", storedReport(t, p).LastRunStatus)
}

func TestReportScheduleDeliveryEditCancelsRetryAndPreservesOutcome(t *testing.T) {
	h, p, _, ctx, s := reportDeliveryFixture(t)
	cfg, attempts := reportSMTP(t, []int{451}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	require.Equal(t, http.StatusAccepted, runDeliveryRequest(h, ctx, false).Code)
	pending := storedReport(t, p)
	lastRun := *pending.LastRunAt
	pending.Enabled = false
	pending.LastRunStatus = "ok"
	pending.LastReportPath = "/forged"
	pending.RunAttempts = 99
	r := httptest.NewRequest(http.MethodPut, "/", encodeScheduleBody(t, pending)).WithContext(ctx)
	r.SetPathValue("id", "delivery")
	w := httptest.NewRecorder()
	h.HandleUpdateReportSchedule(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out := storedReport(t, p)
	require.Equal(t, "failed", out.LastRunStatus)
	require.Equal(t, lastRun, *out.LastRunAt)
	require.NotEqual(t, "/forged", out.LastReportPath)
	require.Equal(t, 1, out.RunAttempts)
	require.Nil(t, out.RetryAt)
	require.Nil(t, out.RunWindowEnd)
	h.runDueReportSchedulesForOrg(ctx, "default", time.Now().Add(time.Hour))
	require.EqualValues(t, 1, attempts.Load())
	require.Equal(t, http.StatusConflict, runDeliveryRequest(h, ctx, true).Code)
}

func TestReportScheduleDeliveryPersistsBeforeSMTPAndCannotResurrectDeletedSchedule(t *testing.T) {
	h, p, _, ctx, s := reportDeliveryFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var observed atomic.Bool
	cfg, _ := reportSMTP(t, []int{250}, func() {
		store, err := p.LoadReportScheduleStore()
		if err == nil && len(store.Schedules) == 1 && store.Schedules[0].LastRunStatus == "running" {
			observed.Store(true)
		}
		close(entered)
		<-release
	})
	t.Cleanup(unblock)
	require.NoError(t, p.SaveEmailConfig(cfg))
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	runDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { runDone <- runDeliveryRequest(h, ctx, false) }()
	select {
	case <-entered:
	case result := <-runDone:
		t.Fatalf("report ended before SMTP: %d %s", result.Code, result.Body.String())
	case <-time.After(15 * time.Second):
		t.Fatal("report did not reach SMTP")
	}
	deleteDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRequest(http.MethodDelete, "/", nil).WithContext(ctx)
		r.SetPathValue("id", "delivery")
		w := httptest.NewRecorder()
		h.HandleDeleteReportSchedule(w, r)
		deleteDone <- w
	}()
	select {
	case <-deleteDone:
		t.Error("delete passed an active delivery")
	case <-time.After(30 * time.Millisecond):
	}
	unblock()
	require.Equal(t, http.StatusOK, (<-runDone).Code)
	require.Equal(t, http.StatusNoContent, (<-deleteDone).Code)
	require.True(t, observed.Load())
	store, err := p.LoadReportScheduleStore()
	require.NoError(t, err)
	require.Empty(t, store.Schedules)
}

func TestReportScheduleDeliveryOversizeAndDiskOutcomes(t *testing.T) {
	h, p, _, ctx, s := reportDeliveryFixture(t)
	cfg, attempts := reportSMTP(t, []int{250}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	_, err := sendScheduledReportEmail(p, s, generatedMultiReport{Data: make([]byte, reportScheduleAttachmentLimitBytes+1)}, "/owned/report.pdf")
	require.ErrorContains(t, err, "15 MiB")
	require.Zero(t, attempts.Load())
	s.Delivery.Method = "disk"
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	require.Equal(t, http.StatusOK, runDeliveryRequest(h, ctx, false).Code)
	out := storedReport(t, p)
	require.Equal(t, "saved", out.LastDeliveryStatus)
	require.Equal(t, "ok", out.LastRunStatus)
	require.FileExists(t, out.LastReportPath)
	require.Zero(t, attempts.Load())
}

func TestReportScheduleDeliveryRechecksEntitlementBeforeRetry(t *testing.T) {
	h, p, _, ctx, s := reportDeliveryFixture(t)
	cfg, attempts := reportSMTP(t, []int{451}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	require.Equal(t, http.StatusAccepted, runDeliveryRequest(h, ctx, false).Code)
	pending := storedReport(t, p)
	service := newLicenseService()
	h.SetCommercialLicenseResolver(func(context.Context) *licenseService { return service })
	h.runDueReportSchedulesForOrg(ctx, "default", pending.RetryAt.Add(time.Second))
	out := storedReport(t, p)
	require.Equal(t, "failed", out.LastRunStatus)
	require.Contains(t, out.LastError, "advanced reporting entitlement required")
	require.Nil(t, out.RetryAt)
	require.EqualValues(t, 1, attempts.Load(), "expired entitlement must not dispatch another message")
}
