package kubernetesagent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
	"k8s.io/client-go/kubernetes/fake"
)

func TestComputeClusterID(t *testing.T) {
	id1 := computeClusterID("https://k8s", "ctx", "name")
	id2 := computeClusterID("https://k8s", "ctx", "name")
	if id1 == "" || id1 != id2 {
		t.Fatalf("expected stable cluster ID, got %s and %s", id1, id2)
	}

	id3 := computeClusterID("https://k8s", "ctx", "other")
	if id3 == id1 {
		t.Fatalf("expected different IDs for different inputs")
	}
}

func TestFlushReportsStopsOnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := zerolog.New(io.Discard)
	reportBuffer := buffer.New[agentsk8s.Report](10)
	reportBuffer.Push(agentsk8s.Report{Timestamp: time.Now().UTC()})

	agent := &Agent{
		cfg:          Config{APIToken: "token"},
		logger:       logger,
		httpClient:   server.Client(),
		pulseURL:     server.URL,
		agentVersion: "v1",
		reportBuffer: reportBuffer,
	}

	agent.flushReports(context.Background())
	if _, ok := reportBuffer.Peek(); !ok {
		t.Fatal("expected report to remain buffered after failure")
	}
}

func TestFlushReportsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.New(io.Discard)
	reportBuffer := buffer.New[agentsk8s.Report](10)
	reportBuffer.Push(agentsk8s.Report{Timestamp: time.Now().UTC()})
	reportBuffer.Push(agentsk8s.Report{Timestamp: time.Now().UTC()})

	agent := &Agent{
		cfg:          Config{APIToken: "token"},
		logger:       logger,
		httpClient:   server.Client(),
		pulseURL:     server.URL,
		agentVersion: "v1",
		reportBuffer: reportBuffer,
	}

	agent.flushReports(context.Background())
	if _, ok := reportBuffer.Peek(); ok {
		t.Fatal("expected report buffer to be empty after flush")
	}
}

func TestRunOnceBuffersOnSendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	logger := zerolog.New(io.Discard)
	reportBuffer := buffer.New[agentsk8s.Report](10)
	agent := &Agent{
		cfg:               Config{APIToken: "token"},
		logger:            logger,
		httpClient:        server.Client(),
		pulseURL:          server.URL,
		agentID:           "agent1",
		agentVersion:      "v1",
		interval:          time.Second,
		clusterID:         "cluster",
		clusterName:       "cluster",
		clusterServer:     "https://k8s",
		clusterContext:    "ctx",
		kubeClient:        fake.NewSimpleClientset(),
		reportBuffer:      reportBuffer,
		includeNamespaces: nil,
		excludeNamespaces: nil,
	}

	agent.runOnce(context.Background())
	if _, ok := reportBuffer.Peek(); !ok {
		t.Fatal("expected report to be buffered on send failure")
	}
}
