package truenas

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const verifiedHTTPSChild = "PULSE_TRUENAS_VERIFIED_HTTPS_CHILD"
const verifiedHTTPSResultPrefix = "TRUENAS_VERIFIED_HTTPS_RESULT "

type verifiedHTTPSResult struct {
	Errors              map[string]string
	VerificationEnabled bool
	Hostname            string
	Version             string
	PoolNames           []string
	DatasetIDs          []string
	Status              TransportStatus
	Closed              bool
}

// Use the real WSS negotiation, TestConnection and REST inventory clients, not
// protocolFixtureClient (which disables certificate verification). A subprocess
// gives each case its own system-root cache: Go caches SSL_CERT_FILE on first
// use, so changing it inside the ordinary package test process is insufficient.
func TestTrueNASVerifiedHTTPSConnectionAndInventory(t *testing.T) {
	if configJSON := os.Getenv(verifiedHTTPSChild); configJSON != "" {
		var config ClientConfig
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			t.Fatal(err)
		}
		result := runVerifiedHTTPSClient(t, config)
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(verifiedHTTPSResultPrefix + string(encoded))
		return
	}

	for _, test := range []struct {
		name            string
		version         string
		missingEndpoint bool
		untrusted       bool
		wrongHostname   bool
		wrongPin        bool
		replaceAfterWSS bool
		wantError       string
		wantRequests    map[string]int
	}{
		{name: "core12_redirect", version: "TrueNAS-12.0-U5"},
		{name: "core12_missing_endpoint", version: "TrueNAS-12.0-U5", missingEndpoint: true},
		{name: "core13_redirect", version: "TrueNAS-13.0-U6.4"},
		{name: "modern_redirect_fails_closed", version: "TrueNAS-SCALE-25.10.4", wantError: "refusing REST downgrade",
			wantRequests: map[string]int{"/api/current": 4, "/api/v2.0/system/info": 4}},
		{name: "unknown_redirect_fails_closed", version: "unidentified", wantError: "refusing REST downgrade",
			wantRequests: map[string]int{"/api/current": 4, "/api/v2.0/system/info": 4}},
		{name: "untrusted_certificate", version: "TrueNAS-12.0-U5", untrusted: true, wantError: "certificate signed by unknown authority"},
		{name: "wrong_hostname", version: "TrueNAS-12.0-U5", wrongHostname: true, wantError: "certificate is valid for"},
		{name: "wrong_pin", version: "TrueNAS-12.0-U5", wrongPin: true, wantError: "fingerprint mismatch"},
		{name: "rest_certificate_replaced", version: "TrueNAS-12.0-U5", replaceAfterWSS: true,
			wantError: "certificate signed by unknown authority", wantRequests: map[string]int{"/api/current": 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			certificate, rootPEM := verifiedHTTPSCertificate(t, test.wrongHostname)
			otherCertificate, otherRootPEM := verifiedHTTPSCertificate(t, false)
			var replaced atomic.Bool
			var mu sync.Mutex
			requests := make(map[string]int)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests[r.URL.Path]++
				mu.Unlock()
				if r.TLS == nil || r.Method != http.MethodGet {
					t.Errorf("unexpected non-TLS or mutating request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/current" {
					if r.Header.Get("Authorization") != "" {
						t.Error("sent credentials before the WebSocket handshake")
					}
					if test.replaceAfterWSS {
						replaced.Store(true)
					}
					if test.missingEndpoint {
						http.NotFound(w, r)
					} else {
						http.Redirect(w, r, "/ui/", http.StatusFound)
					}
					return
				}
				if r.Header.Get("Authorization") != "Bearer verified-https-fixture-key" {
					t.Errorf("missing expected fixture REST authentication on %s", r.URL.Path)
					http.Error(w, "unauthorised", http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/api/v2.0/system/info":
					_, _ = fmt.Fprintf(w, `{"hostname":"core-fixture","version":%q,"uptime_seconds":1}`, test.version)
				case "/api/v2.0/pool":
					_, _ = io.WriteString(w, `[{"id":1,"name":"core-tank","status":"ONLINE"}]`)
				case "/api/v2.0/pool/dataset":
					_, _ = io.WriteString(w, `[{"id":"core-tank/data","name":"core-tank/data","used":{"parsed":123},"available":{"parsed":456},"readonly":{"parsed":false}}]`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			server.Config.ErrorLog = log.New(io.Discard, "", 0) // Expected TLS refusals are checked below.
			server.TLS = &tls.Config{
				MinVersion: tls.VersionTLS12,
				GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
					selected := certificate
					if replaced.Load() {
						selected = otherCertificate
					}
					return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{selected}}, nil
				},
			}
			server.StartTLS()
			t.Cleanup(server.Close)

			trustDir := t.TempDir()
			if test.untrusted {
				rootPEM = otherRootPEM
			}
			rootFile := filepath.Join(trustDir, "root.pem")
			if err := os.WriteFile(rootFile, rootPEM, 0600); err != nil {
				t.Fatal(err)
			}
			config := ClientConfig{Host: server.URL, UseHTTPS: true, APIKey: "verified-https-fixture-key", Timeout: 2 * time.Second}
			if test.wrongPin {
				config.Fingerprint = strings.Repeat("0", 64)
			}
			configJSON, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestTrueNASVerifiedHTTPSConnectionAndInventory$")
			command.Env = append(os.Environ(), verifiedHTTPSChild+"="+string(configJSON), "SSL_CERT_FILE="+rootFile, "SSL_CERT_DIR="+trustDir)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("isolated verified client: %v\n%s", err, output)
			}
			var result verifiedHTTPSResult
			found := false
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, verifiedHTTPSResultPrefix) {
					if found {
						t.Fatal("duplicate child result")
					}
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, verifiedHTTPSResultPrefix)), &result); err != nil {
						t.Fatal(err)
					}
					found = true
				}
			}
			if !found {
				t.Fatalf("missing isolated result: %s", output)
			}
			mu.Lock()
			defer mu.Unlock()
			t.Logf("verification result: %+v; requests: %v", result, requests)
			if !result.VerificationEnabled || !result.Closed {
				t.Fatal("certificate verification disabled or client not closed")
			}
			if test.wantError != "" {
				for _, operation := range []string{"connection", "system", "pools", "datasets"} {
					if !strings.Contains(result.Errors[operation], test.wantError) {
						t.Errorf("%s error = %q, want %q", operation, result.Errors[operation], test.wantError)
					}
				}
				if result.Status.Connected || result.Status.Mode != TransportUnknown || len(result.PoolNames) != 0 || len(result.DatasetIDs) != 0 {
					t.Error("failed connection admitted a transport or inventory")
				}
				want := test.wantRequests
				if want == nil {
					want = map[string]int{}
				}
				if !reflect.DeepEqual(requests, want) {
					t.Errorf("requests = %v, want %v (no unauthorised inventory or redirect following)", requests, want)
				}
				return
			}
			if len(result.Errors) != 0 {
				t.Fatalf("verified connection/inventory failed: %v", result.Errors)
			}
			if result.Hostname != "core-fixture" || result.Version != test.version ||
				!reflect.DeepEqual(result.PoolNames, []string{"core-tank"}) ||
				!reflect.DeepEqual(result.DatasetIDs, []string{"core-tank/data"}) {
				t.Error("connection did not return the fixture's system and inventory")
			}
			if result.Status.Mode != TransportLegacyREST || !result.Status.Connected || !result.Status.TLS || result.Status.ApplianceVersion != test.version {
				t.Errorf("unexpected transport status: %+v", result.Status)
			}
			want := map[string]int{"/api/current": 1, "/api/v2.0/system/info": 3, "/api/v2.0/pool": 1, "/api/v2.0/pool/dataset": 1}
			if !reflect.DeepEqual(requests, want) {
				t.Errorf("requests = %v, want %v (one negotiation, no UI redirect or writes)", requests, want)
			}
		})
	}
}

func runVerifiedHTTPSClient(t *testing.T, config ClientConfig) verifiedHTTPSResult {
	t.Helper()
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	transport := client.httpClient.Transport.(*http.Transport)
	result := verifiedHTTPSResult{
		Errors: make(map[string]string),
		VerificationEnabled: !client.config.InsecureSkipVerify && !client.allowInsecureRPC &&
			transport.TLSClientConfig != nil && !transport.TLSClientConfig.InsecureSkipVerify,
	}
	recordError := func(operation string, err error) {
		if err != nil {
			result.Errors[operation] = err.Error()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	recordError("connection", client.TestConnection(ctx))
	system, err := client.GetSystemInfo(ctx)
	recordError("system", err)
	if system != nil {
		result.Hostname, result.Version = system.Hostname, system.Version
	}
	pools, err := client.GetPools(ctx)
	recordError("pools", err)
	for _, pool := range pools {
		result.PoolNames = append(result.PoolNames, pool.Name)
	}
	datasets, err := client.GetDatasets(ctx)
	recordError("datasets", err)
	for _, dataset := range datasets {
		result.DatasetIDs = append(result.DatasetIDs, dataset.ID)
	}
	result.Status = client.TransportStatus()
	client.Close()
	result.Closed = !client.TransportStatus().Connected
	return result
}

func verifiedHTTPSCertificate(t *testing.T, wrongHostname bool) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TrueNAS test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	address := "127.0.0.1"
	if wrongHostname {
		address = "192.0.2.1"
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), NotBefore: root.NotBefore, NotAfter: root.NotAfter,
		IPAddresses: []net.IP{net.ParseIP(address)}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{leafDER, rootDER}, PrivateKey: key}, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})
}
