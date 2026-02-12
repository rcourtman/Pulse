package truenas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type apiResponse struct {
	status      int
	body        string
	contentType string
}

type closeTrackingTransport struct {
	closeCalls int
}

func (t *closeTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

func (t *closeTrackingTransport) CloseIdleConnections() {
	t.closeCalls++
}

func TestClientGetters(t *testing.T) {
	server := newMockServer(t, defaultAPIResponses(), nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	ctx := context.Background()

	system, err := client.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("GetSystemInfo() error = %v", err)
	}
	if system.Hostname != "truenas-main" || system.Version != "TrueNAS-SCALE-24.10.2" {
		t.Fatalf("unexpected system info: %+v", system)
	}
	if system.Build != "24.10.2.1" || system.UptimeSeconds != 86400 {
		t.Fatalf("unexpected system fields: %+v", system)
	}
	if !system.Healthy || system.MachineID != "SER123" {
		t.Fatalf("unexpected system health/identity: %+v", system)
	}

	pools, err := client.GetPools(ctx)
	if err != nil {
		t.Fatalf("GetPools() error = %v", err)
	}
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].ID != "1" || pools[0].Name != "tank" || pools[0].UsedBytes != 400 {
		t.Fatalf("unexpected pool mapping: %+v", pools[0])
	}

	datasets, err := client.GetDatasets(ctx)
	if err != nil {
		t.Fatalf("GetDatasets() error = %v", err)
	}
	if len(datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(datasets))
	}
	if datasets[0].ID != "tank/apps" || datasets[0].UsedBytes != 12345 || datasets[0].AvailBytes != 555 {
		t.Fatalf("unexpected dataset usage mapping: %+v", datasets[0])
	}
	if !datasets[0].Mounted || datasets[0].ReadOnly {
		t.Fatalf("unexpected dataset mount/readonly mapping: %+v", datasets[0])
	}

	disks, err := client.GetDisks(ctx)
	if err != nil {
		t.Fatalf("GetDisks() error = %v", err)
	}
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if disks[0].Transport != "sata" || !disks[0].Rotational {
		t.Fatalf("unexpected rotational disk mapping: %+v", disks[0])
	}
	if disks[1].Transport != "nvme" || disks[1].Rotational {
		t.Fatalf("unexpected nvme disk mapping: %+v", disks[1])
	}

	alerts, err := client.GetAlerts(ctx)
	if err != nil {
		t.Fatalf("GetAlerts() error = %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].ID != "a1" || alerts[0].Level != "WARNING" {
		t.Fatalf("unexpected alert identity mapping: %+v", alerts[0])
	}
	if alerts[0].Datetime != time.UnixMilli(1707400000000).UTC() {
		t.Fatalf("unexpected alert datetime: %s", alerts[0].Datetime)
	}
}

func TestClientAuthHeaderAPIKey(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
	}, func(t *testing.T, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected bearer auth header, got %q", got)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "test-key"})
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
}

func TestClientAuthHeaderBasic(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
	}, func(t *testing.T, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok {
			t.Fatalf("expected basic auth")
		}
		if username != "admin" || password != "secret" {
			t.Fatalf("unexpected basic auth credentials: %q:%q", username, password)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{Username: "admin", Password: "secret"})
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
}

func TestTestConnectionSuccessAndFailure(t *testing.T) {
	successServer := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
	}, nil)
	t.Cleanup(successServer.Close)

	successClient := mustClientForServer(t, successServer.URL, ClientConfig{APIKey: "key"})
	if err := successClient.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() success error = %v", err)
	}

	failureServer := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {status: http.StatusUnauthorized, body: `{"error":"unauthorized"}`},
	}, nil)
	t.Cleanup(failureServer.Close)

	failureClient := mustClientForServer(t, failureServer.URL, ClientConfig{APIKey: "bad"})
	if err := failureClient.TestConnection(context.Background()); err == nil {
		t.Fatal("expected TestConnection() failure error")
	}
}

func TestFetchSnapshot(t *testing.T) {
	server := newMockServer(t, defaultAPIResponses(), nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	snapshot, err := client.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.CollectedAt.IsZero() {
		t.Fatal("expected non-zero CollectedAt")
	}
	if snapshot.System.Hostname != "truenas-main" {
		t.Fatalf("unexpected snapshot system: %+v", snapshot.System)
	}
	if len(snapshot.Pools) != 1 || len(snapshot.Datasets) != 1 || len(snapshot.Disks) != 2 || len(snapshot.Alerts) != 1 {
		t.Fatalf("unexpected snapshot counts: pools=%d datasets=%d disks=%d alerts=%d",
			len(snapshot.Pools), len(snapshot.Datasets), len(snapshot.Disks), len(snapshot.Alerts))
	}
}

func TestClientHandlesHTTPAndDecodeErrors(t *testing.T) {
	t.Run("non-2xx response", func(t *testing.T) {
		server := newMockServer(t, map[string]apiResponse{
			"/api/v2.0/pool": {status: http.StatusServiceUnavailable, body: `{"error":"down"}`},
		}, nil)
		t.Cleanup(server.Close)

		client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "key"})
		_, err := client.GetPools(context.Background())
		if err == nil {
			t.Fatal("expected error from non-2xx response")
		}
		if !strings.Contains(err.Error(), "status=503") {
			t.Fatalf("expected status code in error, got %v", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		server := newMockServer(t, map[string]apiResponse{
			"/api/v2.0/system/info": {body: `{"hostname":`},
		}, nil)
		t.Cleanup(server.Close)

		client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "key"})
		_, err := client.GetSystemInfo(context.Background())
		if err == nil {
			t.Fatal("expected malformed json error")
		}
		if !strings.Contains(err.Error(), "decode truenas response") {
			t.Fatalf("unexpected decode error: %v", err)
		}
	})

	t.Run("connection failure", func(t *testing.T) {
		server := newMockServer(t, map[string]apiResponse{
			"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
		}, nil)

		client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "key"})
		server.Close()

		_, err := client.GetSystemInfo(context.Background())
		if err == nil {
			t.Fatal("expected connection error")
		}
		if !strings.Contains(err.Error(), "failed") {
			t.Fatalf("unexpected connection error: %v", err)
		}
	})
}

func TestClientTLSFingerprintPinning(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2.0/system/info" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`))
	})

	tlsServer := httptest.NewTLSServer(handler)
	t.Cleanup(tlsServer.Close)

	cert := tlsServer.Certificate()
	fingerprintRaw := sha256.Sum256(cert.Raw)
	fingerprint := withFingerprintColons(strings.ToUpper(hex.EncodeToString(fingerprintRaw[:])))

	client := mustClientForServer(t, tlsServer.URL, ClientConfig{
		APIKey:             "key",
		InsecureSkipVerify: true,
		Fingerprint:        "SHA256:" + fingerprint,
	})
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("expected pinning success, got %v", err)
	}

	badClient := mustClientForServer(t, tlsServer.URL, ClientConfig{
		APIKey:             "key",
		InsecureSkipVerify: true,
		Fingerprint:        strings.Repeat("0", 64),
	})
	if err := badClient.TestConnection(context.Background()); err == nil {
		t.Fatal("expected pinning failure")
	}
}

func TestClientCloseClosesIdleConnections(t *testing.T) {
	transport := &closeTrackingTransport{}
	client := &Client{
		httpClient: &http.Client{
			Transport: transport,
		},
	}

	client.Close()
	if transport.closeCalls != 1 {
		t.Fatalf("expected CloseIdleConnections to be called once, got %d", transport.closeCalls)
	}
}

func TestClientCloseNilSafe(t *testing.T) {
	var nilClient *Client
	nilClient.Close()

	(&Client{}).Close()
	(&Client{httpClient: &http.Client{}}).Close()
}

func defaultAPIResponses() map[string]apiResponse {
	return map[string]apiResponse{
		"/api/v2.0/system/info": {
			body: `{"hostname":"truenas-main","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER123","system_manufacturer":"iXsystems"}`,
		},
		"/api/v2.0/pool": {
			body: `[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`,
		},
		"/api/v2.0/pool/dataset": {
			body: `[{"id":"tank/apps","name":"tank/apps","pool":"tank","used":{"rawvalue":"12345","parsed":12345},"available":{"rawvalue":"555","parsed":555},"mountpoint":"/mnt/tank/apps","readonly":{"rawvalue":"off","parsed":false},"mounted":true}]`,
		},
		"/api/v2.0/disk": {
			body: `[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"},{"identifier":"{disk-2}","name":"nvme0n1","serial":"SER-B","size":2000000,"model":"Samsung","type":"SSD","pool":"tank","bus":"NVMe","rotationrate":0,"status":"ONLINE"}]`,
		},
		"/api/v2.0/alert/list": {
			body: `[{"id":"a1","level":"WARNING","formatted":"Disk temp high","source":"DiskService","dismissed":false,"datetime":{"$date":1707400000000}}]`,
		},
	}
}

func newMockServer(t *testing.T, responses map[string]apiResponse, assertRequest func(*testing.T, *http.Request)) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if assertRequest != nil {
			assertRequest(t, request)
		}

		response, ok := responses[request.URL.Path]
		if !ok {
			http.NotFound(writer, request)
			return
		}

		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		contentType := response.contentType
		if contentType == "" {
			contentType = "application/json"
		}

		writer.Header().Set("Content-Type", contentType)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(response.body))
	}))
}

func mustClientForServer(t *testing.T, serverURL string, config ClientConfig) *Client {
	t.Helper()

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse server URL %q: %v", serverURL, err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("failed to parse server port from %q: %v", serverURL, err)
	}

	config.Host = parsed.Hostname()
	config.Port = port
	config.UseHTTPS = parsed.Scheme == "https"

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func withFingerprintColons(value string) string {
	if len(value)%2 != 0 {
		return value
	}
	parts := make([]string, 0, len(value)/2)
	for i := 0; i < len(value); i += 2 {
		parts = append(parts, value[i:i+2])
	}
	return strings.Join(parts, ":")
}
