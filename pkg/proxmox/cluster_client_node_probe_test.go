package proxmox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type nodeProbeFixture struct {
	server               *httptest.Server
	reject               atomic.Bool
	logins, nodes        atomic.Int32
	mode, stage, message string
}

func newNodeProbeFixture(t *testing.T, mode, stage, message string) *nodeProbeFixture {
	t.Helper()
	f := &nodeProbeFixture{mode: mode, stage: stage, message: message}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			f.logins.Add(1)
			if mode == "token" {
				t.Error("token mode attempted password authentication")
			}
			if mode == "form" && r.Header.Get("Content-Type") == "application/json" {
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}
			if f.reject.Load() && stage == "login" {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, message)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
		case "/api2/json/nodes":
			f.nodes.Add(1)
			if mode == "token" {
				if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pam!pulse=fixture-token" {
					t.Error("nodes read lost token authentication")
				}
			} else if r.Header.Get("Cookie") != "PVEAuthCookie=fixture-ticket" {
				t.Error("nodes read lost the accepted password session")
			}
			if f.reject.Load() && stage == "nodes" {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, message)
				return
			}
			fmt.Fprint(w, `{"data":[{"node":"member","status":"online","cpu":0.25}]}`)
		default:
			t.Errorf("probe left ordinary session/nodes paths: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *nodeProbeFixture) config() ClientConfig {
	sum := sha256.Sum256(f.server.Certificate().Raw)
	cfg := ClientConfig{Host: f.server.URL, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: hex.EncodeToString(sum[:]), Timeout: 2 * time.Second}
	if f.mode == "token" {
		cfg.Password = ""
		cfg.TokenName = "fixture@pam!pulse"
		cfg.TokenValue = "fixture-token"
	}
	return cfg
}

func (f *nodeProbeFixture) loginAttempts() int32 {
	if f.mode == "token" {
		return 0
	}
	if f.mode == "form" {
		return 2
	}
	return 1
}
