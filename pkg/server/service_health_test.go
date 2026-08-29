package server

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

func TestServiceHealthProbeCoversAPIUIAndFrontendAssets(t *testing.T) {
	tests := []struct {
		name         string
		handler      http.Handler
		wantHealthy  bool
		wantCategory string
	}{
		{
			name: "healthy",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/health":
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"status":"healthy"}`)
				case "/":
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `<html><head><link rel="stylesheet" href="/assets/app.css"></head><body><script src="/assets/app.js"></script></body></html>`)
				case "/assets/app.css":
					fmt.Fprint(w, "body{}")
				case "/assets/app.js":
					fmt.Fprint(w, "console.log('ok')")
				default:
					http.NotFound(w, r)
				}
			}),
			wantHealthy: true,
		},
		{
			name: "api unhealthy",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/health" {
					http.Error(w, `{"status":"unhealthy"}`, http.StatusServiceUnavailable)
					return
				}
				http.NotFound(w, r)
			}),
			wantCategory: telemetry.ServiceHealthFailureAPIStatus,
		},
		{
			name: "ui unavailable",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/health" {
					fmt.Fprint(w, `{"status":"healthy"}`)
					return
				}
				http.NotFound(w, r)
			}),
			wantCategory: telemetry.ServiceHealthFailureUIStatus,
		},
		{
			name: "frontend asset unavailable",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/health":
					fmt.Fprint(w, `{"status":"healthy"}`)
				case "/":
					fmt.Fprint(w, `<html><body><script src="/assets/missing.js"></script></body></html>`)
				default:
					http.NotFound(w, r)
				}
			}),
			wantCategory: telemetry.ServiceHealthFailureFrontendAssets,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			server := &http.Server{Handler: test.handler, ReadHeaderTimeout: time.Second}
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = server.Serve(listener)
			}()
			t.Cleanup(func() {
				_ = server.Close()
				<-done
			})

			got := newServiceHealthProbe(listener, false)()
			if !got.Observed || got.Healthy != test.wantHealthy || got.FailureCategory != test.wantCategory {
				t.Fatalf("service-health observation = %#v, want healthy=%v category=%q", got, test.wantHealthy, test.wantCategory)
			}
		})
	}
}

func TestServiceHealthProbeReportsConnectivityWithoutListener(t *testing.T) {
	got := newServiceHealthProbe(nil, false)()
	if !got.Observed || got.Healthy || got.FailureCategory != telemetry.ServiceHealthFailureAPIConnectivity {
		t.Fatalf("service-health observation = %#v", got)
	}
}

func TestFrontendAssetPathsStayLocalAndBounded(t *testing.T) {
	index := []byte(`<html><head><link href="https://cdn.example/app.css"><link href="/assets/app.css?v=1"></head><body><script src="/assets/app.js"></script><script src="/not-assets/private.js"></script></body></html>`)
	got := frontendAssetPaths(index)
	if len(got) != 2 || got[0] != "/assets/app.css?v=1" || got[1] != "/assets/app.js" {
		t.Fatalf("frontend asset paths = %#v", got)
	}
}
