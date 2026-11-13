package tlsutil

import (
	"net/http"
	"net/url"
	"testing"
)

func TestSameHostRedirectPolicy(t *testing.T) {
	request := func(raw string) *http.Request {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		return &http.Request{URL: u}
	}
	for _, tc := range []struct {
		from, to string
		allowed  bool
	}{
		{"http://nas.example.test/api/v2.0/system/info", "https://nas.example.test/api/v2.0/system/info", true},
		{"https://nas.example.test:8443/api", "https://NAS.example.test./ui/", true},
		{"https://192.0.2.10/api", "https://192.0.2.10:9443/api", true},
		{"https://[2001:db8::1]/api", "https://[2001:db8:0::1]/api", true},
		{"https://nas.example.test/api", "http://127.0.0.1:7655/api/health", false},
		{"https://nas.example.test/api", "http://169.254.169.254/latest/meta-data/", false},
		{"https://nas.example.test/api", "https://other.example.test/api", false},
		{"https://nas.example.test/api", "https://user@nas.example.test/api", false},
		{"https://nas.example.test/api", "ftp://nas.example.test/api", false},
	} {
		err := SameHostRedirectPolicy(request(tc.to), []*http.Request{request(tc.from)})
		if tc.allowed && err != nil {
			t.Errorf("%s -> %s refused: %v", tc.from, tc.to, err)
		}
		if !tc.allowed && err == nil {
			t.Errorf("%s -> %s allowed", tc.from, tc.to)
		}
	}

	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = request("https://nas.example.test/api")
	}
	if err := SameHostRedirectPolicy(request("https://nas.example.test/api"), via); err == nil {
		t.Error("an eleventh redirect was allowed")
	}
	if err := SameHostRedirectPolicy(request("https://nas.example.test/api"), nil); err == nil {
		t.Error("a redirect without an original request was allowed")
	}
}
