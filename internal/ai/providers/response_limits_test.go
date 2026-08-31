package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProviderListModelsRejectsDeclaredOversizedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxProviderResponseBodyBytes+1))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	ollama, err := NewOllamaClient("test", server.URL, "", "", time.Second)
	if err != nil {
		t.Fatalf("NewOllamaClient() error = %v", err)
	}

	tests := []struct {
		name string
		list func(context.Context) ([]ModelInfo, error)
	}{
		{name: "OpenAI-compatible", list: NewOpenAIClient("key", "test", server.URL, time.Second).ListModels},
		{name: "Anthropic", list: NewAnthropicClientWithBaseURL("key", "test", server.URL, time.Second).ListModels},
		{name: "Gemini", list: NewGeminiClient("key", "test", server.URL, time.Second).ListModels},
		{name: "Ollama", list: ollama.ListModels},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.list(context.Background())
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("response body exceeds %d bytes", maxProviderResponseBodyBytes)) {
				t.Fatalf("ListModels() error = %v, want response size limit error", err)
			}
		})
	}
}

func TestReadProviderResponseBodyRejectsUndeclaredOversizedResponse(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		Body:          io.NopCloser(io.LimitReader(zeroReader{}, maxProviderResponseBodyBytes+1)),
		ContentLength: -1,
	}
	body, err := readProviderResponseBody(resp)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("response body exceeds %d bytes", maxProviderResponseBodyBytes)) {
		t.Fatalf("readProviderResponseBody() error = %v, want response size limit error", err)
	}
	if int64(len(body)) > maxProviderResponseBodyBytes {
		t.Fatalf("readProviderResponseBody() returned %d bytes, limit %d", len(body), maxProviderResponseBodyBytes)
	}
}

func TestNotableModelsCacheRejectsDeclaredOversizedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxProviderResponseBodyBytes+1))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cache := NewNotableModelsCache(server.URL, time.Hour)
	err := cache.Refresh(context.Background())
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("response body exceeds %d bytes", maxProviderResponseBodyBytes)) {
		t.Fatalf("Refresh() error = %v, want response size limit error", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
