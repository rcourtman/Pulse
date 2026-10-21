package configapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Scoped IPv6 literal -> real local TLS -> registered saved/draft handlers.
// The proof VM owns the selected loopback interface; this is not a native link-local estate.
// The literal must never be turned into a DNS answer without its interface.
// Existing broader endpoint and guest-response fixtures remain unchanged.
// Real local TLS -> ordinary saved/draft connection handlers. All credentials
// are generated fixture labels; no monitor, persistence, database or native
// guest exists. Wire responses are emitted for the unchanged production editor.
func TestScopedLiteralConnectionJourney(t *testing.T) {
	zone := scopedLiteralLoopbackZone(t)
	for _, network := range []string{"tcp6"} {
		address := "127.0.0.1:0"
		if network == "tcp6" {
			address = "[::1]:0"
		}
		for _, kind := range []string{"pve", "pbs", "pmg"} {
			t.Run(network+"/"+kind, func(t *testing.T) {
				var reads, logins atomic.Int32
				listener, err := net.Listen(network, address)
				if err != nil {
					t.Fatal(err)
				}
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Path == "/api2/json/access/ticket" {
						logins.Add(1)
						if r.Method != http.MethodPost {
							t.Error("login lost its method")
						}
						var payload map[string]string
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["username"] != "fixture@pam" || payload["password"] != "synthetic-password" {
							t.Error("login credentials changed")
						}
						fmt.Fprint(w, `{"data":{"ticket":"synthetic-ticket","CSRFPreventionToken":"synthetic-csrf"}}`)
						return
					}
					reads.Add(1)
					if r.Method != http.MethodGet {
						t.Error("test made an unexpected mutation")
					}
					if r.Header.Get("Cookie") != strings.ToUpper(kind)+"AuthCookie=synthetic-ticket" {
						t.Error("saved/draft password authentication was lost")
					}
					switch r.URL.Path {
					case "/api2/json/nodes":
						fmt.Fprint(w, `{"data":[{"node":"fixture","status":"online"}]}`)
					case "/api2/json/cluster/status":
						fmt.Fprint(w, `{"data":[{"type":"node","name":"fixture","local":1,"online":1}]}`)
					case "/api2/json/admin/datastore":
						fmt.Fprint(w, `{"data":[{"store":"fixture"}]}`)
					case "/api2/json/admin/datastore/fixture/rrd":
						fmt.Fprint(w, `{"data":[{"time":1,"dedup_factor":2}]}`)
					case "/api2/json/admin/datastore/fixture/status":
						fmt.Fprint(w, `{"data":{"total":100,"used":20,"avail":80,"deduplication-factor":2}}`)
					case "/api2/json/version":
						fmt.Fprint(w, `{"data":{"version":"9.1","release":"1"}}`)
					case "/api2/json/statistics/mail":
						fmt.Fprint(w, `{"data":{"count":10,"count_in":6,"count_out":4}}`)
					case "/api2/json/config/cluster/status":
						if r.URL.Query().Get("list_single_node") != "1" {
							t.Error("PMG cluster probe lost its configured single-node query")
						}
						fmt.Fprint(w, `{"data":[{"name":"fixture","type":"node","ip":"127.0.0.1"}]}`)
					case "/api2/json/quarantine/spamstatus":
						fmt.Fprint(w, `{"data":{"count":1,"avgbytes":1024,"mbytes":0.001}}`)
					default:
						t.Errorf("unexpected provider read: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				server.Listener.Close()
				server.Listener = listener
				server.Config.ErrorLog = stdlog.New(io.Discard, "", 0)
				server.StartTLS()
				defer server.Close()
				host := strings.Replace(server.URL, "https://", "HTTPS://", 1)
				host = strings.Replace(host, "[::1]", "[::1%25"+zone+"]", 1)
				pin := fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
				cfg := &config.Config{}
				cfg.PVEInstances = []config.PVEInstance{{Name: "fixture", Host: host, User: "fixture@pam", Password: "synthetic-password", VerifySSL: true, Fingerprint: pin}}
				cfg.PBSInstances = []config.PBSInstance{{Name: "fixture", Host: host, User: "fixture@pam", Password: "synthetic-password", VerifySSL: true, Fingerprint: pin}}
				cfg.PMGInstances = []config.PMGInstance{{Name: "fixture", Host: host, User: "fixture@pam", Password: "synthetic-password", VerifySSL: true, Fingerprint: pin}}
				h := &ConfigHandlers{defaultConfig: cfg}
				before, _ := json.Marshal(cfg)
				for _, mode := range []string{"saved", "draft"} {
					for _, phase := range []string{"matching-uppercase", "wrong-pin", "recovered"} {
						request := map[string]any{"type": kind, "name": strings.ToUpper(kind) + " fixture", "host": host, "user": "fixture@pam", "password": "synthetic-password", "verifySSL": true, "fingerprint": pin}
						requestPin, requestHost := pin, host
						switch phase {
						case "wrong-pin":
							requestPin = strings.Repeat("0", 64)
						}
						request["fingerprint"], request["host"] = requestPin, requestHost
						data, _ := json.Marshal(request)
						path := "/api/config/nodes/test-connection"
						if mode == "saved" {
							path = "/api/config/nodes/" + kind + "-0/test"
							data = nil
							cfg.PVEInstances[0].Host, cfg.PVEInstances[0].Fingerprint = requestHost, requestPin
							cfg.PBSInstances[0].Host, cfg.PBSInstances[0].Fingerprint = requestHost, requestPin
							cfg.PMGInstances[0].Host, cfg.PMGInstances[0].Fingerprint = requestHost, requestPin
						}
						ownBefore, _ := json.Marshal(cfg)
						readStart, loginStart := reads.Load(), logins.Load()
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
						if mode == "saved" {
							h.HandleTestNode(rec, req)
						} else {
							// Match router_routes_registration.go: the editor posts
							// to test-connection, not the older test-config route.
							h.HandleTestConnection(rec, req)
						}
						success := phase == "matching-uppercase" || phase == "recovered"
						wantStatus := http.StatusBadRequest
						if success {
							wantStatus = http.StatusOK
						}
						var response map[string]any
						message := strings.TrimSpace(rec.Body.String())
						if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
							if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
								t.Fatal(err)
							}
							message, _ = response["message"].(string)
						}
						if rec.Code != wantStatus || (response["status"] == "success") != success {
							t.Errorf("%s/%s: unexpected response code=%d body=%s", mode, phase, rec.Code, rec.Body.String())
						}
						wantReads := int32(1)
						wantLogins := int32(1)
						if kind == "pbs" {
							wantReads = 3 // listing, RRD and status for the one real fixture datastore
						} else if kind == "pve" && mode == "draft" {
							wantReads, wantLogins = 2, 2 // nodes plus the ordinary cluster-detection session
						} else if kind == "pmg" && mode == "draft" {
							wantReads = 4 // version, mail statistics, cluster and quarantine
						}
						if success && (reads.Load() != readStart+wantReads || logins.Load() != loginStart+wantLogins) {
							t.Errorf("%s/%s: ordinary authenticated reads not completed exactly once per endpoint", mode, phase)
						}
						if !success && (reads.Load() != readStart || logins.Load() != loginStart) {
							t.Errorf("%s/%s: refused trust or scheme reached provider credentials/reads", mode, phase)
						}
						after, _ := json.Marshal(cfg)
						if !reflect.DeepEqual(ownBefore, after) {
							t.Error("connection test mutated saved configuration")
						}
						for _, secret := range []string{"synthetic-password", "synthetic-ticket", "synthetic-csrf"} {
							if strings.Contains(rec.Body.String(), secret) {
								t.Error("connection test disclosed fixture credentials")
							}
						}
						frame := map[string]any{"network": network, "type": kind, "mode": mode, "phase": phase, "path": path, "request": request, "status": rec.Code, "rawBody": rec.Body.String(), "message": message, "contentType": rec.Header().Get("Content-Type"), "reads": reads.Load() - readStart, "logins": logins.Load() - loginStart, "configurationUnchanged": true}
						wire, _ := json.Marshal(frame)
						t.Logf("SCOPED_LITERAL_FRAME=%s", base64.StdEncoding.EncodeToString(wire))
					}
				}
				after, _ := json.Marshal(cfg)
				if !bytes.Equal(before, after) {
					t.Error("journey did not restore its fixture's initial saved trust")
				}
			})
		}
	}
}

func scopedLiteralLoopbackZone(t *testing.T) string {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			return iface.Name
		}
	}
	t.Fatal("no owned loopback interface for scoped IPv6 fixture")
	return ""
}
