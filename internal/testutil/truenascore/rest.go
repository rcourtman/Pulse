// Package truenascore supplies the explicit CORE 13 REST schema used by
// read-only client/resource/chart integration tests. It is not an appliance.
package truenascore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// RESTFixture rejects the reported old request key independently of the
// production request type. Phase: complete, rejected, CPU-only, zero, recovered.
// Set WideHistory before the first request to return four native observations
// spanning the requested hour, rather than two live observations ten seconds apart.
type RESTFixture struct {
	Server                                       *httptest.Server
	Phase, Posts, CatalogueCalls, RejectedOldKey atomic.Int64
	WideHistory                                  bool
}

func NewRESTFixture(t *testing.T) *RESTFixture {
	t.Helper()
	f := &RESTFixture{}
	f.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			fmt.Fprint(w, `{"hostname":"core-fixture","version":"TrueNAS-13.0-U6.8","cores":8,"physmem":17179869184}`)
		case "/api/v2.0/pool":
			fmt.Fprint(w, `[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`)
		case "/api/v2.0/disk":
			fmt.Fprint(w, `[{"identifier":"disk-a","name":"da0","serial":"fixture-disk","rotationrate":"unknown","size":1000}]`)
		case "/api/v2.0/pool/dataset", "/api/v2.0/alert/list":
			fmt.Fprint(w, `[]`)
		case "/api/v2.0/reporting/graphs":
			f.CatalogueCalls.Add(1)
			fmt.Fprint(w, `[{"name":"disk","identifiers":["da0"]},{"name":"interface","identifiers":["vtnet0"]}]`)
		case "/api/v2.0/reporting/get_data":
			f.Posts.Add(1)
			var body map[string]json.RawMessage
			if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil {
				http.Error(w, "invalid fixture request", http.StatusBadRequest)
				return
			}
			if _, present := body["query"]; present {
				f.RejectedOldKey.Add(1)
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"message":"The following attributes are not expected: query"}`)
				return
			}
			var graphs []struct{ Name, Identifier string }
			var window struct {
				Start, End int64
				Aggregate  bool
			}
			if len(body) != 2 || json.Unmarshal(body["graphs"], &graphs) != nil || json.Unmarshal(body["reporting_query"], &window) != nil || !window.Aggregate || window.Start >= window.End {
				http.Error(w, "missing graphs/reporting_query range", http.StatusBadRequest)
				return
			}
			if f.Phase.Load() == 1 {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"message":"synthetic reporting validation failure"}`)
				return
			}
			var result []map[string]any
			for _, graph := range graphs {
				var legend []string
				var values []any
				zero := f.Phase.Load() == 3
				if f.Phase.Load() == 2 && graph.Name != "cpu" {
					continue // usable CPU does not establish any sibling reading
				}
				switch graph.Name {
				case "cpu":
					legend, values = []string{"user", "nice", "system", "interrupt", "idle"}, []any{8, 0, 3, 1, 88}
					if zero {
						values = []any{0, 0, 0, 0, 100}
					}
				case "memory":
					legend = []string{"memory-active_value", "memory-inactive_value", "memory-wired_value", "memory-laundry_value", "memory-free_value"}
					values = []any{1 << 30, 2 << 30, 11 << 30, 0, 2 << 30}
					if zero {
						values = []any{0, 0, 0, 0, 16 << 30}
					}
				case "arcsize":
					legend, values = []string{"arcsize_value"}, []any{8 << 30}
					if zero {
						values = []any{0}
					}
				case "disk":
					if graph.Identifier != "da0" {
						t.Error("disk identifier lost")
					}
					legend, values = []string{"disk_octets_read", "disk_octets_write"}, []any{1024, 2048}
					if zero {
						values = []any{0, 0}
					}
				case "interface":
					if graph.Identifier != "vtnet0" {
						t.Error("interface identifier lost")
					}
					legend, values = []string{"rx", "tx", "overlap"}, []any{32, 64, 32}
					if zero {
						values = []any{0, 0, 0}
					}
				case "cputemp":
					legend, values = []string{"temperature"}, []any{42}
				default:
					t.Errorf("unexpected graph %s", graph.Name)
				}
				start, step := window.End-10, int64(10)
				data := []any{values, values}
				if f.WideHistory && window.End-window.Start >= 3600 {
					start, step = window.End-3540, 1160
					data = []any{values, values, values, values}
				}
				result = append(result, map[string]any{"name": graph.Name, "identifier": graph.Identifier, "legend": legend, "start": start, "end": window.End, "step": step, "data": data})
			}
			if err := json.NewEncoder(w).Encode(result); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Server.Close)
	return f
}
