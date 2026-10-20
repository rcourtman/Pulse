package pbs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type datastoreStatusBody struct {
	io.Reader
	closed   bool
	closeErr error
}

func (b *datastoreStatusBody) Close() error { b.closed = true; return b.closeErr }

type datastoreStatusReadError struct{}

func (datastoreStatusReadError) Read([]byte) (int, error) {
	return 0, errors.New("private-status-read-error")
}

// Test through the real public reader, not just a decoder: sibling progress,
// exclusion, body closure, bounded diagnostics and dedup precedence survive.
func TestPBSDatastoreStatusAdmission(t *testing.T) {
	valid := `{"data":{"total":1000,"used":600,"avail":350}}`
	cases := []struct {
		name, body string
		valid      bool
	}{
		{"ordinary", valid, true},
		{"extensions", `{"data":{"total":1000,"used":600,"avail":350,"extension":{"private":"ignored"},"error":{"private":"ignored"},"store":42}}`, true},
		{"legacy", `{"data":{"total-space":1000,"used-space":600,"avail-space":350}}`, true},
		{"inferred-total", `{"data":{"used":600,"avail":350}}`, true},
		{"mixed-names", `{"data":{"total":1000,"used-space":600,"avail":350}}`, true},
		{"confirmed-empty", `{"data":{"total":1000,"used":0,"avail":1000}}`, true},
		{"explicit-zero", `{"data":{"total":0,"used":0,"avail":0}}`, true},
		{"integer-precision", `{"data":{"total":9007199254740993,"used":1,"avail":9007199254740992}}`, true},
		{"status-dedup", `{"data":{"total":1000,"used":600,"avail":350,"status":" read_only ","gc-status":" idle ","deduplication_factor":3,"deduplication-factor":4}}`, true},
		{"missing-data", `{"message":"private-status-provider-body"}`, false},
		{"null-data", `{"data":null}`, false},
		{"array-data", `{"data":[]}`, false},
		{"empty-data", `{"data":{}}`, false},
		{"missing-used", `{"data":{"total":1000,"avail":1000}}`, false},
		{"missing-available", `{"data":{"total":1000,"used":0}}`, false},
		{"error-envelope", `{"data":{"total":1000,"used":0,"avail":1000},"errors":{"private":"private-status-provider-body"}}`, false},
		{"duplicate-data", `{"data":{},"data":{"total":1000,"used":0,"avail":1000}}`, false},
		{"case-data", `{"Data":{"total":1000,"used":0,"avail":1000}}`, false},
		{"duplicate-used", `{"data":{"total":1000,"used":600,"used":0,"avail":1000}}`, false},
		{"case-used", `{"data":{"total":1000,"used":600,"Used":0,"avail":1000}}`, false},
		{"null-used", `{"data":{"total":1000,"used":null,"avail":1000}}`, false},
		{"string-used", `{"data":{"total":1000,"used":"private-status-provider-body","avail":1000}}`, false},
		{"negative", `{"data":{"total":1000,"used":-1,"avail":1000}}`, false},
		{"fractional", `{"data":{"total":1000,"used":0.5,"avail":1000}}`, false},
		{"overflow", `{"data":{"total":9223372036854775808,"used":1,"avail":1}}`, false},
		{"inferred-overflow", `{"data":{"used":9223372036854775807,"avail":1}}`, false},
		{"used-exceeds-total", `{"data":{"total":1000,"used":1001,"avail":0}}`, false},
		{"available-exceeds-total", `{"data":{"total":1000,"used":1,"avail":1001}}`, false},
		{"wrong-status", `{"data":{"total":1000,"used":1,"avail":999,"status":42}}`, false},
		{"null-status", `{"data":{"total":1000,"used":1,"avail":999,"status":null}}`, false},
		{"bad-dedup", `{"data":{"total":1000,"used":1,"avail":999,"deduplication-factor":"private-status-provider-body"}}`, false},
		{"negative-dedup", `{"data":{"total":1000,"used":1,"avail":999,"deduplication-factor":-1}}`, false},
		{"trailing-json", valid + `{}`, false},
		{"invalid-utf8", `{"data":{"total":1000,"used":0,"avail":1000,"extension":"` + string([]byte{0xff}) + `"}}`, false},
		{"oversized", `{"data":{"total":1000,"used":0,"avail":1000,"extension":"` + strings.Repeat("x", int(maxResponseBodyBytes)) + `"}}`, false},
		{"non-200", valid, false},
		{"read-error", valid, false},
		{"close-error", valid, false},
	}
	prior := log.Logger
	defer func() { log.Logger = prior }()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			log.Logger = zerolog.New(&logs).Level(zerolog.DebugLevel)
			client, err := NewClient(ClientConfig{Host: "https://pbs.invalid", TokenName: "fixture@pbs!read", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			var bodies []*datastoreStatusBody
			var paths []string
			client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PBSAPIToken=fixture@pbs!read:fixture-only" {
					t.Error("changed read-only token authentication")
				}
				paths = append(paths, r.URL.RequestURI())
				body, status := `{"data":[{"time":1,"dedup_factor":2}]}`, http.StatusOK
				switch r.URL.Path {
				case "/api2/json/admin/datastore":
					body = `{"data":[{"store":"target"},{"store":"sibling"},{"store":"excluded"}]}`
				case "/api2/json/admin/datastore/target/status":
					body = tc.body
					if tc.name == "non-200" {
						status = http.StatusPartialContent
					}
				case "/api2/json/admin/datastore/sibling/status":
					// Target must be closed before sibling I/O begins.
					if len(bodies) >= 3 && !bodies[2].closed {
						t.Error("status body retained across sibling requests")
					}
					body = valid
				}
				reader := &datastoreStatusBody{Reader: strings.NewReader(body)}
				if r.URL.Path == "/api2/json/admin/datastore/target/status" {
					if tc.name == "read-error" {
						reader.Reader = datastoreStatusReadError{}
					}
					if tc.name == "close-error" {
						reader.closeErr = errors.New("private-status-close-error")
					}
				}
				bodies = append(bodies, reader)
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: reader, Request: r}, nil
			})
			stores, err := client.GetDatastoresFiltered(context.Background(), func(name string) bool { return name != "excluded" })
			if err != nil || len(stores) != 2 {
				t.Fatalf("status failure changed discovery: count=%d err=%v", len(stores), err)
			}
			target := stores[0]
			if tc.valid {
				if target.Error != "" || target.Status == "unavailable" {
					t.Errorf("valid capacity became unavailable: %s", tc.name)
				}
				if tc.name == "integer-precision" && target.Total != 9007199254740993 {
					t.Error("integer capacity lost precision through float64")
				}
				if tc.name == "status-dedup" && (target.Status != "read_only" || target.GCStatus != "idle" || target.DeduplicationFactor != 4) {
					t.Error("status/dedup precedence changed")
				}
			} else if target.Status != "unavailable" || target.Error == "" || target.Total != 0 || target.Used != 0 || target.Avail != 0 || target.DeduplicationFactor != 0 {
				t.Errorf("invalid capacity became a measurement: %s", tc.name)
			}
			if stores[1].Status != "available" || stores[1].Total != 1000 || stores[1].Used != 600 || stores[1].DeduplicationFactor != 2 {
				t.Error("target failure blocked readable sibling")
			}
			wantPaths := []string{"/api2/json/admin/datastore", "/api2/json/admin/datastore/target/rrd?cf=AVERAGE&timeframe=hour", "/api2/json/admin/datastore/target/status", "/api2/json/admin/datastore/sibling/rrd?cf=AVERAGE&timeframe=hour", "/api2/json/admin/datastore/sibling/status"}
			if !reflect.DeepEqual(paths, wantPaths) {
				t.Error("changed request selection/order or probed excluded scope")
			}
			for _, b := range bodies {
				if !b.closed {
					t.Error("response body not closed")
				}
			}
			if strings.Contains(logs.String()+fmt.Sprint(stores), "private-status-") || logs.Len() > 4096 {
				t.Error("status failure leaked/scaled with provider details")
			}
		})
	}
}
