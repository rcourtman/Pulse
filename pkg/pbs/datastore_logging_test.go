package pbs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Non-parallel: these tests capture and restore the package logger. Only local
// synthetic PBS responses are used; no installed target or backup is touched.
func TestGetDatastoresRoutineLogging(t *testing.T) {
	for _, level := range []zerolog.Level{zerolog.InfoLevel, zerolog.DebugLevel} {
		t.Run(level.String(), func(t *testing.T) {
			const stores, polls = 12, 5
			var captured bytes.Buffer
			priorLogger, priorLevel := log.Logger, zerolog.GlobalLevel()
			log.Logger = zerolog.New(&captured).Level(level)
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			defer func() {
				log.Logger = priorLogger
				zerolog.SetGlobalLevel(priorLevel)
			}()

			var mu sync.Mutex
			extra := ""
			requests := make(map[string]int)
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				requests[r.URL.RequestURI()]++
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api2/json/admin/datastore" {
					var rows []map[string]string
					for i := 0; i < stores; i++ {
						rows = append(rows, map[string]string{"store": fmt.Sprintf("store-%02d", i)})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
					return
				}
				for i := 0; i < stores; i++ {
					path := fmt.Sprintf("/api2/json/admin/datastore/store-%02d/", i)
					switch r.URL.Path {
					case path + "rrd":
						factor := 0
						if i%3 == 1 {
							factor = 2
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"time": 123, "dedup_factor": factor}}})
						return
					case path + "status":
						fields := map[string]any{"total": 100, "used": 40, "avail": 60, "provider-extension": extra}
						if i%3 == 0 {
							fields["deduplication-factor"] = 4
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"data": fields})
						return
					case path + "gc":
						_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"index-data-bytes": 90, "disk-bytes": 30}})
						return
					}
				}
				t.Error("unexpected datastore request")
				http.NotFound(w, r)
			})
			defer server.Close()
			defer client.httpClient.CloseIdleConnections()

			var smallLogs string
			var smallRequests map[string]int
			for _, large := range []bool{false, true} {
				mu.Lock()
				requests = make(map[string]int)
				if large {
					extra = strings.Repeat("synthetic-provider-only-metadata", 1024)
				}
				mu.Unlock()
				captured.Reset() // Constructor diagnostics are not recurring polls.
				for poll := 0; poll < polls; poll++ {
					rows, err := client.GetDatastores(context.Background())
					if err != nil || len(rows) != stores {
						t.Fatalf("datastore polling failed: rows=%d err=%v", len(rows), err)
					}
					for i, row := range rows {
						factor := []float64{4, 2, 3}[i%3]
						want := Datastore{Store: fmt.Sprintf("store-%02d", i), Total: 100, Used: 40, Avail: 60, Status: "available", DeduplicationFactor: factor}
						if row != want {
							t.Fatalf("log filtering changed datastore %d readings", i)
						}
					}
				}
				t.Logf("level=%s large_provider_extension=%t polls=%d datastores=%d captured_log_bytes=%d", level, large, polls, stores, captured.Len())
				if strings.Contains(captured.String(), "synthetic-provider-only-metadata") || strings.Contains(captured.String(), "all_fields") {
					t.Error("routine datastore logs copied arbitrary provider fields")
				}
				if level == zerolog.InfoLevel && captured.Len() != 0 {
					t.Error("healthy datastore polling must not write recurring Info logs")
				}
				if level == zerolog.DebugLevel && strings.Count(captured.String(), `"message":"PBS datastore status"`) != stores*polls {
					t.Error("Debug must retain one named-measurement summary per datastore poll")
				}
				mu.Lock()
				if !large {
					smallLogs = captured.String()
					smallRequests = requests
				} else if smallLogs != captured.String() || !reflect.DeepEqual(smallRequests, requests) {
					t.Error("unused provider payload changed log volume or requests")
				}
				mu.Unlock()
			}
		})
	}
}

func TestGetDatastoresFailureLogging(t *testing.T) {
	for _, failure := range []string{"list-json", "status-json", "status-http"} {
		t.Run(failure, func(t *testing.T) {
			var captured bytes.Buffer
			priorLogger, priorLevel := log.Logger, zerolog.GlobalLevel()
			log.Logger = zerolog.New(&captured).Level(zerolog.InfoLevel)
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			defer func() {
				log.Logger = priorLogger
				zerolog.SetGlobalLevel(priorLevel)
			}()
			const malformed = `{"provider-extension":"synthetic-provider-only-metadata","data":`
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/admin/datastore":
					if failure == "list-json" {
						_, _ = w.Write([]byte(malformed))
					} else {
						_, _ = w.Write([]byte(`{"data":[{"store":"main"}]}`))
					}
				case "/api2/json/admin/datastore/main/rrd":
					_, _ = w.Write([]byte(`{"data":[]}`))
				case "/api2/json/admin/datastore/main/status":
					if failure == "status-http" {
						http.Error(w, "synthetic upstream unavailable", http.StatusServiceUnavailable)
					} else {
						_, _ = w.Write([]byte(malformed))
					}
				default:
					t.Error("failed response caused an unexpected follow-up")
					http.NotFound(w, r)
				}
			})
			defer server.Close()
			defer client.httpClient.CloseIdleConnections()
			captured.Reset()
			rows, err := client.GetDatastores(context.Background())
			if failure == "list-json" {
				if err == nil || len(rows) != 0 || !strings.HasPrefix(err.Error(), "failed to parse datastore list:") {
					t.Fatal("malformed listing lost its existing error")
				}
			} else if err != nil || len(rows) != 1 || rows[0].Status != "unavailable" || rows[0].Error == "" {
				t.Fatal("failed status lost its existing unavailable state/error")
			}
			var event map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(captured.Bytes()), &event); err != nil {
				t.Fatal("failed poll must retain exactly one structured diagnostic")
			}
			if event["level"] != "error" || event["error"] == nil || event["message"] == nil {
				t.Fatal("failed poll lost its actionable error diagnostic")
			}
			if failure != "list-json" && event["store"] != "main" {
				t.Error("failed status lost its datastore context")
			}
			if failure != "status-http" && event["responseBytes"] != float64(len(malformed)) {
				t.Error("malformed response diagnostic must retain its byte count")
			}
			if _, raw := event["response"]; raw || strings.Contains(captured.String(), "synthetic-provider-only-metadata") {
				t.Error("parse diagnostic copied an arbitrary response payload")
			}
		})
	}
}
