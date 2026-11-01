package api

import (
	"net/http/httptest"
	"testing"
)

func TestSupportBundleMediaDoesNotWidenAssistantResults(t *testing.T) {
	for _, policy := range []struct {
		name, media string
		zip         bool
	}{{"json", "application/json", false}, {"zip", "application/zip", true}} {
		for _, provided := range []string{"", "application/json", "application/zip", "text/html", "image/svg+xml", "application/json; invalid"} {
			t.Run(policy.name+"/"+provided, func(t *testing.T) {
				base := httptest.NewRecorder()
				writer := &tokenResponseWriter{writer: base, header: base.Header().Clone(), allowed: func() bool { return true }, zipDownload: policy.zip}
				writer.Header()["content-type"] = []string{"text/html"}
				writer.Header().Set("Content-Type", provided)
				if _, err := writer.Write([]byte("synthetic bytes")); err != nil {
					t.Fatal(err)
				}
				want := "text/plain; charset=utf-8"
				if provided == policy.media {
					want = policy.media
				}
				if base.Header().Get("Content-Type") != want || base.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatalf("media = %v", base.Header())
				}
				if _, exists := base.Header()["content-type"]; exists {
					t.Fatal("alternate-case media bypassed boundary")
				}
				if base.Body.String() != "synthetic bytes" {
					t.Fatal("media selection changed body bytes")
				}
			})
		}
	}
}
