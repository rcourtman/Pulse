package kubernetesagent

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
)

type kubeClientWithDiscovery struct {
	kubernetes.Interface
	discoveryClient discovery.DiscoveryInterface
}

func (c *kubeClientWithDiscovery) Discovery() discovery.DiscoveryInterface {
	return c.discoveryClient
}

type discoveryWithREST struct {
	discovery.DiscoveryInterface
	restClient rest.Interface
}

func (d *discoveryWithREST) RESTClient() rest.Interface {
	return d.restClient
}

func newTestRESTClient(handler func(path string) (int, string)) *restfake.RESTClient {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	return &restfake.RESTClient{
		GroupVersion:         schema.GroupVersion{Version: "v1"},
		NegotiatedSerializer: serializer.WithoutConversionCodecFactory{CodecFactory: codecs},
		VersionedAPIPath:     "",
		Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			status, body := handler(req.URL.Path)
			if status == 0 {
				status = http.StatusNotFound
			}
			if body == "" {
				body = `{}`
			}
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
}

func newTestAgentWithREST(restClient rest.Interface) *Agent {
	base := fake.NewSimpleClientset()
	return &Agent{
		logger: zerolog.New(io.Discard),
		kubeClient: &kubeClientWithDiscovery{
			Interface: base,
			discoveryClient: &discoveryWithREST{
				DiscoveryInterface: base.Discovery(),
				restClient:         restClient,
			},
		},
	}
}

func TestParseNodeMetricsPayload(t *testing.T) {
	raw := []byte(`{
		"items": [
			{"metadata": {"name": "node-a"}, "usage": {"cpu": "150m", "memory": "1Gi"}},
			{"metadata": {"name": "node-b"}, "usage": {"cpu": "2"}},
			{"metadata": {"name": "node-c"}, "usage": {"cpu": "bad", "memory": "bad"}},
			{"metadata": {"name": ""}, "usage": {"cpu": "100m", "memory": "128Mi"}}
		]
	}`)

	usage, err := parseNodeMetricsPayload(raw)
	if err != nil {
		t.Fatalf("parseNodeMetricsPayload: %v", err)
	}

	if len(usage) != 2 {
		t.Fatalf("expected 2 usable node entries, got %d", len(usage))
	}

	nodeA := usage["node-a"]
	if nodeA.CPUMilliCores != 150 || nodeA.MemoryBytes != 1073741824 {
		t.Fatalf("unexpected node-a usage: %+v", nodeA)
	}

	nodeB := usage["node-b"]
	if nodeB.CPUMilliCores != 2000 || nodeB.MemoryBytes != 0 {
		t.Fatalf("unexpected node-b usage: %+v", nodeB)
	}
}

func TestParseNodeMetricsPayload_InvalidJSON(t *testing.T) {
	if _, err := parseNodeMetricsPayload([]byte("{")); err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
}

func TestParsePodMetricsPayload(t *testing.T) {
	raw := []byte(`{
		"items": [
			{
				"metadata": {"namespace": "default", "name": "api"},
				"containers": [
					{"usage": {"cpu": "100m", "memory": "64Mi"}},
					{"usage": {"cpu": "250m", "memory": "128Mi"}}
				]
			},
			{
				"metadata": {"namespace": "ops", "name": "worker"},
				"containers": [
					{"usage": {"cpu": "bad", "memory": "bad"}}
				]
			},
			{
				"metadata": {"namespace": "", "name": "skip"},
				"containers": [
					{"usage": {"cpu": "100m", "memory": "128Mi"}}
				]
			}
		]
	}`)

	usage, err := parsePodMetricsPayload(raw)
	if err != nil {
		t.Fatalf("parsePodMetricsPayload: %v", err)
	}

	if len(usage) != 1 {
		t.Fatalf("expected 1 usable pod entry, got %d", len(usage))
	}

	pod := usage["default/api"]
	if pod.CPUMilliCores != 350 || pod.MemoryBytes != 201326592 {
		t.Fatalf("unexpected pod usage: %+v", pod)
	}
}

func TestParsePodMetricsPayload_InvalidJSON(t *testing.T) {
	if _, err := parsePodMetricsPayload([]byte("{")); err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
}

func TestParseQuantityHelpers(t *testing.T) {
	if got := parseCPUMilli(" "); got != 0 {
		t.Fatalf("parseCPUMilli(empty) = %d, want 0", got)
	}
	if got := parseCPUMilli("bad"); got != 0 {
		t.Fatalf("parseCPUMilli(bad) = %d, want 0", got)
	}
	if got := parseCPUMilli("3"); got != 3000 {
		t.Fatalf("parseCPUMilli(3) = %d, want 3000", got)
	}

	if got := parseBytes(" "); got != 0 {
		t.Fatalf("parseBytes(empty) = %d, want 0", got)
	}
	if got := parseBytes("bad"); got != 0 {
		t.Fatalf("parseBytes(bad) = %d, want 0", got)
	}
	if got := parseBytes("1Ki"); got != 1024 {
		t.Fatalf("parseBytes(1Ki) = %d, want 1024", got)
	}
}

func TestApplyNodeAndPodUsage(t *testing.T) {
	nodes := []agentsk8s.Node{{Name: "node-a"}, {Name: "node-b"}, {Name: "missing"}}
	applyNodeUsage(nodes, map[string]agentsk8s.NodeUsage{
		"node-a": {CPUMilliCores: 100, MemoryBytes: 256},
		"node-b": {CPUMilliCores: 200, MemoryBytes: 512},
	})
	if nodes[0].Usage == nil || nodes[0].Usage.CPUMilliCores != 100 {
		t.Fatalf("expected node-a usage to be applied, got %+v", nodes[0].Usage)
	}
	if nodes[1].Usage == nil || nodes[1].Usage.MemoryBytes != 512 {
		t.Fatalf("expected node-b usage to be applied, got %+v", nodes[1].Usage)
	}
	if nodes[2].Usage != nil {
		t.Fatalf("expected unmatched node usage to remain nil, got %+v", nodes[2].Usage)
	}

	pods := []agentsk8s.Pod{
		{Namespace: "default", Name: "api"},
		{Namespace: "ops", Name: "worker"},
		{Name: "missing-namespace"},
	}
	applyPodUsage(pods, map[string]agentsk8s.PodUsage{
		"default/api": {CPUMilliCores: 150, MemoryBytes: 128},
		"ops/worker":  {CPUMilliCores: 300, MemoryBytes: 256},
	})
	if pods[0].Usage == nil || pods[0].Usage.CPUMilliCores != 150 {
		t.Fatalf("expected default/api usage to be applied, got %+v", pods[0].Usage)
	}
	if pods[1].Usage == nil || pods[1].Usage.MemoryBytes != 256 {
		t.Fatalf("expected ops/worker usage to be applied, got %+v", pods[1].Usage)
	}
	if pods[2].Usage != nil {
		t.Fatalf("expected invalid pod key usage to remain nil, got %+v", pods[2].Usage)
	}

	applyNodeUsage(nil, map[string]agentsk8s.NodeUsage{"node-a": {CPUMilliCores: 1}})
	applyPodUsage(nil, map[string]agentsk8s.PodUsage{"default/api": {CPUMilliCores: 1}})
}

func TestInt64FromUint64Ptr_ClampsOverflow(t *testing.T) {
	if got := int64FromUint64Ptr(nil); got != 0 {
		t.Fatalf("int64FromUint64Ptr(nil) = %d, want 0", got)
	}

	value := uint64(123)
	if got := int64FromUint64Ptr(&value); got != 123 {
		t.Fatalf("int64FromUint64Ptr(123) = %d, want 123", got)
	}

	overflow := ^uint64(0)
	if got := int64FromUint64Ptr(&overflow); got != int64(^uint64(0)>>1) {
		t.Fatalf("expected overflow clamp to max int64, got %d", got)
	}
}

func TestCollectUsageMetrics_MergesMetricsAndSummary(t *testing.T) {
	restClient := newTestRESTClient(func(path string) (int, string) {
		switch path {
		case "/apis/metrics.k8s.io/v1beta1/nodes":
			return http.StatusOK, `{"items":[{"metadata":{"name":"node-a"},"usage":{"cpu":"125m","memory":"256Mi"}}]}`
		case "/apis/metrics.k8s.io/v1beta1/pods":
			return http.StatusOK, `{"items":[{"metadata":{"namespace":"default","name":"api"},"containers":[{"usage":{"cpu":"100m","memory":"64Mi"}},{"usage":{"cpu":"250m","memory":"128Mi"}}]}]}`
		case "/api/v1/nodes/node-a/proxy/stats/summary":
			return http.StatusOK, `{"pods":[{"podRef":{"namespace":"default","name":"api"},"network":{"rxBytes":1000,"txBytes":2000},"ephemeral-storage":{"usedBytes":3000,"capacityBytes":6000}}]}`
		default:
			return http.StatusNotFound, `{"error":"not found"}`
		}
	})

	agent := newTestAgentWithREST(restClient)
	nodes := []agentsk8s.Node{{Name: "node-a"}}

	nodeUsage, podUsage, err := agent.collectUsageMetrics(context.Background(), nodes)
	if err != nil {
		t.Fatalf("collectUsageMetrics: %v", err)
	}

	nodeA := nodeUsage["node-a"]
	if nodeA.CPUMilliCores != 125 || nodeA.MemoryBytes != 268435456 {
		t.Fatalf("unexpected node usage: %+v", nodeA)
	}

	pod := podUsage["default/api"]
	if pod.CPUMilliCores != 350 || pod.MemoryBytes != 201326592 {
		t.Fatalf("unexpected pod cpu/memory usage: %+v", pod)
	}
	if pod.NetworkRxBytes != 1000 || pod.NetworkTxBytes != 2000 {
		t.Fatalf("unexpected pod network usage: %+v", pod)
	}
	if pod.EphemeralStorageUsedBytes != 3000 || pod.EphemeralStorageCapacityBytes != 6000 {
		t.Fatalf("unexpected pod ephemeral storage usage: %+v", pod)
	}
}

func TestCollectUsageMetrics_ReturnsErrorWhenAllBackendsUnavailable(t *testing.T) {
	restClient := newTestRESTClient(func(path string) (int, string) {
		switch path {
		case "/apis/metrics.k8s.io/v1beta1/nodes", "/apis/metrics.k8s.io/v1beta1/pods", "/api/v1/nodes/node-a/proxy/stats/summary":
			return http.StatusServiceUnavailable, `{"error":"unavailable"}`
		default:
			return http.StatusNotFound, `{"error":"not found"}`
		}
	})

	agent := newTestAgentWithREST(restClient)
	nodes := []agentsk8s.Node{{Name: "node-a"}}

	nodeUsage, podUsage, err := agent.collectUsageMetrics(context.Background(), nodes)
	if err == nil {
		t.Fatal("expected collectUsageMetrics error when metrics and summary are unavailable")
	}
	if !strings.Contains(err.Error(), "metrics.k8s.io unavailable") {
		t.Fatalf("expected metrics unavailability in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "summary unavailable") {
		t.Fatalf("expected summary unavailability in error, got %v", err)
	}
	if nodeUsage != nil || podUsage != nil {
		t.Fatalf("expected nil usage maps on terminal error, got nodes=%v pods=%v", nodeUsage, podUsage)
	}
}
