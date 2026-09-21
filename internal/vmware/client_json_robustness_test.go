package vmware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecodeVIJSONBodyTreatsEmptySuccessAsAbsent(t *testing.T) {
	var ref *viJSONReference
	if err := decodeVIJSONBody([]byte("  \n\t "), "folder parent", &ref); err != nil {
		t.Fatalf("empty success body should be tolerated, got %v", err)
	}
	if ref != nil {
		t.Fatalf("empty success body should leave target nil, got %+v", ref)
	}
}

func TestDecodeVIJSONBodyRejectsMalformedBody(t *testing.T) {
	var ref *viJSONReference
	err := decodeVIJSONBody([]byte("<html>nope</html>"), "folder parent", &ref)
	if err == nil {
		t.Fatal("expected malformed body to fail")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "endpoint" || !strings.Contains(connectionErr.Message, "not valid JSON") {
		t.Fatalf("unexpected error: %+v", connectionErr)
	}
}

func TestVIJSONFaultDetailExtractsBoundedSummary(t *testing.T) {
	body := []byte(`{"_typeName":"InvalidArgument","invalidProperty":"perfSpec","faultstring":"Unknown argument."}`)
	got := viJSONFaultDetail(body)
	for _, want := range []string{"Unknown argument.", "invalidProperty=perfSpec", "type=InvalidArgument"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fault detail %q missing %q", got, want)
		}
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Fatalf("fault detail must be single-line, got %q", got)
	}
}

func TestVIJSONFaultDetailIgnoresNonFaultBody(t *testing.T) {
	for _, body := range [][]byte{nil, []byte(""), []byte("internal error"), []byte(`{"foo":"bar"}`), []byte("not json")} {
		if got := viJSONFaultDetail(body); got != "" {
			t.Fatalf("body %q produced fault detail %q, want empty", body, got)
		}
	}
}

func TestClassifyReadStatusCodeWithBodySurfacesFault(t *testing.T) {
	body := []byte(`{"faultstring":"Invalid URI format"}`)
	err := classifyReadStatusCodeWithBody("vmware performance metrics", http.StatusInternalServerError, body)
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "endpoint" {
		t.Fatalf("category = %q, want endpoint", connectionErr.Category)
	}
	if !strings.Contains(connectionErr.Message, "HTTP 500") || !strings.Contains(connectionErr.Message, "Invalid URI format") {
		t.Fatalf("message %q should carry status and fault detail", connectionErr.Message)
	}
}

func TestCollectEntityReferenceToleratesEmptyParentBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-h5/parent", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ref, err := client.collectEntityReference(context.Background(), "9.0.0.0", "vi-session", "Folder", "group-h5", "parent", "folder parent")
	if err != nil {
		t.Fatalf("empty parent body should not degrade the connection: %v", err)
	}
	if ref != nil {
		t.Fatalf("expected nil parent reference, got %+v", ref)
	}
}

func TestCollectEntityReferenceSurfacesFaultDetail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-h5/parent", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"_typeName":"InvalidArgument","invalidProperty":"entity","faultstring":"Invalid MoRef field: entity"}`))
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.collectEntityReference(context.Background(), "9.0.0.0", "vi-session", "Folder", "group-h5", "parent", "folder parent")
	if err == nil {
		t.Fatal("expected fault response to fail")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if !strings.Contains(connectionErr.Message, "Invalid MoRef field: entity") {
		t.Fatalf("message %q should carry the fault detail", connectionErr.Message)
	}
}
