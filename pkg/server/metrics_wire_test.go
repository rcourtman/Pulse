package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// Exercise the same handler used by Pulse's metrics server, with actual Pulse
// collectors. Unsupported zstd requests retain the identity fallback; Pulse
// does not opt into promhttp/zstd. Protobuf remains a transitive dependency.
func TestAlertExporterWireFormats(t *testing.T) {
	const kind = "wire_contract"
	metrics.AlertsActive.WithLabelValues("warning", kind).Set(7)
	t.Cleanup(func() { metrics.AlertsActive.DeleteLabelValues("warning", kind) })
	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(metrics.AlertsActive)
	if err := registry.Register(metrics.AlertsActive); err == nil {
		t.Fatal("duplicate collector registration accepted")
	}
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	for _, format := range []struct{ name, accept string }{
		{"text", "text/plain; version=0.0.4"},
		{"wildcard", "*/*"},
		{"browser", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		{"json-fallback", "application/json, */*"},
		{"protobuf", "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited"},
	} {
		for _, encoding := range []string{"identity", "gzip", "zstd"} {
			t.Run(format.name+"/"+encoding, func(t *testing.T) {
				request := httptest.NewRequest("GET", "/metrics", nil)
				request.Header.Set("Accept", format.accept)
				request.Header.Set("Accept-Encoding", encoding)
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				if response.Code != 200 {
					t.Fatalf("status: %d: %s", response.Code, response.Body)
				}
				var body io.Reader = response.Body
				switch encoding {
				case "gzip":
					if response.Header().Get("Content-Encoding") != encoding {
						t.Fatal("gzip not negotiated")
					}
					reader, err := gzip.NewReader(body)
					if err != nil {
						t.Fatal(err)
					}
					defer reader.Close()
					body = reader
				default:
					if response.Header().Get("Content-Encoding") != "" {
						t.Fatal("unexpected compression")
					}
				}
				negotiated := expfmt.ResponseFormat(response.Header())
				if format.name == "protobuf" && negotiated.FormatType() != expfmt.TypeProtoDelim {
					t.Fatalf("not protobuf: %s", negotiated)
				}
				if format.name != "protobuf" && negotiated.FormatType() != expfmt.TypeTextPlain {
					t.Fatalf("not text: %s", negotiated)
				}
				decoder := expfmt.NewDecoder(body, negotiated)
				family := &dto.MetricFamily{}
				if err := decoder.Decode(family); err != nil {
					t.Fatal(err)
				}
				if family.GetName() != "pulse_alerts_active" || family.GetType() != dto.MetricType_GAUGE {
					t.Fatalf("unexpected family: %v", family)
				}
				found := false
				for _, metric := range family.Metric {
					labels := map[string]string{}
					for _, label := range metric.Label {
						labels[label.GetName()] = label.GetValue()
					}
					if labels["type"] == kind && labels["level"] == "warning" {
						found = true
						if len(labels) != 2 || metric.GetGauge().GetValue() != 7 {
							t.Fatalf("changed sample: %v", metric)
						}
					}
				}
				if !found {
					t.Fatal("Pulse labelled sample missing")
				}
				if err := decoder.Decode(&dto.MetricFamily{}); err != io.EOF {
					t.Fatalf("trailing decode: %v", err)
				}
			})
		}
	}
}

// The module requires klauspost/compress even though Pulse does not enable
// promhttp's optional zstd integration. Qualify the codec without enabling it.
func TestOptionalExporterZstdCodec(t *testing.T) {
	payload := []byte("pulse_alerts_active{level=\"warning\",type=\"wire_contract\"} 7\n")
	writer, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	encoded := writer.EncodeAll(payload, nil)
	reader, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	decoded, err := reader.DecodeAll(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatal("zstd round trip changed payload")
	}
	if _, err := reader.DecodeAll([]byte("not a zstd frame"), nil); err == nil {
		t.Fatal("invalid frame accepted")
	}
}
