package hostagent

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/hostmetrics"
	"github.com/rs/zerolog"
)

func TestAgent_ApplyRemoteConfig_Direct(t *testing.T) {
	mc := &mockCollector{}
	logger := zerolog.Nop()
	a, _ := New(Config{
		APIToken:  "token",
		PulseURL:  "http://pulse",
		Collector: mc,
		Logger:    &logger,
	})
	if a != nil {
		a.applyRemoteConfig(true)
		a.applyRemoteConfig(false)
	}
}

func TestAgent_Run_ImmediateCancel(t *testing.T) {
	mc := &mockCollector{metricsFn: func(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	}}
	logger := zerolog.Nop()
	a, _ := New(Config{
		APIToken:  "token",
		PulseURL:  "http://pulse",
		Interval:  1 * time.Second,
		Collector: mc,
		Logger:    &logger,
	})
	if a != nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately
		_ = a.Run(ctx)
	}
}

func TestAgent_Run_ProcessError(t *testing.T) {
	mc := &mockCollector{
		metricsFn: func(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error) {
			return hostmetrics.Snapshot{}, fmt.Errorf("collection fail")
		},
	}
	logger := zerolog.Nop()
	a, _ := New(Config{
		APIToken:  "token",
		PulseURL:  "http://pulse",
		Interval:  1 * time.Millisecond,
		Collector: mc,
		Logger:    &logger,
	})
	if a != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_ = a.Run(ctx)
	}
}

func TestAgent_RunOnce_Coverage(t *testing.T) {
	mc := &mockCollector{metricsFn: func(ctx context.Context, exclude []string) (hostmetrics.Snapshot, error) {
		return hostmetrics.Snapshot{}, nil
	}}
	logger := zerolog.Nop()
	a, _ := New(Config{
		APIToken:  "token",
		PulseURL:  "http://pulse",
		RunOnce:   true,
		Collector: mc,
		Logger:    &logger,
	})
	if a != nil {
		a.httpClient = &http.Client{Transport: &mockTransport{statusCode: 200}}
		_ = a.Run(context.Background())
	}
}

func TestIsHexString(t *testing.T) {
	if !isHexString("abc123") {
		t.Errorf("expected true for abc123")
	}
	if isHexString("xyz") {
		t.Errorf("expected false for xyz")
	}
}

func TestAgent_RunProxmoxSetup(t *testing.T) {
	mc := &mockCollector{}
	logger := zerolog.Nop()
	a, _ := New(Config{
		APIToken:  "token",
		PulseURL:  "http://pulse",
		Collector: mc,
		Logger:    &logger,
	})
	if a != nil {
		a.httpClient = &http.Client{Transport: &mockTransport{statusCode: 200}}
		a.runProxmoxSetup(context.Background())
	}
}
