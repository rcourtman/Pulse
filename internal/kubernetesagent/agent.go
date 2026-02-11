package kubernetesagent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/IGLOU-EU/go-wildcard/v2"
	"github.com/rcourtman/pulse-go-rewrite/internal/buffer"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	PulseURL           string
	APIToken           string
	Interval           time.Duration
	AgentID            string
	AgentType          string // "unified" when running as part of pulse-agent
	AgentVersion       string // Version to report; if empty, uses kubernetesagent.Version
	InsecureSkipVerify bool
	LogLevel           zerolog.Level
	Logger             *zerolog.Logger

	// Kubernetes connection
	KubeconfigPath string
	KubeContext    string

	// Report shaping
	IncludeNamespaces     []string
	ExcludeNamespaces     []string
	IncludeAllPods        bool // Include all non-succeeded pods (still capped)
	IncludeAllDeployments bool // Include all deployments, not just problem ones
	MaxPods               int  // Max pods included in the report
}

type Agent struct {
	cfg        Config
	logger     zerolog.Logger
	httpClient *http.Client

	kubeClient kubernetes.Interface
	restCfg    *rest.Config

	agentID      string
	agentVersion string
	interval     time.Duration
	pulseURL     string

	clusterID      string
	clusterName    string
	clusterServer  string
	clusterContext string
	clusterVersion string

	includeNamespaces []string
	excludeNamespaces []string

	reportBuffer *buffer.Queue[agentsk8s.Report]
}

const (
	defaultInterval = 30 * time.Second
	defaultMaxPods  = 200
	requestTimeout  = 20 * time.Second
	reportUserAgent = "pulse-kubernetes-agent/"
)

func New(cfg Config) (*Agent, error) {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.MaxPods <= 0 {
		cfg.MaxPods = defaultMaxPods
	}

	if zerolog.GlobalLevel() == zerolog.DebugLevel && cfg.LogLevel != zerolog.DebugLevel {
		zerolog.SetGlobalLevel(cfg.LogLevel)
	}

	logger := cfg.Logger
	if logger == nil {
		defaultLogger := zerolog.New(os.Stdout).Level(cfg.LogLevel).With().Timestamp().Str("component", "pulse-kubernetes-agent").Logger()
		logger = &defaultLogger
	} else {
		scoped := logger.With().Str("component", "pulse-kubernetes-agent").Logger()
		logger = &scoped
	}

	if strings.TrimSpace(cfg.APIToken) == "" {
		return nil, fmt.Errorf("api token is required")
	}

	pulseURL := strings.TrimSpace(cfg.PulseURL)
	if pulseURL == "" {
		pulseURL = "http://localhost:7655"
	}
	normalizedPulseURL, err := normalizePulseURL(pulseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid pulse url: %w", err)
	}
	pulseURL = normalizedPulseURL
	cfg.PulseURL = pulseURL

	restCfg, contextName, err := buildRESTConfig(cfg.KubeconfigPath, cfg.KubeContext)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	agentVersion := strings.TrimSpace(cfg.AgentVersion)
	if agentVersion == "" {
		agentVersion = Version
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.InsecureSkipVerify {
		//nolint:gosec // Insecure mode is explicitly user-controlled.
		tlsConfig.InsecureSkipVerify = true
	}
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
		// Disallow redirects for agent API calls. If a reverse proxy redirects
		// HTTP to HTTPS, Go's default behavior converts POST to GET (per HTTP spec),
		// causing 405 errors. Return an error with guidance instead.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("server returned redirect to %s - if using a reverse proxy, ensure you use the correct protocol (https:// instead of http://) in your --url flag", req.URL)
		},
	}

	clusterServer := strings.TrimSpace(restCfg.Host)
	clusterContext := strings.TrimSpace(contextName)
	clusterName := clusterContext
	clusterID := computeClusterID(clusterServer, clusterContext, clusterName)

	agentID := strings.TrimSpace(cfg.AgentID)
	if agentID == "" {
		agentID = clusterID
	}

	agent := &Agent{
		cfg:               cfg,
		logger:            *logger,
		httpClient:        httpClient,
		kubeClient:        kubeClient,
		restCfg:           restCfg,
		agentID:           agentID,
		agentVersion:      agentVersion,
		interval:          cfg.Interval,
		pulseURL:          pulseURL,
		clusterID:         clusterID,
		clusterName:       clusterName,
		clusterServer:     clusterServer,
		clusterContext:    clusterContext,
		includeNamespaces: cfg.IncludeNamespaces,
		excludeNamespaces: cfg.ExcludeNamespaces,
		reportBuffer:      buffer.New[agentsk8s.Report](60),
	}

	if err := agent.discoverClusterMetadata(context.Background()); err != nil {
		agent.logger.Warn().Err(err).Msg("Failed to discover Kubernetes cluster metadata")
	}

	agent.logger.Info().
		Str("cluster_id", agent.clusterID).
		Str("cluster_name", agent.clusterName).
		Str("server", agent.clusterServer).
		Str("context", agent.clusterContext).
		Msg("Kubernetes agent initialized")

	return agent, nil
}

func normalizePulseURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("pulse URL %q is invalid: %w", rawURL, err)
	}

	if parsed.Scheme == "" {
		return "", fmt.Errorf("pulse URL %q must include scheme (https:// or loopback http://)", rawURL)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("pulse URL %q must include host", rawURL)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("pulse URL %q must not include user credentials", rawURL)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("pulse URL %q must not include query or fragment", rawURL)
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "https":
		// Always allowed.
	case "http":
		if !isLoopbackPulseHost(parsed.Hostname()) {
			return "", fmt.Errorf("pulse URL %q must use https unless host is loopback", rawURL)
		}
	default:
		return "", fmt.Errorf("pulse URL %q has unsupported scheme %q", rawURL, parsed.Scheme)
	}

	parsed.Scheme = scheme
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = strings.TrimRight(parsed.RawPath, "/")

	return parsed.String(), nil
}

func isLoopbackPulseHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func buildRESTConfig(kubeconfigPath, kubeContext string) (*rest.Config, string, error) {
	kubeconfigPath = strings.TrimSpace(kubeconfigPath)
	kubeContext = strings.TrimSpace(kubeContext)

	// Prefer explicit kubeconfig.
	if kubeconfigPath != "" {
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
		overrides := &clientcmd.ConfigOverrides{}
		if kubeContext != "" {
			overrides.CurrentContext = kubeContext
		}

		cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
		rawCfg, err := cc.RawConfig()
		if err != nil {
			return nil, "", fmt.Errorf("load kubeconfig: %w", err)
		}

		contextName := rawCfg.CurrentContext
		if kubeContext != "" {
			contextName = kubeContext
		}

		restCfg, err := cc.ClientConfig()
		if err != nil {
			return nil, "", fmt.Errorf("build kubeconfig rest config: %w", err)
		}
		return restCfg, contextName, nil
	}

	// Otherwise try in-cluster configuration.
	restCfg, err := rest.InClusterConfig()
	if err == nil {
		return restCfg, "in-cluster", nil
	}

	// Fallback: default kubeconfig path.
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
	)
	rawCfg, rawErr := cc.RawConfig()
	if rawErr != nil {
		return nil, "", fmt.Errorf("kubernetes config not available (in-cluster failed: %v; kubeconfig failed: %w)", err, rawErr)
	}

	contextName := rawCfg.CurrentContext
	if kubeContext != "" {
		contextName = kubeContext
	}

	restCfg, cfgErr := cc.ClientConfig()
	if cfgErr != nil {
		return nil, "", fmt.Errorf("build kubeconfig rest config: %w", cfgErr)
	}
	return restCfg, contextName, nil
}

func computeClusterID(server, context, name string) string {
	payload := strings.TrimSpace(server) + "|" + strings.TrimSpace(context) + "|" + strings.TrimSpace(name)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (a *Agent) discoverClusterMetadata(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	version, err := a.kubeClient.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	if version != nil {
		a.clusterVersion = strings.TrimSpace(version.GitVersion)
	}
	return nil
}

func (a *Agent) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	a.runOnce(ctx)

	for {
		select {
		case <-ticker.C:
			a.runOnce(ctx)
		case <-ctx.Done():
			return nil
		}
	}
}

func (a *Agent) runOnce(ctx context.Context) {
	a.flushReports(ctx)

	report, err := a.collectReport(ctx)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to collect Kubernetes report")
		return
	}

	if err := a.sendReport(ctx, report); err != nil {
		a.logger.Warn().Err(err).Msg("Failed to send Kubernetes report, buffering")
		a.reportBuffer.Push(report)
	}
}

func (a *Agent) flushReports(ctx context.Context) {
	for {
		report, ok := a.reportBuffer.Peek()
		if !ok {
			return
		}
		if err := a.sendReport(ctx, report); err != nil {
			return
		}
		_, _ = a.reportBuffer.Pop()
	}
}

func (a *Agent) namespaceAllowed(ns string) bool {
	ns = strings.TrimSpace(ns)
	if ns == "" {
		return false
	}
	for _, excludeNamespace := range a.excludeNamespaces {
		if wildcard.Match(excludeNamespace, ns) {
			return false
		}
	}
	if len(a.includeNamespaces) == 0 {
		return true
	}
	for _, includeNamespace := range a.includeNamespaces {
		if wildcard.Match(includeNamespace, ns) {
			return true
		}
	}
	return false
}

func (a *Agent) collectReport(ctx context.Context) (agentsk8s.Report, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	nodes, err := a.collectNodes(ctx)
	if err != nil {
		return agentsk8s.Report{}, err
	}

	pods, err := a.collectPods(ctx)
	if err != nil {
		return agentsk8s.Report{}, err
	}

	deployments, err := a.collectDeployments(ctx)
	if err != nil {
		return agentsk8s.Report{}, err
	}

	nodeUsage, podUsage, usageErr := a.collectUsageMetrics(ctx, nodes)
	if usageErr != nil {
		a.logger.Debug().Err(usageErr).Msg("Kubernetes usage metrics unavailable; continuing with inventory-only report")
	}
	applyNodeUsage(nodes, nodeUsage)
	applyPodUsage(pods, podUsage)

	return agentsk8s.Report{
		Agent: agentsk8s.AgentInfo{
			ID:              a.agentID,
			Version:         a.agentVersion,
			Type:            strings.TrimSpace(a.cfg.AgentType),
			IntervalSeconds: int(a.interval / time.Second),
		},
		Cluster: agentsk8s.ClusterInfo{
			ID:      a.clusterID,
			Name:    a.clusterName,
			Server:  a.clusterServer,
			Context: a.clusterContext,
			Version: a.clusterVersion,
		},
		Nodes:       nodes,
		Pods:        pods,
		Deployments: deployments,
		Timestamp:   time.Now().UTC(),
	}, nil
}

func (a *Agent) collectUsageMetrics(ctx context.Context, nodes []agentsk8s.Node) (map[string]agentsk8s.NodeUsage, map[string]agentsk8s.PodUsage, error) {
	if a == nil || a.kubeClient == nil {
		return nil, nil, nil
	}

	discovery := a.kubeClient.Discovery()
	if discovery == nil || discovery.RESTClient() == nil {
		return nil, nil, nil
	}

	restClient := discovery.RESTClient()

	nodeRaw, nodeErr := restClient.Get().AbsPath("/apis/metrics.k8s.io/v1beta1/nodes").DoRaw(ctx)
	podRaw, podErr := restClient.Get().AbsPath("/apis/metrics.k8s.io/v1beta1/pods").DoRaw(ctx)

	if nodeErr != nil && podErr != nil {
		return nil, nil, fmt.Errorf("metrics.k8s.io unavailable (nodes: %w; pods: %v)", nodeErr, podErr)
	}

	nodeUsage := map[string]agentsk8s.NodeUsage{}
	if nodeErr == nil {
		parsed, err := parseNodeMetricsPayload(nodeRaw)
		if err != nil {
			a.logger.Debug().Err(err).Msg("Failed to parse Kubernetes node metrics payload")
		} else {
			nodeUsage = parsed
		}
	}

	podUsage := map[string]agentsk8s.PodUsage{}
	if podErr == nil {
		parsed, err := parsePodMetricsPayload(podRaw)
		if err != nil {
			a.logger.Debug().Err(err).Msg("Failed to parse Kubernetes pod metrics payload")
		} else {
			podUsage = parsed
		}
	}

	summaryUsage, summaryErr := a.collectPodSummaryMetrics(ctx, nodes)
	if summaryErr != nil {
		a.logger.Debug().Err(summaryErr).Msg("Failed to collect Kubernetes pod summary metrics")
	}
	mergePodSummaryUsage(podUsage, summaryUsage)

	if nodeErr != nil && podErr != nil && len(podUsage) == 0 && len(nodeUsage) == 0 {
		if summaryErr != nil {
			return nil, nil, fmt.Errorf("metrics.k8s.io unavailable (nodes: %w; pods: %v); summary unavailable: %v", nodeErr, podErr, summaryErr)
		}
		return nil, nil, fmt.Errorf("metrics.k8s.io unavailable (nodes: %w; pods: %v)", nodeErr, podErr)
	}

	return nodeUsage, podUsage, nil
}

type podSummaryUsage struct {
	NetworkRxBytes                int64
	NetworkTxBytes                int64
	EphemeralStorageUsedBytes     int64
	EphemeralStorageCapacityBytes int64
}

func (a *Agent) collectPodSummaryMetrics(ctx context.Context, nodes []agentsk8s.Node) (map[string]podSummaryUsage, error) {
	if a == nil || a.kubeClient == nil {
		return nil, nil
	}

	discovery := a.kubeClient.Discovery()
	if discovery == nil || discovery.RESTClient() == nil {
		return nil, nil
	}

	restClient := discovery.RESTClient()
	result := make(map[string]podSummaryUsage)
	var failed int
	var succeeded int

	for _, node := range nodes {
		nodeName := strings.TrimSpace(node.Name)
		if nodeName == "" {
			continue
		}

		path := "/api/v1/nodes/" + url.PathEscape(nodeName) + "/proxy/stats/summary"
		raw, err := restClient.Get().AbsPath(path).DoRaw(ctx)
		if err != nil {
			failed++
			continue
		}
		succeeded++

		parsed, parseErr := parsePodSummaryMetricsPayload(raw)
		if parseErr != nil {
			failed++
			continue
		}
		for key, usage := range parsed {
			existing := result[key]
			if usage.NetworkRxBytes > existing.NetworkRxBytes {
				existing.NetworkRxBytes = usage.NetworkRxBytes
			}
			if usage.NetworkTxBytes > existing.NetworkTxBytes {
				existing.NetworkTxBytes = usage.NetworkTxBytes
			}
			if usage.EphemeralStorageUsedBytes > existing.EphemeralStorageUsedBytes {
				existing.EphemeralStorageUsedBytes = usage.EphemeralStorageUsedBytes
			}
			if usage.EphemeralStorageCapacityBytes > existing.EphemeralStorageCapacityBytes {
				existing.EphemeralStorageCapacityBytes = usage.EphemeralStorageCapacityBytes
			}
			result[key] = existing
		}
	}

	if succeeded == 0 && failed > 0 {
		return nil, fmt.Errorf("no node summary metrics endpoints available")
	}
	return result, nil
}

func parseNodeMetricsPayload(raw []byte) (map[string]agentsk8s.NodeUsage, error) {
	var payload struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Usage map[string]string `json:"usage"`
		} `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	result := make(map[string]agentsk8s.NodeUsage, len(payload.Items))
	for _, item := range payload.Items {
		name := strings.TrimSpace(item.Metadata.Name)
		if name == "" {
			continue
		}

		cpuMilli := parseCPUMilli(item.Usage["cpu"])
		memBytes := parseBytes(item.Usage["memory"])
		if cpuMilli <= 0 && memBytes <= 0 {
			continue
		}

		result[name] = agentsk8s.NodeUsage{
			CPUMilliCores: cpuMilli,
			MemoryBytes:   memBytes,
		}
	}
	return result, nil
}

func parsePodMetricsPayload(raw []byte) (map[string]agentsk8s.PodUsage, error) {
	var payload struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Containers []struct {
				Usage map[string]string `json:"usage"`
			} `json:"containers"`
		} `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	result := make(map[string]agentsk8s.PodUsage, len(payload.Items))
	for _, item := range payload.Items {
		key := podUsageKey(item.Metadata.Namespace, item.Metadata.Name)
		if key == "" {
			continue
		}

		var cpuMilli int64
		var memBytes int64
		for _, container := range item.Containers {
			cpuMilli += parseCPUMilli(container.Usage["cpu"])
			memBytes += parseBytes(container.Usage["memory"])
		}
		if cpuMilli <= 0 && memBytes <= 0 {
			continue
		}
		result[key] = agentsk8s.PodUsage{
			CPUMilliCores: cpuMilli,
			MemoryBytes:   memBytes,
		}
	}

	return result, nil
}

func parsePodSummaryMetricsPayload(raw []byte) (map[string]podSummaryUsage, error) {
	var payload struct {
		Pods []struct {
			PodRef struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"podRef"`
			Network struct {
				RxBytes *uint64 `json:"rxBytes"`
				TxBytes *uint64 `json:"txBytes"`
			} `json:"network"`
			EphemeralStorage struct {
				UsedBytes     *uint64 `json:"usedBytes"`
				CapacityBytes *uint64 `json:"capacityBytes"`
			} `json:"ephemeral-storage"`
			Volume []struct {
				UsedBytes     *uint64 `json:"usedBytes"`
				CapacityBytes *uint64 `json:"capacityBytes"`
			} `json:"volume"`
		} `json:"pods"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	result := make(map[string]podSummaryUsage, len(payload.Pods))
	for _, pod := range payload.Pods {
		key := podUsageKey(pod.PodRef.Namespace, pod.PodRef.Name)
		if key == "" {
			continue
		}

		usedBytes := int64FromUint64Ptr(pod.EphemeralStorage.UsedBytes)
		capacityBytes := int64FromUint64Ptr(pod.EphemeralStorage.CapacityBytes)
		if usedBytes <= 0 || capacityBytes <= 0 {
			var volumeUsed int64
			var volumeCapacity int64
			for _, volume := range pod.Volume {
				volumeUsed += int64FromUint64Ptr(volume.UsedBytes)
				volumeCapacity += int64FromUint64Ptr(volume.CapacityBytes)
			}
			if usedBytes <= 0 && volumeUsed > 0 {
				usedBytes = volumeUsed
			}
			if capacityBytes <= 0 && volumeCapacity > 0 {
				capacityBytes = volumeCapacity
			}
		}

		result[key] = podSummaryUsage{
			NetworkRxBytes:                int64FromUint64Ptr(pod.Network.RxBytes),
			NetworkTxBytes:                int64FromUint64Ptr(pod.Network.TxBytes),
			EphemeralStorageUsedBytes:     usedBytes,
			EphemeralStorageCapacityBytes: capacityBytes,
		}
	}

	return result, nil
}

func mergePodSummaryUsage(podUsage map[string]agentsk8s.PodUsage, summary map[string]podSummaryUsage) {
	if len(summary) == 0 {
		return
	}
	for key, usage := range summary {
		merged := podUsage[key]
		if usage.NetworkRxBytes > 0 {
			merged.NetworkRxBytes = usage.NetworkRxBytes
		}
		if usage.NetworkTxBytes > 0 {
			merged.NetworkTxBytes = usage.NetworkTxBytes
		}
		if usage.EphemeralStorageUsedBytes > 0 {
			merged.EphemeralStorageUsedBytes = usage.EphemeralStorageUsedBytes
		}
		if usage.EphemeralStorageCapacityBytes > 0 {
			merged.EphemeralStorageCapacityBytes = usage.EphemeralStorageCapacityBytes
		}
		if hasPodUsage(merged) {
			podUsage[key] = merged
		}
	}
}

func hasPodUsage(usage agentsk8s.PodUsage) bool {
	return usage.CPUMilliCores > 0 ||
		usage.MemoryBytes > 0 ||
		usage.NetworkRxBytes > 0 ||
		usage.NetworkTxBytes > 0 ||
		usage.EphemeralStorageUsedBytes > 0 ||
		usage.EphemeralStorageCapacityBytes > 0
}

func parseCPUMilli(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	quantity, err := k8sresource.ParseQuantity(value)
	if err != nil {
		return 0
	}
	return quantity.MilliValue()
}

func int64FromUint64Ptr(value *uint64) int64 {
	if value == nil {
		return 0
	}
	const maxInt64 = ^uint64(0) >> 1
	if *value > maxInt64 {
		return int64(maxInt64)
	}
	return int64(*value)
}

func parseBytes(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	quantity, err := k8sresource.ParseQuantity(value)
	if err != nil {
		return 0
	}
	return quantity.Value()
}

func podUsageKey(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return ""
	}
	return namespace + "/" + name
}

func applyNodeUsage(nodes []agentsk8s.Node, usage map[string]agentsk8s.NodeUsage) {
	if len(nodes) == 0 || len(usage) == 0 {
		return
	}
	for i := range nodes {
		if nodeUsage, ok := usage[strings.TrimSpace(nodes[i].Name)]; ok {
			u := nodeUsage
			nodes[i].Usage = &u
		}
	}
}

func applyPodUsage(pods []agentsk8s.Pod, usage map[string]agentsk8s.PodUsage) {
	if len(pods) == 0 || len(usage) == 0 {
		return
	}
	for i := range pods {
		key := podUsageKey(pods[i].Namespace, pods[i].Name)
		if key == "" {
			continue
		}
		if podUsage, ok := usage[key]; ok {
			u := podUsage
			pods[i].Usage = &u
		}
	}
}

func (a *Agent) collectNodes(ctx context.Context) ([]agentsk8s.Node, error) {
	list, err := a.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	nodes := make([]agentsk8s.Node, 0, len(list.Items))
	for _, node := range list.Items {
		ready := isNodeReady(node)
		nodes = append(nodes, agentsk8s.Node{
			UID:                     string(node.UID),
			Name:                    node.Name,
			Ready:                   ready,
			Unschedulable:           node.Spec.Unschedulable,
			KubeletVersion:          node.Status.NodeInfo.KubeletVersion,
			ContainerRuntimeVersion: node.Status.NodeInfo.ContainerRuntimeVersion,
			OSImage:                 node.Status.NodeInfo.OSImage,
			KernelVersion:           node.Status.NodeInfo.KernelVersion,
			Architecture:            node.Status.NodeInfo.Architecture,
			Capacity:                toNodeResources(node.Status.Capacity),
			Allocatable:             toNodeResources(node.Status.Allocatable),
			Roles:                   rolesFromNodeLabels(node.Labels),
		})
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	return nodes, nil
}

func isNodeReady(node corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func rolesFromNodeLabels(nodeLabels map[string]string) []string {
	roles := make([]string, 0, 4)
	for k := range nodeLabels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
			role = strings.TrimSpace(role)
			if role == "" {
				role = "master"
			}
			roles = append(roles, role)
		}
	}
	if v := strings.TrimSpace(nodeLabels["kubernetes.io/role"]); v != "" {
		roles = append(roles, v)
	}
	roles = dedupeStrings(roles)
	sort.Strings(roles)
	return roles
}

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func toNodeResources(list corev1.ResourceList) agentsk8s.NodeResources {
	cpu := list[corev1.ResourceCPU]
	mem := list[corev1.ResourceMemory]
	pods := list[corev1.ResourcePods]

	return agentsk8s.NodeResources{
		CPUCores:    cpu.MilliValue() / 1000,
		MemoryBytes: mem.Value(),
		Pods:        pods.Value(),
	}
}

func (a *Agent) collectPods(ctx context.Context) ([]agentsk8s.Pod, error) {
	// Default: focus on non-succeeded pods to reduce payload size.
	selector := fields.OneTermNotEqualSelector("status.phase", string(corev1.PodSucceeded))
	listOpts := metav1.ListOptions{FieldSelector: selector.String()}

	podList, err := a.kubeClient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	items := make([]agentsk8s.Pod, 0)
	for _, pod := range podList.Items {
		if !a.namespaceAllowed(pod.Namespace) {
			continue
		}
		if !a.cfg.IncludeAllPods && !isProblemPod(pod) {
			continue
		}

		labelsCopy := make(map[string]string, len(pod.Labels))
		for k, v := range pod.Labels {
			labelsCopy[k] = v
		}

		containers := make([]agentsk8s.PodContainer, 0, len(pod.Status.ContainerStatuses))
		restarts := 0
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += int(cs.RestartCount)
			state, reason, message := summarizeContainerState(cs)
			containers = append(containers, agentsk8s.PodContainer{
				Name:         cs.Name,
				Image:        cs.Image,
				Ready:        cs.Ready,
				RestartCount: cs.RestartCount,
				State:        state,
				Reason:       reason,
				Message:      message,
			})
		}

		ownerKind, ownerName := ownerRef(pod.OwnerReferences)
		createdAt := pod.CreationTimestamp.Time
		var startTime *time.Time
		if pod.Status.StartTime != nil {
			t := pod.Status.StartTime.Time
			startTime = &t
		}

		items = append(items, agentsk8s.Pod{
			UID:        string(pod.UID),
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			NodeName:   pod.Spec.NodeName,
			Phase:      string(pod.Status.Phase),
			Reason:     pod.Status.Reason,
			Message:    pod.Status.Message,
			QoSClass:   string(pod.Status.QOSClass),
			CreatedAt:  createdAt,
			StartTime:  startTime,
			Restarts:   restarts,
			Labels:     labelsCopy,
			OwnerKind:  ownerKind,
			OwnerName:  ownerName,
			Containers: containers,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace == items[j].Namespace {
			return items[i].Name < items[j].Name
		}
		return items[i].Namespace < items[j].Namespace
	})

	if len(items) > a.cfg.MaxPods {
		items = items[:a.cfg.MaxPods]
	}

	return items, nil
}

func isProblemPod(pod corev1.Pod) bool {
	switch pod.Status.Phase {
	case corev1.PodPending, corev1.PodFailed, corev1.PodUnknown:
		return true
	}

	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			return true
		}
		if cs.State.Waiting != nil {
			return true
		}
		if !cs.Ready && (cs.State.Running == nil) {
			return true
		}
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil || cs.State.Terminated != nil {
			return true
		}
		if !cs.Ready {
			return true
		}
	}

	return false
}

func summarizeContainerState(cs corev1.ContainerStatus) (string, string, string) {
	switch {
	case cs.State.Running != nil:
		return "running", "", ""
	case cs.State.Waiting != nil:
		return "waiting", strings.TrimSpace(cs.State.Waiting.Reason), strings.TrimSpace(cs.State.Waiting.Message)
	case cs.State.Terminated != nil:
		return "terminated", strings.TrimSpace(cs.State.Terminated.Reason), strings.TrimSpace(cs.State.Terminated.Message)
	default:
		return "unknown", "", ""
	}
}

func ownerRef(refs []metav1.OwnerReference) (string, string) {
	for _, r := range refs {
		if r.Controller != nil && *r.Controller {
			return r.Kind, r.Name
		}
	}
	if len(refs) > 0 {
		return refs[0].Kind, refs[0].Name
	}
	return "", ""
}

func (a *Agent) collectDeployments(ctx context.Context) ([]agentsk8s.Deployment, error) {
	depList, err := a.kubeClient.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	items := make([]agentsk8s.Deployment, 0)
	for _, dep := range depList.Items {
		if !a.namespaceAllowed(dep.Namespace) {
			continue
		}
		if !a.cfg.IncludeAllDeployments && !isProblemDeployment(dep) {
			continue
		}

		labelsCopy := make(map[string]string, len(dep.Labels))
		for k, v := range dep.Labels {
			labelsCopy[k] = v
		}

		items = append(items, agentsk8s.Deployment{
			UID:               string(dep.UID),
			Name:              dep.Name,
			Namespace:         dep.Namespace,
			DesiredReplicas:   desiredReplicas(dep),
			UpdatedReplicas:   dep.Status.UpdatedReplicas,
			ReadyReplicas:     dep.Status.ReadyReplicas,
			AvailableReplicas: dep.Status.AvailableReplicas,
			Labels:            labelsCopy,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace == items[j].Namespace {
			return items[i].Name < items[j].Name
		}
		return items[i].Namespace < items[j].Namespace
	})

	return items, nil
}

func desiredReplicas(dep appsv1.Deployment) int32 {
	if dep.Spec.Replicas == nil {
		return 0
	}
	return *dep.Spec.Replicas
}

func isProblemDeployment(dep appsv1.Deployment) bool {
	desired := desiredReplicas(dep)
	if desired <= 0 {
		return false
	}
	return dep.Status.AvailableReplicas < desired || dep.Status.ReadyReplicas < desired || dep.Status.UpdatedReplicas < desired
}

func (a *Agent) sendReport(ctx context.Context, report agentsk8s.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	url := fmt.Sprintf("%s/api/agents/kubernetes/report", a.pulseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIToken)
	req.Header.Set("X-API-Token", a.cfg.APIToken)
	req.Header.Set("User-Agent", reportUserAgent+a.agentVersion)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("pulse responded with status %s: %s", resp.Status, msg)
		}
		return fmt.Errorf("pulse responded with status %s", resp.Status)
	}

	return nil
}
