package cloudcp

import "testing"

func TestBaseDomainFromURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "https-domain", in: "https://cloud.pulserelay.pro", want: "cloud.pulserelay.pro"},
		{name: "http-with-port-and-path", in: "http://cloud.pulserelay.pro:8443/api/v1", want: "cloud.pulserelay.pro"},
		{name: "no-scheme-with-path", in: "cloud.pulserelay.pro/path", want: "cloud.pulserelay.pro"},
		{name: "bare-domain", in: "cloud.pulserelay.pro", want: "cloud.pulserelay.pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := baseDomainFromURL(tt.in); got != tt.want {
				t.Fatalf("baseDomainFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
