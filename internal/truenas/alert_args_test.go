package truenas

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Exercise the real HTTP decoder and snapshot orchestration without a listener.
type alertArgsTransport map[string]apiResponse

func (routes alertArgsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, ok := routes[r.URL.Path]
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	if !ok {
		status = http.StatusNotFound
		response.body = "{}"
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(response.body)), Request: r}, nil
}

func TestRESTAlertArgsSnapshot(t *testing.T) {
	for _, args := range []string{`"53 errors on /dev/sda"`, `null`, `[]`, `53`, `true`, `{}`, `{"name":"/dev/sda","serial":"SER-A","ue":53}`} {
		t.Run(args, func(t *testing.T) {
			routes := alertArgsTransport(defaultAPIResponses())
			routes["/api/v2.0/system/info"] = apiResponse{body: `{"hostname":"truenas-main","version":"TrueNAS-13.0-U6.1"}`}
			routes["/api/v2.0/alert/list"] = apiResponse{body: fmt.Sprintf(`[{"id":"smart-1","level":"WARNING","formatted":"53 errors on /dev/sda","source":"SMART","klass":"SMARTUncorrectedErrorsAlert","args":%s,"dismissed":true,"datetime":{"$date":1707400000000}},{"id":"spare","level":"CRITICAL","formatted":"Reserve low","klass":"SMARTSpareBlockCountAlert","args":{"sb":8},"datetime":{"$date":1707400000000}}]`, args)}
			client, err := NewClient(ClientConfig{Host: "http://truenas.invalid", APIKey: "synthetic"})
			if err != nil {
				t.Fatal(err)
			}
			client.httpClient.Transport = routes
			// Negotiation is outside this REST decoder regression; never open a socket.
			client.mode = TransportLegacyREST
			if err := client.TestConnection(context.Background()); err != nil {
				t.Fatal(err)
			}
			for cycle := 0; cycle < 2; cycle++ {
				snapshot, err := client.FetchSnapshot(context.Background())
				if err != nil {
					t.Fatalf("cycle %d: %v", cycle, err)
				}
				if snapshot.System.Hostname != "truenas-main" || snapshot.CollectedAt.IsZero() || len(snapshot.Pools) != 1 || len(snapshot.Datasets) != 1 || len(snapshot.Disks) != 2 || len(snapshot.Alerts) != 2 {
					t.Fatalf("incomplete snapshot: %+v", snapshot)
				}
				a := snapshot.Alerts[0]
				if a.ID != "smart-1" || a.Level != "WARNING" || a.Message != "53 errors on /dev/sda" || a.Source != "SMART" || a.Class != "SMARTUncorrectedErrorsAlert" || !a.Dismissed || !a.Datetime.Equal(time.UnixMilli(1707400000000).UTC()) {
					t.Fatalf("lost alert: %+v", a)
				}
				object := strings.Contains(args, "serial")
				if a.SMARTUncorrectedReported != object || a.SMARTAvailableSpareReported {
					t.Fatalf("invented/lost evidence: %+v", a)
				}
				if object {
					if a.DiskName != "/dev/sda" || a.DiskSerial != "SER-A" || a.SMARTUncorrectedErrors != 53 {
						t.Fatalf("lost SMART metadata: %+v", a)
					}
				} else if a.DiskName != "" || a.DiskSerial != "" || a.SMARTUncorrectedErrors != 0 {
					t.Fatalf("inferred metadata: %+v", a)
				}
				if b := snapshot.Alerts[1]; !b.SMARTAvailableSpareReported || b.SMARTAvailableSpare != 8 {
					t.Fatalf("lost neighbouring alert: %+v", b)
				}
			}
			routes["/api/v2.0/alert/list"] = apiResponse{body: `[{"args":`}
			if _, err := client.FetchSnapshot(context.Background()); err == nil {
				t.Fatal("malformed JSON must still fail")
			}
			routes["/api/v2.0/alert/list"] = apiResponse{status: http.StatusUnauthorized, body: "{}"}
			if _, err := client.FetchSnapshot(context.Background()); err == nil {
				t.Fatal("HTTP failure must still fail")
			}
		})
	}
}
