package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

const responseMediaMarkup = `<script>globalThis.__pulseMediaExecuted=true</script><p>Synthetic result</p>`

type responseMediaCase struct {
	name, contentType, wantType string
}

var responseMediaCases = []responseMediaCase{
	{"missing", "", "text/plain; charset=utf-8"},
	{"html", "text/html", "text/plain; charset=utf-8"},
	{"xhtml", "application/xhtml+xml", "text/plain; charset=utf-8"},
	{"svg", "image/svg+xml", "text/plain; charset=utf-8"},
	{"javascript", "application/javascript", "text/plain; charset=utf-8"},
	{"malformed", "application/json; broken", "text/plain; charset=utf-8"},
	{"json", "application/json", "application/json"},
	{"json_parameters", "Application/JSON; charset=utf-8", "application/json"},
	{"plain", "text/plain", "text/plain; charset=utf-8"},
}

// Use a real HTTP server: ResponseRecorder does not reproduce the transport's
// automatic HTML sniff when a downstream writer omits Content-Type.
func requestResponseMedia(t *testing.T, media responseMediaCase, explicit bool) (http.Header, int, []byte) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writer := &tokenResponseWriter{writer: w, header: w.Header().Clone(), allowed: func() bool { return true }}
		defer writer.close()
		if media.contentType != "" {
			writer.Header().Set("Content-Type", media.contentType)
		}
		writer.Header().Set("X-Synthetic-Result", "preserved")
		if explicit {
			writer.WriteHeader(http.StatusAccepted)
		}
		if _, err := writer.Write([]byte(responseMediaMarkup)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.Header, response.StatusCode, body
}

func TestTokenResponseWriterNativeMedia(t *testing.T) {
	for _, media := range responseMediaCases {
		for _, explicit := range []bool{false, true} {
			name := media.name + "/implicit"
			if explicit {
				name = media.name + "/explicit"
			}
			t.Run(name, func(t *testing.T) {
				headers, status, body := requestResponseMedia(t, media, explicit)
				wantStatus := http.StatusOK
				if explicit {
					wantStatus = http.StatusAccepted
				}
				if status != wantStatus || string(body) != responseMediaMarkup || headers.Get("X-Synthetic-Result") != "preserved" {
					t.Fatalf("response changed: status=%d headers=%v body=%q", status, headers, body)
				}
				if headers.Get("Content-Type") != media.wantType || headers.Get("X-Content-Type-Options") != "nosniff" {
					t.Fatalf("executable/ambiguous media escaped the result boundary: type=%q nosniff=%q", headers.Get("Content-Type"), headers.Get("X-Content-Type-Options"))
				}
			})
		}
	}
}

func TestTokenResponseWriterJSONRoundTrip(t *testing.T) {
	value := responseMediaMarkup + " & quotes \" line\u2028paragraph\u2029"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writer := &tokenResponseWriter{writer: w, header: w.Header().Clone(), allowed: func() bool { return true }}
		defer writer.close()
		if err := utils.WriteJSONResponse(writer, map[string]string{"content": value}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if response.StatusCode != http.StatusOK || json.Unmarshal(body, &got) != nil || got["content"] != value {
		t.Fatalf("JSON changed: %d %q", response.StatusCode, body)
	}
	if response.Header.Get("Content-Type") != "application/json" || response.Header.Get("X-Content-Type-Options") != "nosniff" || strings.Contains(string(body), "<script>") {
		t.Fatalf("JSON response not safe as data: %v %q", response.Header, body)
	}
}

func TestTokenResponseWriterMediaCannotChangeAfterCommit(t *testing.T) {
	base := httptest.NewRecorder()
	writer := &tokenResponseWriter{writer: base, header: base.Header().Clone(), allowed: func() bool { return true }}
	defer writer.close()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	writer.Header().Set("Content-Type", "text/html")
	writer.Header().Del("X-Content-Type-Options")
	if _, err := writer.Write([]byte(`{"content":"<script>not executable</script>"}`)); err != nil {
		t.Fatal(err)
	}
	response := base.Result()
	defer response.Body.Close()
	if response.Header.Get("Content-Type") != "application/json" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("committed response media changed: %v", response.Header)
	}
}

func TestTokenResponseWriterNonCanonicalMediaHeaders(t *testing.T) {
	base := httptest.NewRecorder()
	writer := &tokenResponseWriter{writer: base, header: base.Header().Clone(), allowed: func() bool { return true }}
	defer writer.close()
	writer.Header().Set("Content-Type", "application/json")
	writer.Header()["content-type"] = []string{"text/html"}
	writer.Header()["CONTENT-TYPE"] = []string{"image/svg+xml"}
	writer.Header()["x-content-type-options"] = []string{"invalid"}
	if _, err := writer.Write([]byte(`{"content":"<script>data</script>"}`)); err != nil {
		t.Fatal(err)
	}
	response := base.Result()
	defer response.Body.Close()
	if len(response.Header.Values("Content-Type")) != 1 || response.Header.Get("Content-Type") != "application/json" || len(response.Header.Values("X-Content-Type-Options")) != 1 || response.Header.Get("X-Content-Type-Options") != "nosniff" || len(response.Header) != 2 {
		t.Fatalf("alternate-case media headers survived: %v", response.Header)
	}
}
