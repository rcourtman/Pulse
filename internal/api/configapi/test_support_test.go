package configapi

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agentbinding"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	agentExecBindingVersionKey = agentbinding.VersionKey
	agentExecBindingVersion    = agentbinding.Version
)

type agentExecBindingDecision struct {
	admit         bool
	firstBind     bool
	legacyMigrate bool
}

func evaluateAgentExecBinding(record *config.APITokenRecord, agentID, hostname string) agentExecBindingDecision {
	decision := agentbinding.Evaluate(record, agentID, hostname)
	return agentExecBindingDecision{admit: decision.Admit, firstBind: decision.FirstBind, legacyMigrate: decision.LegacyMigrate}
}

func canBindAgentInstallExecToken(record *config.APITokenRecord, agentID, hostname string) bool {
	return agentbinding.CanBindInstallToken(record, agentID, hostname)
}

func newIPv4TLSServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on tcp4 loopback (tests require local sockets): %v", err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
		TLS:      &tls.Config{},
	}
	server.StartTLS()
	return server
}

func setUnexportedField(t *testing.T, target any, fieldName string, value any) {
	t.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("field %q not found", fieldName)
	}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func newTestMonitor(t *testing.T) (*monitoring.Monitor, *models.State, *monitoring.MetricsHistory) {
	t.Helper()
	monitor := &monitoring.Monitor{}
	state := models.NewState()
	history := monitoring.NewMetricsHistory(10, time.Hour)
	setUnexportedField(t, monitor, "state", state)
	setUnexportedField(t, monitor, "metricsHistory", history)
	return monitor, state, history
}

func syncTestResourceStore(t *testing.T, monitor *monitoring.Monitor, state *models.State) {
	t.Helper()
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(state.GetSnapshot())
	setUnexportedField(t, monitor, "resourceStore", monitoring.ResourceStoreInterface(adapter))
}

func newTokenRecord(t *testing.T, raw string, scopes []string, metadata map[string]string) config.APITokenRecord {
	t.Helper()
	record, err := config.NewAPITokenRecord(raw, "test-token", scopes)
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	if metadata != nil {
		record.Metadata = metadata
	}
	return *record
}

func setMaxMonitoredSystemsLicenseForTests(t *testing.T, _ int) {
	t.Helper()
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")
}

func setMockModeForTest(t *testing.T, enabled bool) {
	t.Helper()
	testutil.SetMockMode(t, enabled)
}
