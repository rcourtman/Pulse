package vmware

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

type releaseProbeTransport func(*http.Request) (*http.Response, error)

func (f releaseProbeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// #2070 demonstrates method routing at 8.0.3.0. Probe that canonical
// release before the legacy spelling or an older API schema.
func TestVIJSONCanonical803Release(t *testing.T) {
	for _, tc := range []struct {
		name            string
		canonicalStatus int
		fallback        string
		wantRelease     string
		wantCategory    string
		wantProbes      []string
	}{
		{"canonical", 200, "8.0.2.0", "8.0.3.0", "", []string{"9.0.0.0", "8.0.3.0"}},
		{"legacy", 404, "8.0.3", "8.0.3", "", []string{"9.0.0.0", "8.0.3.0", "8.0.3"}},
		{"older", 500, "8.0.2.0", "8.0.2.0", "", []string{"9.0.0.0", "8.0.3.0", "8.0.3", "8.0.2.0"}},
		{"auth", 401, "8.0.2.0", "", "auth", []string{"9.0.0.0", "8.0.3.0"}},
		{"permission", 403, "8.0.2.0", "", "permission", []string{"9.0.0.0", "8.0.3.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var probes []string
			transport := releaseProbeTransport(func(r *http.Request) (*http.Response, error) {
				release := strings.Split(r.URL.Path, "/")[3]
				probes = append(probes, release)
				status := http.StatusNotFound
				if release == "9.0.0.0" {
					status = http.StatusInternalServerError
				}
				if release == tc.fallback {
					status = http.StatusOK
				}
				if release == "8.0.3.0" {
					status = tc.canonicalStatus
				}
				body := ""
				if status == http.StatusOK {
					body = `{"sessionManager":{"type":"SessionManager","value":"SessionManager"},"perfManager":{"type":"PerformanceManager","value":"PerfMgr"},"eventManager":{"type":"EventManager","value":"EventManager"}}`
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			base, err := url.Parse("https://vcenter.invalid")
			if err != nil {
				t.Fatal(err)
			}
			client := &Client{baseURL: base, httpClient: &http.Client{Transport: transport}}
			release, refs, err := client.resolveVIJSONRelease(context.Background())
			if tc.wantCategory != "" {
				ce, ok := err.(*ConnectionError)
				if !ok || ce.Category != tc.wantCategory {
					t.Fatalf("error = %v, want %s", err, tc.wantCategory)
				}
			} else if err != nil || refs.PerfManagerMoID != "PerfMgr" {
				t.Fatalf("refs=%+v err=%v", refs, err)
			}
			if release != tc.wantRelease {
				t.Errorf("release=%q want %q", release, tc.wantRelease)
			}
			if !reflect.DeepEqual(probes, tc.wantProbes) {
				t.Errorf("probes=%v want %v", probes, tc.wantProbes)
			}
		})
	}
}
