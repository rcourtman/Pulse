package pbs

import (
	"context"
	"net/http"
	"testing"
)

func TestNamespaceCompatibilityUsesActualHTTPStatus(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 408, 429, 500, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "upstream API error 404: not the response status", status)
			})
			defer server.Close()
			namespaces, err := client.ListNamespaces(context.Background(), "archive", "", 0)
			if status == http.StatusNotFound {
				if err != nil || len(namespaces) != 0 {
					t.Fatalf("legacy endpoint compatibility lost: namespaces=%+v err=%v", namespaces, err)
				}
				return
			}
			if got, known := HTTPStatus(err); !known || got != status {
				t.Fatalf("body text concealed actual HTTP %d: namespaces=%+v err=%v", status, namespaces, err)
			}
		})
	}
}
