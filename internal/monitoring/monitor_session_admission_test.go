package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// The real password constructor, normal retry, poll executor and broadcast
// projection feed browser proof of the unchanged PBS consuming surface. No
// guest command, backup, installed service, database or provider is exercised.
func TestPBSInvalidSessionKeepsFirstPollBoundary(t *testing.T) {
	var logins, reads atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/access/ticket" {
			if logins.Add(1) == 1 {
				// A syntactically valid 200 with no session, not a credential
				// refusal and not an invitation to replay another encoding.
				fmt.Fprint(w, `{"data":{}}`)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
			return
		}
		reads.Add(1)
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Cookie"), "fixture-ticket") {
			t.Errorf("non-ordinary or unauthenticated PBS fixture read: %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/api2/json/version":
			fmt.Fprint(w, `{"data":{"version":"3.4.2"}}`)
		case "/api2/json/nodes/localhost/status":
			fmt.Fprint(w, `{"data":{"cpu":0.15,"memory":{"used":512,"total":1024},"uptime":120}}`)
		case "/api2/json/admin/datastore":
			fmt.Fprint(w, `{"data":[{"store":"backups","total":1000,"used":250,"avail":750}]}`)
		case "/api2/json/admin/datastore/backups/status":
			fmt.Fprint(w, `{"data":{"status":"available","total":1000,"used":250,"avail":750}}`)
		default:
			fmt.Fprint(w, `{"data":[]}`)
		}
	}))
	defer server.Close()
	pin := sha256.Sum256(server.Certificate().Raw)
	instance := config.PBSInstance{Name: "session-pbs", Host: server.URL, User: "fixture@pbs", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(pin[:]), MonitorDatastores: true}
	monitor := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	project := func(phase string) {
		snapshot, scope := monitor.currentStateWithScope()
		frontend := monitor.buildBroadcastFrontendStateFromSnapshot(snapshot, scope)
		wire, err := json.Marshal(struct {
			Phase string               `json:"phase"`
			State models.StateFrontend `json:"state"`
		}{phase, frontend})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PBS_SESSION_ADMISSION_PROJECTION_JSON=%s", wire)
	}
	monitor.initPBSClients(monitor.config)
	assertPBSConnectionProjection(t, monitor, instance.Name, false, "offline")
	if logins.Load() != 1 || reads.Load() != 0 || len(monitor.pbsClients) != 0 || len(newPBSPollProvider().ListInstances(monitor)) != 0 {
		t.Fatal("unusable login success published a polling client or dispatched API/form work")
	}
	project("invalid-session")

	originalDelays := connRetryDelays
	connRetryDelays = []time.Duration{time.Millisecond}
	defer func() { connRetryDelays = originalDelays }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	monitor.retryFailedConnections(ctx)
	if logins.Load() != 2 || reads.Load() != 0 || len(newPBSPollProvider().ListInstances(monitor)) != 1 {
		t.Fatal("later normal retry did not install its accepted session exactly once")
	}
	assertPBSConnectionProjection(t, monitor, instance.Name, false, "offline")
	project("awaiting-poll")
	task, err := newPBSPollProvider().BuildPollTask(monitor, instance.Name)
	if err != nil {
		t.Fatal(err)
	}
	newRealExecutor(monitor).Execute(ctx, task)
	assertPBSConnectionProjection(t, monitor, instance.Name, true, "online")
	actual := pbsInstanceByName(t, monitor.state.GetSnapshot(), instance.Name)
	if actual.CPU != 0.15 || actual.Memory != 50 || len(actual.Datastores) != 1 || actual.Datastores[0].Usage != 25 || logins.Load() != 2 || !reflect.DeepEqual(instance, monitor.config.PBSInstances[0]) {
		t.Fatal("ordinary first poll lost readings, identity or saved trust")
	}
	project("recovered")
}
