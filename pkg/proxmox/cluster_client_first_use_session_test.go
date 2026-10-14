package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type firstUseSessionFixture struct {
	server                    *httptest.Server
	cert                      atomic.Pointer[tls.Certificate]
	handshakes, logins, nodes atomic.Int32
	refuse                    atomic.Int32
	gateAt                    atomic.Int32
	entered, resume           chan struct{}
	form                      bool
	token                     bool
}

func newFirstUseSessionFixture(t *testing.T, token, form bool) *firstUseSessionFixture {
	t.Helper()
	f := &firstUseSessionFixture{entered: make(chan struct{}), resume: make(chan struct{}), token: token, form: form}
	f.cert.Store(clusterTrustCertificate(t))
	f.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			f.logins.Add(1)
			if token {
				t.Error("API token triggered password login")
			}
			if form && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				w.WriteHeader(http.StatusUnsupportedMediaType)
				return
			}
			var credentials map[string]string
			if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
					t.Error(err)
				}
			} else {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				credentials = map[string]string{"username": r.Form.Get("username"), "password": r.Form.Get("password")}
			}
			if credentials["username"] != "fixture@pam" || credentials["password"] != "fixture-password" {
				t.Error("password authentication changed")
			}
			switch f.refuse.Load() {
			case 401, 403:
				w.WriteHeader(int(f.refuse.Load()))
				fmt.Fprint(w, `{"message":"fixture login refused"}`)
				return
			case 200:
				fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket"}}`)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
		case "/api2/json/nodes":
			f.nodes.Add(1)
			if token {
				if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
					t.Error("token authentication changed")
				}
			} else if r.Header.Get("Cookie") != "PVEAuthCookie=fixture-ticket" || r.Header.Get("CSRFPreventionToken") != "" {
				t.Error("ordinary read lost the coherent fixture session")
			}
			fmt.Fprint(w, `{"data":[{"node":"member","status":"online","cpu":0.25}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	f.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	f.server.TLS = &tls.Config{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
		n := f.handshakes.Add(1)
		cert := f.cert.Load()
		if n == f.gateAt.Load() {
			close(f.entered)
			<-f.resume
		}
		return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{*cert}}, nil
	}}
	f.server.StartTLS()
	t.Cleanup(f.server.Close)
	return f
}

func firstUseSessionConfig(primary string, pin string, token bool) ClientConfig {
	cfg := ClientConfig{Host: primary, User: "fixture@pam", Password: "fixture-password", VerifySSL: true, Fingerprint: pin, Timeout: 2 * time.Second}
	if token {
		cfg.Password = ""
		cfg.TokenName = "fixture@pve!pulse"
		cfg.TokenValue = "fixture"
	}
	return cfg
}

func closeFirstUseClusterClients(t *testing.T, cc *ClusterClient) {
	t.Helper()
	t.Cleanup(func() {
		cc.mu.RLock()
		defer cc.mu.RUnlock()
		for _, c := range cc.clients {
			if c != nil {
				c.httpClient.CloseIdleConnections()
			}
		}
	})
}

func runFirstUsePhase(t *testing.T, phase string, cfg ClientConfig, f *firstUseSessionFixture, pins map[string]string) *ClusterClient {
	t.Helper()
	var cc *ClusterClient
	if phase == "startup" {
		primary := newFirstUseSessionFixture(t, f.token, f.form)
		cfg.Host = primary.server.URL
		cfg.Fingerprint = clusterTrustPin(primary.cert.Load())
		cc = NewClusterClient("first-use", cfg, []string{primary.server.URL, f.server.URL}, pins)
	} else {
		cc = NewClusterClient("first-use", cfg, []string{f.server.URL}, pins)
		if phase == "recovery" {
			cc.nodeHealth[f.server.URL] = false
			cc.lastError[f.server.URL] = "fixture unavailable"
			cc.recoverUnhealthyNodes(context.Background())
		} else {
			if _, err := cc.GetNodes(context.Background()); err != nil {
				t.Errorf("lazy first-use read: %v", err)
			}
		}
	}
	closeFirstUseClusterClients(t, cc)
	return cc
}

func testClusterFirstUseSessionAdmission(t *testing.T) {
	for _, phase := range []string{"startup", "recovery", "selection"} {
		for _, mode := range []string{"password-json", "password-form", "api-token"} {
			t.Run(phase+"/"+mode, func(t *testing.T) {
				token, form := mode == "api-token", mode == "password-form"
				f := newFirstUseSessionFixture(t, token, form)
				primaryPin := clusterTrustPin(clusterTrustCertificate(t))
				cfg := firstUseSessionConfig("https://primary.session.invalid", primaryPin, token)
				saved := map[string]string{}
				cc := runFirstUsePhase(t, phase, cfg, f, saved)
				cc.mu.RLock()
				client := cc.clients[f.server.URL]
				healthy := cc.nodeHealth[f.server.URL]
				cc.mu.RUnlock()
				if !healthy || client == nil || cc.getEndpointFingerprint(f.server.URL) != clusterTrustPin(f.cert.Load()) {
					t.Fatalf("declared new member has no usable pinned session: healthy=%t client=%t logins=%d nodes=%d", healthy, client != nil, f.logins.Load(), f.nodes.Load())
				}
				if len(saved) != 0 || cc.config.VerifySSL != true || cc.config.Fingerprint == cc.getEndpointFingerprint(f.server.URL) {
					t.Fatal("first use changed saved trust or primary policy")
				}
				if client.config.Timeout != cfg.Timeout {
					t.Fatal("retained polling timeout changed")
				}
				loginWant := int32(2)
				if form {
					loginWant = 4
				}
				if token {
					loginWant = 0
				}
				nodesWant := int32(1)
				if phase == "selection" {
					nodesWant++
				}
				if f.logins.Load() != loginWant || f.nodes.Load() != nodesWant || f.handshakes.Load() != 4-int32(boolInt(token && phase != "selection")) {
					t.Fatalf("probe/capture/full admission counts: login=%d nodes=%d handshakes=%d", f.logins.Load(), f.nodes.Load(), f.handshakes.Load())
				}
				// Use the actually retained full-timeout session through ordinary routing.
				cc.mu.Lock()
				for ep := range cc.nodeHealth {
					if ep != f.server.URL {
						cc.nodeHealth[ep] = false
						cc.lastHealthCheck[ep] = time.Now()
					}
				}
				cc.mu.Unlock()
				nodes, err := cc.GetNodes(context.Background())
				if err != nil || len(nodes) != 1 || nodes[0].CPU != 0.25 || f.logins.Load() != loginWant || f.nodes.Load() != nodesWant+1 {
					t.Fatalf("ordinary member read failed or logged in again: %+v %v", nodes, err)
				}
				t.Logf("FIRST_USE_SESSION_FACTS phase=%s mode=%s saved-pins-unchanged=true primary-pin-preserved=true capture-http=0 retained-full-session=true nodes=%d logins=%d", phase, mode, f.nodes.Load(), f.logins.Load())
			})
		}
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestClusterFirstUseSessionRefusalKeepsTrust(t *testing.T) {
	for _, failure := range []int32{401, 403, 200} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			f := newFirstUseSessionFixture(t, false, false)
			f.refuse.Store(failure)
			cfg := firstUseSessionConfig("https://primary.session.invalid", clusterTrustPin(clusterTrustCertificate(t)), false)
			cc := runFirstUsePhase(t, "recovery", cfg, f, nil)
			if cc.GetHealthStatus()[f.server.URL] || cc.clients[f.server.URL] != nil || f.logins.Load() != 1 || f.nodes.Load() != 0 {
				t.Fatalf("refused first-use session became healthy or replayed: login=%d nodes=%d", f.logins.Load(), f.nodes.Load())
			}
			pin := cc.getEndpointFingerprint(f.server.URL)
			if pin != clusterTrustPin(f.cert.Load()) || cc.lastError[f.server.URL] == "" {
				t.Fatal("refusal lost its fixed pin or diagnostic")
			}
			// Eligibility changes only in the fixture; no native clock/fence reset.
			f.refuse.Store(0)
			delete(cc.lastHealthCheck, f.server.URL)
			before := f.handshakes.Load()
			cc.recoverUnhealthyNodes(context.Background())
			if !cc.GetHealthStatus()[f.server.URL] || cc.clients[f.server.URL] == nil || cc.getEndpointFingerprint(f.server.URL) != pin || f.handshakes.Load() != before+2 || f.logins.Load() != 3 || f.nodes.Load() != 1 {
				t.Fatal("separate ordinary recovery failed, recaptured trust or replayed the refusal")
			}
		})
	}
}

func TestClusterPasswordEstablishedPinsAreNotFirstUse(t *testing.T) {
	for _, phase := range []string{"startup", "recovery", "selection"} {
		for _, role := range []string{"primary", "member"} {
			t.Run(phase+"/"+role, func(t *testing.T) {
				f := newFirstUseSessionFixture(t, false, false)
				trusted := f.cert.Load()
				pin := clusterTrustPin(trusted)
				f.cert.Store(clusterTrustCertificate(t))
				cfg := firstUseSessionConfig(f.server.URL, pin, false)
				pins := map[string]string{}
				if role == "member" {
					cfg.Host = "https://primary.session.invalid"
					pins[f.server.URL] = pin
				}
				cc := NewClusterClient("known-password", cfg, []string{f.server.URL}, pins)
				closeFirstUseClusterClients(t, cc)
				switch phase {
				case "startup":
					cc.endpoints = append(cc.endpoints, "http://127.0.0.1:1")
					cc.initialHealthCheck(context.Background())
				case "recovery":
					cc.nodeHealth[f.server.URL] = false
					cc.recoverUnhealthyNodes(context.Background())
				case "selection":
					_, _ = cc.GetNodes(context.Background())
				}
				if f.logins.Load() != 0 || f.nodes.Load() != 0 || f.handshakes.Load() != 1 || cc.getEndpointFingerprint(f.server.URL) != pin || cc.GetHealthStatus()[f.server.URL] {
					t.Fatal("known leaf change captured new trust or sent a password")
				}
				f.cert.Store(trusted)
				delete(cc.lastHealthCheck, f.server.URL)
				cc.recoverUnhealthyNodes(context.Background())
				if !cc.GetHealthStatus()[f.server.URL] || cc.getEndpointFingerprint(f.server.URL) != pin || f.logins.Load() != 2 || f.nodes.Load() != 1 {
					t.Fatal("re-presenting trusted leaf did not recover")
				}
			})
		}
	}
}

func TestClusterFirstUsePasswordCaptureCancellation(t *testing.T) {
	f := newFirstUseSessionFixture(t, false, false)
	f.gateAt.Store(2)
	cfg := firstUseSessionConfig("https://primary.session.invalid", clusterTrustPin(clusterTrustCertificate(t)), false)
	cc := NewClusterClient("cancel-first-use", cfg, []string{f.server.URL}, nil)
	closeFirstUseClusterClients(t, cc)
	cc.nodeHealth[f.server.URL] = false
	cc.lastError[f.server.URL] = "prior failure"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { cc.recoverUnhealthyNodes(ctx); close(done) }()
	select {
	case <-f.entered:
	case <-time.After(5 * time.Second):
		cancel()
		close(f.resume)
		<-done
		t.Fatal("password construction mismatch did not reach credential-free capture")
	}
	cancel()
	close(f.resume)
	<-done
	if len(cc.endpointFingerprints) != 0 || cc.lastError[f.server.URL] != "prior failure" || cc.GetHealthStatus()[f.server.URL] || f.logins.Load() != 0 || f.nodes.Load() != 0 {
		t.Fatal("cancelled capture installed trust, changed failure or sent authentication")
	}
}

func TestClusterFirstUseConcurrentSelectionSharesConstruction(t *testing.T) {
	f := newFirstUseSessionFixture(t, false, false)
	f.gateAt.Store(2)
	cfg := firstUseSessionConfig("https://primary.session.invalid", clusterTrustPin(clusterTrustCertificate(t)), false)
	cc := NewClusterClient("joined-first-use", cfg, []string{f.server.URL}, nil)
	closeFirstUseClusterClients(t, cc)
	done := make(chan error, 8)
	read := func() {
		nodes, err := cc.GetNodes(context.Background())
		if err == nil && (len(nodes) != 1 || nodes[0].CPU != 0.25) {
			err = fmt.Errorf("lost ordinary member reading")
		}
		done <- err
	}
	go read()
	select {
	case <-f.entered:
	case <-time.After(5 * time.Second):
		close(f.resume)
		<-done
		t.Fatal("first selection did not reach capture")
	}
	for i := 0; i < 7; i++ {
		go read()
	}
	// Health inspection remains usable while the credential-free capture is
	// held. Closing this synthetic handshake does not cancel any native work.
	_ = cc.GetHealthStatusWithErrors()
	close(f.resume)
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	if f.logins.Load() != 2 || f.nodes.Load() != 9 || f.handshakes.Load() != 4 {
		t.Fatalf("concurrent selection duplicated capture/login: logins=%d nodes=%d handshakes=%d", f.logins.Load(), f.nodes.Load(), f.handshakes.Load())
	}
}
