package pbs

import (
	"context"
	"net/http"
	"testing"
)

func TestGetDatastoresRequiresExplicitNamedScope(t *testing.T) {
	for _, body := range []string{
		`null`, `{}`, `{"data":null}`, `{"data":{}}`, `{"data":`,
		`{"data":[null]}`, `{"data":[{}]}`, `{"data":[{"store":"  "}]}`,
		`{"data":[{"store":"valid"},{}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			detailReads := 0
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api2/json/admin/datastore" {
					detailReads++
					t.Error("malformed scope caused a datastore detail read")
				}
				_, _ = w.Write([]byte(body))
			})
			defer server.Close()
			// Validate the whole scope before filtering or issuing detail reads;
			// an exclusion must not turn malformed provider data into absence.
			rows, err := client.GetDatastoresFiltered(context.Background(), func(string) bool { return false })
			if err == nil || len(rows) != 0 || detailReads != 0 {
				t.Fatalf("invalid scope accepted: rows=%v err=%v detailReads=%d", rows, err, detailReads)
			}
		})
	}
}

func TestGetDatastoresAcceptsExplicitEmptyScope(t *testing.T) {
	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/admin/datastore" {
			t.Errorf("empty scope caused a detail read: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	defer server.Close()
	rows, err := client.GetDatastores(context.Background())
	if err != nil || len(rows) != 0 {
		t.Fatalf("explicit empty scope rejected: rows=%v err=%v", rows, err)
	}
}
