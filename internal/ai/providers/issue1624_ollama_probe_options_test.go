package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Issue #1624: the Patrol readiness advisor failed capable Ollama models
// because the adapter never sent num_ctx (so large fixtures truncated at the
// server's 4096-token default), dropped an explicitly pinned temperature 0
// (falling back to Ollama's 0.8 default against a nonce-exact validator),
// and discarded done_reason so a generation-cap cutoff was undiagnosable.

func decodeCapturedOllamaRequest(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var request map[string]interface{}
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("decode captured request: %v", err)
	}
	return request
}

func newCapturingOllamaServer(t *testing.T, response string, captured *[]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		*captured = body
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(response))
	}))
}

func TestIssue1624OllamaStreamForwardsNumCtxAndPinnedZeroTemperature(t *testing.T) {
	var captured []byte
	server := newCapturingOllamaServer(t,
		`{"message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`+"\n",
		&captured)
	defer server.Close()

	client, err := NewOllamaClient("test-model", server.URL, "", "", 30*time.Second)
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	err = client.ChatStream(context.Background(), ChatRequest{
		Messages:         []Message{{Role: "user", Content: "probe"}},
		MaxTokens:        2048,
		Temperature:      0,
		TemperatureSet:   true,
		MinContextTokens: 16384,
	}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	request := decodeCapturedOllamaRequest(t, captured)
	options, ok := request["options"].(map[string]interface{})
	if !ok {
		t.Fatalf("request carried no options: %s", captured)
	}
	if got := options["num_ctx"]; got != float64(16384) {
		t.Fatalf("num_ctx = %v, want 16384", got)
	}
	if got := options["num_predict"]; got != float64(2048) {
		t.Fatalf("num_predict = %v, want 2048", got)
	}
	temperature, present := options["temperature"]
	if !present {
		t.Fatalf("pinned temperature 0 was dropped from the request: %s", captured)
	}
	if temperature != float64(0) {
		t.Fatalf("temperature = %v, want 0", temperature)
	}
}

func TestIssue1624OllamaChatForwardsNumCtxAndPinnedZeroTemperature(t *testing.T) {
	var captured []byte
	server := newCapturingOllamaServer(t,
		`{"message":{"role":"assistant","content":"ok"},"done":true,"done_reason":"stop"}`,
		&captured)
	defer server.Close()

	client, err := NewOllamaClient("test-model", server.URL, "", "", 30*time.Second)
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	if _, err := client.Chat(context.Background(), ChatRequest{
		Messages:         []Message{{Role: "user", Content: "probe"}},
		Temperature:      0,
		TemperatureSet:   true,
		MinContextTokens: 8192,
	}); err != nil {
		t.Fatalf("Chat: %v", err)
	}

	request := decodeCapturedOllamaRequest(t, captured)
	options, ok := request["options"].(map[string]interface{})
	if !ok {
		t.Fatalf("request carried no options: %s", captured)
	}
	if got := options["num_ctx"]; got != float64(8192) {
		t.Fatalf("num_ctx = %v, want 8192", got)
	}
	if got, present := options["temperature"]; !present || got != float64(0) {
		t.Fatalf("temperature = %v (present=%v), want explicit 0", got, present)
	}
}

func TestIssue1624OllamaUnpinnedZeroTemperatureStaysServerDefault(t *testing.T) {
	request := ChatRequest{MaxTokens: 100}
	options := ollamaOptionsForRequest(request)
	if options == nil {
		t.Fatal("expected options for MaxTokens")
	}
	if options.Temperature != nil {
		t.Fatalf("unpinned zero temperature must not be forwarded, got %v", *options.Temperature)
	}
	if options.NumCtx != 0 {
		t.Fatalf("num_ctx must stay unset without MinContextTokens, got %d", options.NumCtx)
	}
	if ollamaOptionsForRequest(ChatRequest{}) != nil {
		t.Fatal("a request pinning nothing must not send options")
	}
}

func TestIssue1624OllamaStreamSurfacesLengthDoneReason(t *testing.T) {
	var captured []byte
	server := newCapturingOllamaServer(t,
		`{"message":{"role":"assistant","content":"partial"},"done":false}`+"\n"+
			`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"length","prompt_eval_count":10,"eval_count":256}`+"\n",
		&captured)
	defer server.Close()

	client, err := NewOllamaClient("test-model", server.URL, "", "", 30*time.Second)
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	var done DoneEvent
	err = client.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "probe"}},
	}, func(event StreamEvent) {
		if event.Type == "done" {
			done = event.Data.(DoneEvent)
		}
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if done.StopReason != "length" {
		t.Fatalf("stop reason = %q, want the provider's done_reason %q surfaced", done.StopReason, "length")
	}
}
