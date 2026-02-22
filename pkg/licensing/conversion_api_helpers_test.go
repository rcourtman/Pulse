package licensing

import (
	"errors"
	"testing"
	"time"
)

func TestConversionValidationReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: "unknown"},
		{name: "missing type", err: errors.New("type is required"), want: "missing_type"},
		{name: "unsupported type", err: errors.New("event type foo is not supported"), want: "unsupported_type"},
		{name: "missing surface", err: errors.New("surface is required"), want: "missing_surface"},
		{name: "missing timestamp", err: errors.New("timestamp is required"), want: "missing_timestamp"},
		{name: "missing idempotency key", err: errors.New("idempotency_key is required"), want: "missing_idempotency_key"},
		{name: "invalid tenant mode", err: errors.New("tenant_mode must be one of"), want: "invalid_tenant_mode"},
		{name: "missing capability", err: errors.New("capability is required"), want: "missing_capability"},
		{name: "missing limit key", err: errors.New("limit_key is required"), want: "missing_limit_key"},
		{name: "fallback", err: errors.New("something else"), want: "validation_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConversionValidationReason(tt.err); got != tt.want {
				t.Fatalf("reason=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOptionalTimeParam(t *testing.T) {
	defaultTime := time.Unix(1700000000, 0).UTC()

	tests := []struct {
		name    string
		raw     string
		want    time.Time
		wantErr bool
	}{
		{name: "empty defaults", raw: "", want: defaultTime, wantErr: false},
		{name: "rfc3339", raw: "2026-01-02T03:04:05Z", want: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), wantErr: false},
		{name: "date only", raw: "2026-01-02", want: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), wantErr: false},
		{name: "unix seconds", raw: "1700000000", want: time.Unix(1700000000, 0).UTC(), wantErr: false},
		{name: "unix millis", raw: "1700000000000", want: time.UnixMilli(1700000000000).UTC(), wantErr: false},
		{name: "invalid", raw: "not-a-time", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOptionalTimeParam(tt.raw, defaultTime)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("time=%s, want %s", got.Format(time.RFC3339Nano), tt.want.Format(time.RFC3339Nano))
			}
		})
	}
}
