package kubernetesagent

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

const kubernetesProxyFixtureEnv = "PULSE_KUBERNETES_PROXY_FIXTURE"
const kubernetesProxyFixtureAddress = "fixture-proxy.invalid:3128"

func runKubernetesControlPlaneProxyFixture(t *testing.T) {
	t.Helper()
	// ProxyFromEnvironment caches process state. Give the real client a fresh
	// process rather than depending on test order or changing the global cache.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestKubernetesControlPlaneProxyFixture$", "-test.v")
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		switch strings.ToUpper(key) {
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "REQUEST_METHOD", kubernetesProxyFixtureEnv:
			continue
		}
		cmd.Env = append(cmd.Env, value)
	}
	cmd.Env = append(cmd.Env,
		kubernetesProxyFixtureEnv+"=1",
		"HTTP_PROXY=http://"+kubernetesProxyFixtureAddress,
		"HTTPS_PROXY=http://"+kubernetesProxyFixtureAddress,
		"NO_PROXY=",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated proxy fixture: %v\n%s", err, output)
	}
	t.Logf("isolated proxy fixture:\n%s", output)
}

func TestKubernetesControlPlaneProxyFixture(t *testing.T) {
	if os.Getenv(kubernetesProxyFixtureEnv) != "1" {
		t.Skip("run through TestKubernetesControlPlaneProxy for isolated proxy settings")
	}
	report := agentsk8s.Report{
		Agent:   agentsk8s.AgentInfo{ID: "fixture-agent", Version: "fixture", IntervalSeconds: 30},
		Cluster: agentsk8s.ClusterInfo{ID: "fixture-cluster", Name: "private-inventory-fixture"},
		Nodes:   []agentsk8s.Node{{UID: "fixture-node", Name: "private-node-fixture", Ready: true}},
	}

	for _, tc := range []struct {
		name          string
		url           string
		authoritative bool
	}{
		{"primary-ipv4-basepath", "http://192.168.50.10:7655/pulse", true},
		{"primary-ipv6", "http://[fd00::10]:7655", true},
		{"primary-service-dns", "http://pulse.monitoring.svc:7655", true},
		{"observer-explicit-http", "http://observer.example:7655", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := TargetConfig{URL: tc.url, Token: "fixture-bearer-not-a-secret", Authoritative: tc.authoritative, AllowPlaintextHTTP: !tc.authoritative}
			client, err := newKubernetesHTTPClient(target)
			if err != nil {
				t.Fatal(err)
			}
			transport := client.Transport.(*http.Transport)
			defer transport.CloseIdleConnections()
			parsed, err := url.Parse(target.URL)
			if err != nil {
				t.Fatal(err)
			}
			observed := make(chan kubernetesProxyObservation, 1)
			transport.DialContext = kubernetesProxyFixtureDial(t, observed, nil)
			agent := &Agent{httpClient: client, pulseURL: target.URL, cfg: Config{APIToken: target.Token}, agentVersion: "fixture"}
			if tc.authoritative {
				agent.targets = []*reportTarget{{config: target, client: client}}
				err = agent.sendReport(context.Background(), report)
			} else {
				err = agent.sendReportToTarget(context.Background(), report, &reportTarget{config: target, client: client})
			}
			if err != nil {
				t.Fatal(err)
			}
			got := <-observed
			if got.err != nil {
				t.Fatal(got.err)
			}
			if got.address != parsed.Host || got.connect {
				want, _ := json.Marshal(report)
				t.Fatalf("plaintext reached %s, want only configured endpoint %s (both bearer headers=%t, exact report=%t)",
					got.address, parsed.Host, got.authorization == "Bearer "+target.Token && got.token == target.Token, bytes.Equal(got.payload, want))
			}
			checkKubernetesProxyReport(t, got, parsed, target.Token, report)
		})
	}

	certificate, caPath := kubernetesProxyFixtureCertificate(t)
	sum := sha256.Sum256(certificate.Certificate[0])
	for _, tc := range []struct {
		name string
		ca   string
		pin  string
		fail bool
	}{
		{name: "https-custom-ca", ca: caPath},
		{name: "https-matching-pin", pin: hex.EncodeToString(sum[:])},
		{name: "https-mismatched-pin", pin: strings.Repeat("0", 64), fail: true},
		{name: "https-untrusted-certificate", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := TargetConfig{URL: "https://pulse.example/pulse", Token: "fixture-tls-bearer", CACertPath: tc.ca, ServerFingerprint: tc.pin, Authoritative: true}
			client, err := newKubernetesHTTPClient(target)
			if err != nil {
				t.Fatal(err)
			}
			transport := client.Transport.(*http.Transport)
			defer transport.CloseIdleConnections()
			observed := make(chan kubernetesProxyObservation, 1)
			transport.DialContext = kubernetesProxyFixtureDial(t, observed, &certificate)
			agent := &Agent{httpClient: client, pulseURL: target.URL, cfg: Config{APIToken: target.Token}, agentVersion: "fixture"}
			err = agent.sendReport(context.Background(), report)
			got := <-observed
			if got.address != kubernetesProxyFixtureAddress || !got.connect {
				t.Fatalf("HTTPS lost ambient CONNECT proxy: address=%s connect=%v", got.address, got.connect)
			}
			if got.connectAuthority != "pulse.example:443" || got.connectCredentials {
				t.Fatal("CONNECT changed the Pulse authority or exposed report credentials")
			}
			if tc.fail {
				if err == nil || got.err == nil || got.authorization != "" || len(got.payload) != 0 {
					t.Fatalf("unverified TLS sent a report: client error=%v, server error=%v", err, got.err)
				}
				if tc.pin != "" && !strings.Contains(err.Error(), "fingerprint mismatch") {
					t.Fatalf("wrong pin failure: %v", err)
				}
				return
			}
			if err != nil || got.err != nil {
				t.Fatalf("verified TLS: client=%v server=%v", err, got.err)
			}
			if got.serverName != "pulse.example" {
				t.Fatalf("TLS server name = %q", got.serverName)
			}
			parsed, _ := url.Parse(target.URL)
			checkKubernetesProxyReport(t, got, parsed, target.Token, report)
		})
	}
}

type kubernetesProxyObservation struct {
	address, connectAuthority, serverName   string
	connect, connectCredentials             bool
	method, uri, host, authorization, token string
	payload                                 []byte
	err                                     error
}

func kubernetesProxyFixtureDial(t *testing.T, observed chan<- kubernetesProxyObservation, certificate *tls.Certificate) func(context.Context, string, string) (net.Conn, error) {
	t.Helper()
	return func(_ context.Context, _, address string) (net.Conn, error) {
		// No listener, DNS lookup, socket or real proxy is used. The production
		// Transport selects its dial address and sends actual HTTP/TLS via Pipe.
		client, server := net.Pipe()
		deadline := time.Now().Add(3 * time.Second)
		if err := client.SetDeadline(deadline); err != nil {
			t.Error(err)
		}
		if err := server.SetDeadline(deadline); err != nil {
			t.Error(err)
		}
		go func() {
			got := kubernetesProxyObservation{address: address}
			defer func() { _ = server.Close(); observed <- got }()
			reader := bufio.NewReader(server)
			if certificate != nil {
				connect, err := http.ReadRequest(reader)
				if err != nil {
					got.err = err
					return
				}
				got.connect = connect.Method == http.MethodConnect
				got.connectAuthority = connect.Host
				got.connectCredentials = connect.Header.Get("Authorization") != "" || connect.Header.Get("X-API-Token") != ""
				if !got.connect {
					got.err = fmt.Errorf("expected CONNECT")
					return
				}
				if _, err := io.WriteString(server, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
					got.err = err
					return
				}
				secure := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{*certificate}, MinVersion: tls.VersionTLS12})
				if err := secure.Handshake(); err != nil {
					got.err = err
					return
				}
				got.serverName = secure.ConnectionState().ServerName
				reader = bufio.NewReader(secure)
				// Keep the original Pipe for deferred cleanup: tls.Close can wait
				// for close_notify and obscure a completed HTTP fixture result.
				defer secure.Close()
				got.err = kubernetesProxyReadReport(reader, secure, &got)
			} else {
				got.err = kubernetesProxyReadReport(reader, server, &got)
			}
		}()
		return client, nil
	}
}

func kubernetesProxyReadReport(reader *bufio.Reader, writer io.Writer, got *kubernetesProxyObservation) error {
	req, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	got.method, got.uri, got.host = req.Method, req.RequestURI, req.Host
	got.authorization, got.token = req.Header.Get("Authorization"), req.Header.Get("X-API-Token")
	if req.Header.Get("Content-Encoding") != "gzip" || req.Header.Get("Content-Type") != "application/json" {
		return fmt.Errorf("report encoding changed")
	}
	compressed, err := gzip.NewReader(req.Body)
	if err != nil {
		return err
	}
	got.payload, err = io.ReadAll(compressed)
	if err != nil {
		return err
	}
	if err := compressed.Close(); err != nil {
		return err
	}
	_, err = io.WriteString(writer, "HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n")
	return err
}

func checkKubernetesProxyReport(t *testing.T, got kubernetesProxyObservation, target *url.URL, token string, report agentsk8s.Report) {
	t.Helper()
	if got.method != http.MethodPost || got.uri != target.Path+"/api/agents/kubernetes/report" || got.host != target.Host {
		t.Fatalf("report destination changed: %s %s host=%s", got.method, got.uri, got.host)
	}
	if got.authorization != "Bearer "+token || got.token != token {
		t.Fatal("report authentication changed")
	}
	want, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.payload, want) {
		t.Fatal("compressed report contents changed")
	}
}

func kubernetesProxyFixtureCertificate(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Pulse proxy fixture"},
		DNSNames: []string{"pulse.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture-ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, path
}
