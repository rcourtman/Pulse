package kubernetesagent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IGLOU-EU/go-wildcard/v2"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config holds all configuration needed to run the Kubernetes agent.
// It specifies how to connect to the Pulse backend, the Kubernetes cluster,
// and what resources to include in reports.
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

// Agent collects and reports Kubernetes cluster state to Pulse.
// It periodically gathers pod, deployment, and node metrics, then sends
// them to the configured Pulse URL. The agent handles authentication,
// Kubernetes API connection, and automatic retry on failure.
type Agent struct {
	cfg        Config
	logger     zerolog.Logger
	httpClient *http.Client

	kubeClient     kubernetes.Interface
	metadataClient metadata.Interface
	restCfg        *rest.Config

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

	reportBuffer *utils.Queue[agentsk8s.Report]
}

const (
	defaultInterval                    = 30 * time.Second
	defaultMaxPods                     = 200
	defaultMaxDeployments              = 1000
	requestTimeout                     = 20 * time.Second
	collectReportTimeout               = 45 * time.Second
	listPageSize                 int64 = 250
	maxKubeAPIRetries                  = 3
	initialRetryBackoff                = 300 * time.Millisecond
	maxRetryBackoff                    = 3 * time.Second
	maxSummaryMetricNodes              = 200
	maxInventoryItems                  = 1000
	maxEventItems                      = 200
	summaryMetricsWorkers              = 8
	reportUserAgent                    = "pulse-kubernetes-agent/"
	maxMetricsResponseBodyBytes  int64 = 32 * 1024 * 1024 // 32 MB
	maxRecoveryResponseBodyBytes int64 = 8 * 1024 * 1024  // 8 MB (recovery APIs can be large)
)

var (
	configMapsGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	secretsGVR    = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
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
	pulseURL, err := normalizePulseURL(pulseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid pulse URL: %w", err)
	}
	cfg.PulseURL = pulseURL

	restCfg, contextName, err := buildRESTConfig(cfg.KubeconfigPath, cfg.KubeContext)
	if err != nil {
		return nil, fmt.Errorf("build Kubernetes REST config: %w", err)
	}
	if restCfg.Timeout <= 0 {
		restCfg.Timeout = requestTimeout
	}

	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	metadataClient, err := metadata.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes metadata client: %w", err)
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
		metadataClient:    metadataClient,
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
		reportBuffer:      utils.New[agentsk8s.Report](60),
	}

	if err := agent.discoverClusterMetadata(context.Background()); err != nil {
		agent.logger.Warn().Err(err).Str("cluster_id", agent.clusterID).Msg("failed to discover cluster metadata")
	}

	agent.logger.Info().
		Str("cluster_id", agent.clusterID).
		Str("cluster_name", agent.clusterName).
		Str("server", agent.clusterServer).
		Str("context", agent.clusterContext).
		Msg("kubernetes agent initialized")

	return agent, nil
}

func normalizePulseURL(rawURL string) (string, error) {
	parsed, err := securityutil.NormalizePulseHTTPBaseURLWithOptions(rawURL, securityutil.PulseURLValidationOptions{
		AllowLocalNetworkHTTP: true,
	})
	if err != nil {
		return "", err
	}

	return parsed.String(), nil
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
		return fmt.Errorf("discover cluster server version: %w", err)
	}
	if version != nil {
		a.clusterVersion = strings.TrimSpace(version.GitVersion)
	}
	return nil
}

func (a *Agent) closeIdleConnections() {
	if a.httpClient != nil {
		a.httpClient.CloseIdleConnections()
	}
}

func (a *Agent) Run(ctx context.Context) error {
	defer a.closeIdleConnections()

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

func (a *Agent) bufferedReportCount() int {
	if a == nil || a.reportBuffer == nil {
		return 0
	}
	return a.reportBuffer.Len()
}

func (a *Agent) runOnce(ctx context.Context) {
	a.flushReports(ctx)

	report, err := a.collectReport(ctx)
	if err != nil {
		a.logger.Warn().
			Err(err).
			Str("phase", "collect_report").
			Str("cluster_id", a.clusterID).
			Int("buffer_depth", a.bufferedReportCount()).
			Msg("Failed to collect Kubernetes report")
		return
	}

	if err := a.sendReport(ctx, report); err != nil {
		a.logger.Warn().
			Err(err).
			Str("phase", "send_report").
			Str("cluster_id", a.clusterID).
			Str("agent_id", a.agentID).
			Int("report_nodes", len(report.Nodes)).
			Int("report_pods", len(report.Pods)).
			Int("report_deployments", len(report.Deployments)).
			Int("buffer_depth_before", a.bufferedReportCount()).
			Msg("Failed to send Kubernetes report, buffering")
		a.reportBuffer.Push(report)
		a.logger.Debug().
			Str("phase", "buffer_report").
			Str("cluster_id", a.clusterID).
			Int("buffer_depth_after", a.bufferedReportCount()).
			Msg("Buffered Kubernetes report for retry")
	}
}

func (a *Agent) flushReports(ctx context.Context) {
	flushed := 0
	for {
		report, ok := a.reportBuffer.Peek()
		if !ok {
			if flushed > 0 {
				a.logger.Debug().
					Str("phase", "flush_buffered_reports").
					Str("cluster_id", a.clusterID).
					Str("agent_id", a.agentID).
					Int("flushed_reports", flushed).
					Int("buffer_depth_remaining", a.bufferedReportCount()).
					Msg("Flushed buffered Kubernetes reports")
			}
			return
		}
		if err := a.sendReport(ctx, report); err != nil {
			a.logger.Warn().
				Err(err).
				Str("phase", "flush_buffered_report").
				Str("cluster_id", a.clusterID).
				Str("agent_id", a.agentID).
				Int("report_nodes", len(report.Nodes)).
				Int("report_pods", len(report.Pods)).
				Int("report_deployments", len(report.Deployments)).
				Int("buffer_depth", a.bufferedReportCount()).
				Msg("Failed to flush buffered Kubernetes report")
			return
		}
		if _, ok := a.reportBuffer.Pop(); !ok {
			a.logger.Debug().Msg("Failed to remove buffered report after successful send")
			return
		}
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
	ctx, cancel := context.WithTimeout(ctx, collectReportTimeout)
	defer cancel()

	nodes, err := a.collectNodes(ctx)
	if err != nil {
		return agentsk8s.Report{}, fmt.Errorf("collect nodes: %w", err)
	}

	pods, err := a.collectPods(ctx)
	if err != nil {
		return agentsk8s.Report{}, fmt.Errorf("collect pods: %w", err)
	}

	deployments, err := a.collectDeployments(ctx)
	if err != nil {
		return agentsk8s.Report{}, fmt.Errorf("collect deployments: %w", err)
	}

	replicaSets, err := a.collectReplicaSets(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes replicasets unavailable, continuing without replicaset inventory")
	}
	namespaces, err := a.collectNamespaces(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes namespaces unavailable, continuing without namespace inventory")
	}
	services, err := a.collectServices(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes services unavailable, continuing without service inventory")
	}
	statefulSets, err := a.collectStatefulSets(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes statefulsets unavailable, continuing without statefulset inventory")
	}
	daemonSets, err := a.collectDaemonSets(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes daemonsets unavailable, continuing without daemonset inventory")
	}
	jobs, err := a.collectJobs(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes jobs unavailable, continuing without job inventory")
	}
	cronJobs, err := a.collectCronJobs(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes cronjobs unavailable, continuing without cronjob inventory")
	}
	horizontalPodAutoscalers, err := a.collectHorizontalPodAutoscalers(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes horizontalpodautoscalers unavailable, continuing without autoscaling inventory")
	}
	ingresses, err := a.collectIngresses(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes ingresses unavailable, continuing without ingress inventory")
	}
	endpointSlices, err := a.collectEndpointSlices(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes endpointslices unavailable, continuing without endpointslice inventory")
	}
	networkPolicies, err := a.collectNetworkPolicies(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes network policies unavailable, continuing without network policy inventory")
	}
	podDisruptionBudgets, err := a.collectPodDisruptionBudgets(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes poddisruptionbudgets unavailable, continuing without disruption budget inventory")
	}
	persistentVolumes, err := a.collectPersistentVolumes(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes persistent volumes unavailable, continuing without persistent volume inventory")
	}
	persistentVolumeClaims, err := a.collectPersistentVolumeClaims(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes persistent volume claims unavailable, continuing without persistent volume claim inventory")
	}
	storageClasses, err := a.collectStorageClasses(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes storage classes unavailable, continuing without storage class inventory")
	}
	configMaps, err := a.collectConfigMaps(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes configmaps unavailable, continuing without configmap inventory")
	}
	secrets, err := a.collectSecrets(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes secrets unavailable, continuing without secret metadata inventory")
	}
	serviceAccounts, err := a.collectServiceAccounts(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes serviceaccounts unavailable, continuing without serviceaccount inventory")
	}
	roles, err := a.collectRoles(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes roles unavailable, continuing without role inventory")
	}
	clusterRoles, err := a.collectClusterRoles(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes clusterroles unavailable, continuing without clusterrole inventory")
	}
	roleBindings, err := a.collectRoleBindings(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes rolebindings unavailable, continuing without rolebinding inventory")
	}
	clusterRoleBindings, err := a.collectClusterRoleBindings(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes clusterrolebindings unavailable, continuing without clusterrolebinding inventory")
	}
	resourceQuotas, err := a.collectResourceQuotas(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes resourcequotas unavailable, continuing without quota inventory")
	}
	limitRanges, err := a.collectLimitRanges(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes limitranges unavailable, continuing without limit range inventory")
	}
	events, err := a.collectEvents(ctx)
	if err != nil {
		a.logger.Debug().Err(err).Str("cluster_id", a.clusterID).Msg("kubernetes events unavailable, continuing without event inventory")
	}

	nodeUsage, podUsage, usageErr := a.collectUsageMetrics(ctx, nodes)
	if usageErr != nil {
		a.logger.Debug().Err(usageErr).Str("cluster_id", a.clusterID).Msg("kubernetes usage metrics unavailable, continuing with inventory-only report")
	}
	applyNodeUsage(nodes, nodeUsage)
	applyPodUsage(pods, podUsage)

	recoveryReport, recoveryErr := a.collectRecovery(ctx)
	if recoveryErr != nil {
		a.logger.Debug().Err(recoveryErr).Str("cluster_id", a.clusterID).Msg("kubernetes recovery artifacts unavailable, continuing without recovery report")
	}

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
		Nodes:                    nodes,
		Namespaces:               namespaces,
		Pods:                     pods,
		Deployments:              deployments,
		ReplicaSets:              replicaSets,
		StatefulSets:             statefulSets,
		DaemonSets:               daemonSets,
		Services:                 services,
		Jobs:                     jobs,
		CronJobs:                 cronJobs,
		HorizontalPodAutoscalers: horizontalPodAutoscalers,
		Ingresses:                ingresses,
		EndpointSlices:           endpointSlices,
		NetworkPolicies:          networkPolicies,
		PodDisruptionBudgets:     podDisruptionBudgets,
		PersistentVolumes:        persistentVolumes,
		PersistentVolumeClaims:   persistentVolumeClaims,
		StorageClasses:           storageClasses,
		ConfigMaps:               configMaps,
		Secrets:                  secrets,
		ServiceAccounts:          serviceAccounts,
		Roles:                    roles,
		ClusterRoles:             clusterRoles,
		RoleBindings:             roleBindings,
		ClusterRoleBindings:      clusterRoleBindings,
		ResourceQuotas:           resourceQuotas,
		LimitRanges:              limitRanges,
		Events:                   events,
		Recovery:                 recoveryReport,
		Timestamp:                time.Now().UTC(),
	}, nil
}

func (a *Agent) collectRecovery(ctx context.Context) (*agentsk8s.RecoveryReport, error) {
	restClient := a.getDiscoveryRESTClient()
	if restClient == nil {
		return nil, nil
	}

	volumeSnapshots, _ := a.collectVolumeSnapshots(ctx, restClient)
	veleroBackups, _ := a.collectVeleroBackups(ctx, restClient)

	if len(volumeSnapshots) == 0 && len(veleroBackups) == 0 {
		return nil, nil
	}

	return &agentsk8s.RecoveryReport{
		VolumeSnapshots: volumeSnapshots,
		VeleroBackups:   veleroBackups,
	}, nil
}

func (a *Agent) collectVolumeSnapshots(ctx context.Context, restClient rest.Interface) ([]agentsk8s.VolumeSnapshot, error) {
	raw, ok, err := a.doOptionalRawPath(ctx, restClient, "list volumesnapshots", "/apis/snapshot.storage.k8s.io/v1/volumesnapshots?limit=200")
	if err != nil || !ok || len(raw) == 0 {
		return nil, err
	}

	type vsError struct {
		Message string `json:"message"`
	}
	type vsItem struct {
		Metadata struct {
			UID               string    `json:"uid"`
			Name              string    `json:"name"`
			Namespace         string    `json:"namespace"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		Spec struct {
			VolumeSnapshotClassName string `json:"volumeSnapshotClassName"`
			Source                  struct {
				PersistentVolumeClaimName string `json:"persistentVolumeClaimName"`
			} `json:"source"`
		} `json:"spec"`
		Status struct {
			ReadyToUse                     *bool      `json:"readyToUse"`
			CreationTime                   *time.Time `json:"creationTime"`
			CompletionTime                 *time.Time `json:"completionTime"`
			BoundVolumeSnapshotContentName string     `json:"boundVolumeSnapshotContentName"`
			RestoreSize                    string     `json:"restoreSize"`
			Error                          *vsError   `json:"error"`
		} `json:"status"`
	}
	type vsList struct {
		Items []vsItem `json:"items"`
	}

	var parsed vsList
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse volumesnapshots: %w", err)
	}

	out := make([]agentsk8s.VolumeSnapshot, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		createdAt := item.Metadata.CreationTimestamp.UTC()
		name := strings.TrimSpace(item.Metadata.Name)
		namespace := strings.TrimSpace(item.Metadata.Namespace)
		if name == "" || namespace == "" {
			continue
		}

		var restoreSizeBytes *int64
		if strings.TrimSpace(item.Status.RestoreSize) != "" {
			if q, err := k8sresource.ParseQuantity(strings.TrimSpace(item.Status.RestoreSize)); err == nil {
				v := q.Value()
				restoreSizeBytes = &v
			}
		}

		errText := ""
		if item.Status.Error != nil {
			errText = strings.TrimSpace(item.Status.Error.Message)
		}

		creationTime := item.Status.CreationTime
		if creationTime == nil {
			creationTime = &createdAt
		}

		out = append(out, agentsk8s.VolumeSnapshot{
			UID:              strings.TrimSpace(item.Metadata.UID),
			Name:             name,
			Namespace:        namespace,
			SnapshotClass:    strings.TrimSpace(item.Spec.VolumeSnapshotClassName),
			SourcePVC:        strings.TrimSpace(item.Spec.Source.PersistentVolumeClaimName),
			ReadyToUse:       item.Status.ReadyToUse,
			RestoreSizeBytes: restoreSizeBytes,
			CreationTime:     creationTime,
			CompletionTime:   item.Status.CompletionTime,
			ContentName:      strings.TrimSpace(item.Status.BoundVolumeSnapshotContentName),
			Error:            errText,
		})
	}

	// Sort newest-first to keep payload stable.
	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].CompletionTime
		b := out[j].CompletionTime
		if a == nil && b == nil {
			return out[i].Name > out[j].Name
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return a.After(*b)
	})

	const maxItems = 200
	if len(out) > maxItems {
		out = out[:maxItems]
	}
	return out, nil
}

func (a *Agent) collectVeleroBackups(ctx context.Context, restClient rest.Interface) ([]agentsk8s.VeleroBackup, error) {
	raw, ok, err := a.doOptionalRawPath(ctx, restClient, "list velero backups", "/apis/velero.io/v1/backups?limit=200")
	if err != nil || !ok || len(raw) == 0 {
		return nil, err
	}

	type vbItem struct {
		Metadata struct {
			UID       string    `json:"uid"`
			Name      string    `json:"name"`
			Namespace string    `json:"namespace"`
			CreatedAt time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		Spec struct {
			StorageLocation string `json:"storageLocation"`
		} `json:"spec"`
		Status struct {
			Phase               string     `json:"phase"`
			StartTimestamp      *time.Time `json:"startTimestamp"`
			CompletionTimestamp *time.Time `json:"completionTimestamp"`
			Expiration          *time.Time `json:"expiration"`
		} `json:"status"`
	}
	type vbList struct {
		Items []vbItem `json:"items"`
	}

	var parsed vbList
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse velero backups: %w", err)
	}

	out := make([]agentsk8s.VeleroBackup, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		name := strings.TrimSpace(item.Metadata.Name)
		if name == "" {
			continue
		}

		namespace := strings.TrimSpace(item.Metadata.Namespace)
		if namespace == "" {
			namespace = "velero"
		}

		out = append(out, agentsk8s.VeleroBackup{
			UID:             strings.TrimSpace(item.Metadata.UID),
			Name:            name,
			Namespace:       namespace,
			Phase:           strings.TrimSpace(item.Status.Phase),
			StartedAt:       item.Status.StartTimestamp,
			CompletedAt:     item.Status.CompletionTimestamp,
			Expiration:      item.Status.Expiration,
			StorageLocation: strings.TrimSpace(item.Spec.StorageLocation),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		a := out[i].CompletedAt
		b := out[j].CompletedAt
		if a == nil && b == nil {
			return out[i].Name > out[j].Name
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return a.After(*b)
	})
	const maxItems = 200
	if len(out) > maxItems {
		out = out[:maxItems]
	}
	return out, nil
}

func (a *Agent) doOptionalRawPath(ctx context.Context, restClient rest.Interface, action, path string) ([]byte, bool, error) {
	if restClient == nil {
		return nil, false, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	payload, err := readKubernetesResponseBody(callCtx, restClient, path, maxRecoveryResponseBodyBytes)
	cancel()
	if err == nil {
		return payload, true, nil
	}

	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		if apierrors.IsForbidden(statusErr) || apierrors.IsNotFound(statusErr) || apierrors.IsUnauthorized(statusErr) {
			return nil, false, nil
		}
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") || strings.Contains(msg, "forbidden") {
		return nil, false, nil
	}

	return nil, false, fmt.Errorf("%s: %w", action, err)
}

func (a *Agent) getDiscoveryRESTClient() rest.Interface {
	if a == nil || a.kubeClient == nil {
		return nil
	}
	discovery := a.kubeClient.Discovery()
	if discovery == nil || discovery.RESTClient() == nil {
		return nil
	}
	return discovery.RESTClient()
}

func (a *Agent) collectUsageMetrics(ctx context.Context, nodes []agentsk8s.Node) (map[string]agentsk8s.NodeUsage, map[string]agentsk8s.PodUsage, error) {
	restClient := a.getDiscoveryRESTClient()
	if restClient == nil {
		return nil, nil, nil
	}

	nodeRaw, nodeErr := readKubernetesResponseBody(ctx, restClient, "/apis/metrics.k8s.io/v1beta1/nodes", maxMetricsResponseBodyBytes)
	podRaw, podErr := readKubernetesResponseBody(ctx, restClient, "/apis/metrics.k8s.io/v1beta1/pods", maxMetricsResponseBodyBytes)

	nodeUsage := map[string]agentsk8s.NodeUsage{}
	if nodeErr == nil {
		parsed, err := parseNodeMetricsPayload(nodeRaw)
		if err != nil {
			a.logger.Debug().
				Err(err).
				Int("payload_bytes", len(nodeRaw)).
				Msg("Failed to parse Kubernetes node metrics payload")
		} else {
			nodeUsage = parsed
		}
	}

	podUsage := map[string]agentsk8s.PodUsage{}
	if podErr == nil {
		parsed, err := parsePodMetricsPayload(podRaw)
		if err != nil {
			a.logger.Debug().
				Err(err).
				Int("payload_bytes", len(podRaw)).
				Msg("Failed to parse Kubernetes pod metrics payload")
		} else {
			podUsage = parsed
		}
	}

	summaryUsage, summaryErr := a.collectPodSummaryMetrics(ctx, nodes)
	if summaryErr != nil {
		a.logger.Debug().
			Err(summaryErr).
			Int("node_count", len(nodes)).
			Msg("Failed to collect Kubernetes pod summary metrics")
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

type resourceUsage struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

func summaryNodeNames(nodes []agentsk8s.Node, max int) ([]string, int) {
	names := make([]string, 0, len(nodes))
	seen := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		nodeName := strings.TrimSpace(node.Name)
		if nodeName == "" {
			continue
		}
		if _, ok := seen[nodeName]; ok {
			continue
		}
		seen[nodeName] = struct{}{}
		names = append(names, nodeName)
	}
	total := len(names)
	if max > 0 && len(names) > max {
		names = names[:max]
	}
	return names, total
}

func (a *Agent) collectPodSummaryMetrics(ctx context.Context, nodes []agentsk8s.Node) (map[string]podSummaryUsage, error) {
	restClient := a.getDiscoveryRESTClient()
	if restClient == nil {
		return nil, nil
	}
	result := make(map[string]podSummaryUsage)
	nodeNames, totalNodeNames := summaryNodeNames(nodes, maxSummaryMetricNodes)
	if totalNodeNames > len(nodeNames) {
		a.logger.Debug().
			Int("total_nodes", totalNodeNames).
			Int("queried_nodes", len(nodeNames)).
			Msg("Limiting pod summary metrics collection to avoid overloading large clusters")
	}

	workerCount := summaryMetricsWorkers
	if workerCount > len(nodeNames) {
		workerCount = len(nodeNames)
	}
	if workerCount == 0 {
		return result, nil
	}

	jobs := make(chan string)
	var wg sync.WaitGroup
	var lock sync.Mutex
	var failed int
	var succeeded int

	collectNode := func(nodeName string) {
		path := "/api/v1/nodes/" + url.PathEscape(nodeName) + "/proxy/stats/summary"
		raw, err := a.doRawPathWithRetry(ctx, restClient, fmt.Sprintf("fetch pod summary metrics from node %q", nodeName), path)
		if err != nil {
			lock.Lock()
			failed++
			lock.Unlock()
			return
		}

		parsed, parseErr := parsePodSummaryMetricsPayload(raw)
		if parseErr != nil {
			lock.Lock()
			failed++
			a.logger.Debug().
				Err(parseErr).
				Str("node", nodeName).
				Int("payload_bytes", len(raw)).
				Msg("Failed to parse Kubernetes pod summary metrics payload")
			lock.Unlock()
			return
		}

		lock.Lock()
		succeeded++
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
		lock.Unlock()
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for nodeName := range jobs {
				if ctx.Err() != nil {
					return
				}
				collectNode(nodeName)
			}
		}()
	}

dispatchLoop:
	for _, nodeName := range nodeNames {
		select {
		case <-ctx.Done():
			break dispatchLoop
		case jobs <- nodeName:
		}
	}
	close(jobs)
	wg.Wait()

	if succeeded == 0 && failed > 0 {
		return nil, fmt.Errorf("no node summary metrics endpoints available")
	}
	if succeeded > 0 && failed > 0 {
		a.logger.Debug().
			Int("nodes_attempted", len(nodes)).
			Int("summary_success_nodes", succeeded).
			Int("summary_failed_nodes", failed).
			Int("pods_with_summary_usage", len(result)).
			Msg("Collected Kubernetes pod summary metrics with partial availability")
	}
	return result, nil
}

func readKubernetesResponseBody(ctx context.Context, restClient rest.Interface, path string, maxBytes int64) ([]byte, error) {
	stream, err := restClient.Get().AbsPath(path).Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	body, err := readBoundedBody(stream, maxBytes)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", path, err)
	}
	return body, nil
}

func readBoundedBody(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid max bytes %d", maxBytes)
	}

	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBytes)
	}
	return body, nil
}

func parseNodeMetricsPayload(raw []byte) (map[string]agentsk8s.NodeUsage, error) {
	var payload struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Usage resourceUsage `json:"usage"`
		} `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal node metrics payload: %w", err)
	}

	result := make(map[string]agentsk8s.NodeUsage, len(payload.Items))
	for _, item := range payload.Items {
		name := strings.TrimSpace(item.Metadata.Name)
		if name == "" {
			continue
		}

		cpuMilli := parseCPUMilli(item.Usage.CPU)
		memBytes := parseBytes(item.Usage.Memory)
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
				Usage resourceUsage `json:"usage"`
			} `json:"containers"`
		} `json:"items"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal pod metrics payload: %w", err)
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
			cpuMilli += parseCPUMilli(container.Usage.CPU)
			memBytes += parseBytes(container.Usage.Memory)
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
		return nil, fmt.Errorf("unmarshal pod summary metrics payload: %w", err)
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

func parseQuantity(value string, convert func(k8sresource.Quantity) int64) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	quantity, err := k8sresource.ParseQuantity(value)
	if err != nil {
		return 0
	}
	return convert(quantity)
}

func parseCPUMilli(value string) int64 {
	return parseQuantity(value, func(q k8sresource.Quantity) int64 { return q.MilliValue() })
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
	return parseQuantity(value, func(q k8sresource.Quantity) int64 { return q.Value() })
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
			nodeUsageCopy := nodeUsage
			nodes[i].Usage = &nodeUsageCopy
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
			podUsageCopy := podUsage
			pods[i].Usage = &podUsageCopy
		}
	}
}

func (a *Agent) collectNodes(ctx context.Context) ([]agentsk8s.Node, error) {
	opts := metav1.ListOptions{Limit: listPageSize}
	nodes := make([]agentsk8s.Node, 0, int(listPageSize))

	for {
		list, err := a.listNodesPage(ctx, opts)
		if err != nil {
			return nil, err
		}

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

		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
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

func (a *Agent) effectiveMaxPods() int {
	if a.cfg.MaxPods > 0 {
		return a.cfg.MaxPods
	}
	return defaultMaxPods
}

func (a *Agent) effectiveMaxDeployments() int {
	return defaultMaxDeployments
}

func explicitNamespaces(patterns []string) ([]string, bool) {
	if len(patterns) == 0 {
		return nil, false
	}

	namespaces := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.ContainsAny(pattern, "*?[]") {
			return nil, false
		}
		namespaces = append(namespaces, pattern)
	}

	namespaces = dedupeStrings(namespaces)
	if len(namespaces) == 0 {
		return nil, false
	}
	sort.Strings(namespaces)
	return namespaces, true
}

func comparePodKey(left, right agentsk8s.Pod) int {
	if left.Namespace < right.Namespace {
		return -1
	}
	if left.Namespace > right.Namespace {
		return 1
	}
	if left.Name < right.Name {
		return -1
	}
	if left.Name > right.Name {
		return 1
	}
	return 0
}

func insertPodSortedBounded(items []agentsk8s.Pod, pod agentsk8s.Pod, max int) []agentsk8s.Pod {
	if max <= 0 {
		return items
	}

	idx := sort.Search(len(items), func(i int) bool {
		return comparePodKey(items[i], pod) >= 0
	})

	if len(items) >= max && idx >= max {
		return items
	}

	if len(items) < max {
		items = append(items, agentsk8s.Pod{})
		copy(items[idx+1:], items[idx:])
		items[idx] = pod
		return items
	}

	copy(items[idx+1:], items[idx:max-1])
	items[idx] = pod
	return items
}

func compareDeploymentKey(left, right agentsk8s.Deployment) int {
	if left.Namespace < right.Namespace {
		return -1
	}
	if left.Namespace > right.Namespace {
		return 1
	}
	if left.Name < right.Name {
		return -1
	}
	if left.Name > right.Name {
		return 1
	}
	return 0
}

func insertDeploymentSortedBounded(items []agentsk8s.Deployment, deployment agentsk8s.Deployment, max int) []agentsk8s.Deployment {
	if max <= 0 {
		return items
	}

	idx := sort.Search(len(items), func(i int) bool {
		return compareDeploymentKey(items[i], deployment) >= 0
	})

	if len(items) >= max && idx >= max {
		return items
	}

	if len(items) < max {
		items = append(items, agentsk8s.Deployment{})
		copy(items[idx+1:], items[idx:])
		items[idx] = deployment
		return items
	}

	copy(items[idx+1:], items[idx:max-1])
	items[idx] = deployment
	return items
}

func isRetryableKubernetesError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) || apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) || apierrors.IsInternalError(err) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "tls handshake timeout") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "server closed idle connection")
}

func retryAfterForError(err error) (time.Duration, bool) {
	var statusErr *apierrors.StatusError
	if !errors.As(err, &statusErr) {
		return 0, false
	}

	details := statusErr.Status().Details
	if details == nil || details.RetryAfterSeconds <= 0 {
		return 0, false
	}
	delay := time.Duration(details.RetryAfterSeconds) * time.Second
	if delay > maxRetryBackoff {
		delay = maxRetryBackoff
	}
	return delay, true
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func wrapKubernetesError(action string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) {
		return fmt.Errorf("%s: kubernetes API timeout or unreachable control plane: %w", action, err)
	}
	if apierrors.IsUnauthorized(err) {
		return fmt.Errorf("%s: kubernetes authentication failed (unauthorized); verify kubeconfig credentials or service account token: %w", action, err)
	}
	if apierrors.IsForbidden(err) {
		return fmt.Errorf("%s: kubernetes access forbidden (RBAC); verify Role/ClusterRole permissions for this agent: %w", action, err)
	}
	if apierrors.IsTooManyRequests(err) {
		return fmt.Errorf("%s: kubernetes API rate limited (429); reduce scope or increase collection interval: %w", action, err)
	}
	if apierrors.IsServiceUnavailable(err) {
		return fmt.Errorf("%s: kubernetes API server unavailable: %w", action, err)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func (a *Agent) runKubernetesCallWithRetry(ctx context.Context, action string, fn func(context.Context) error) error {
	backoff := initialRetryBackoff
	var lastErr error

	for attempt := 1; attempt <= maxKubeAPIRetries; attempt++ {
		if ctx.Err() != nil {
			return wrapKubernetesError(action, ctx.Err())
		}

		callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		err := fn(callCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == maxKubeAPIRetries || !isRetryableKubernetesError(err) {
			return wrapKubernetesError(action, err)
		}

		delay, ok := retryAfterForError(err)
		if !ok {
			delay = backoff
		}
		if delay > maxRetryBackoff {
			delay = maxRetryBackoff
		}

		a.logger.Debug().
			Int("attempt", attempt).
			Dur("backoff", delay).
			Err(err).
			Str("action", action).
			Msg("Kubernetes API call failed; retrying")

		if waitErr := waitForRetry(ctx, delay); waitErr != nil {
			return wrapKubernetesError(action, waitErr)
		}

		backoff *= 2
		if backoff > maxRetryBackoff {
			backoff = maxRetryBackoff
		}
	}

	return wrapKubernetesError(action, lastErr)
}

func (a *Agent) listNodesPage(ctx context.Context, listOpts metav1.ListOptions) (*corev1.NodeList, error) {
	if listOpts.Limit <= 0 {
		listOpts.Limit = listPageSize
	}

	var list *corev1.NodeList
	err := a.runKubernetesCallWithRetry(ctx, "list nodes", func(callCtx context.Context) error {
		var err error
		list, err = a.kubeClient.CoreV1().Nodes().List(callCtx, listOpts)
		return err
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (a *Agent) listPodsPage(ctx context.Context, namespace string, listOpts metav1.ListOptions) (*corev1.PodList, error) {
	if listOpts.Limit <= 0 {
		listOpts.Limit = listPageSize
	}

	action := "list pods"
	if namespace != metav1.NamespaceAll {
		action = fmt.Sprintf("list pods in namespace %q", namespace)
	}

	var list *corev1.PodList
	err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
		var err error
		list, err = a.kubeClient.CoreV1().Pods(namespace).List(callCtx, listOpts)
		return err
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (a *Agent) listDeploymentsPage(ctx context.Context, namespace string, listOpts metav1.ListOptions) (*appsv1.DeploymentList, error) {
	if listOpts.Limit <= 0 {
		listOpts.Limit = listPageSize
	}

	action := "list deployments"
	if namespace != metav1.NamespaceAll {
		action = fmt.Sprintf("list deployments in namespace %q", namespace)
	}

	var list *appsv1.DeploymentList
	err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
		var err error
		list, err = a.kubeClient.AppsV1().Deployments(namespace).List(callCtx, listOpts)
		return err
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (a *Agent) doRawPathWithRetry(ctx context.Context, restClient rest.Interface, action, path string) ([]byte, error) {
	var payload []byte
	err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
		var err error
		payload, err = restClient.Get().AbsPath(path).DoRaw(callCtx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
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
	maxPods := a.effectiveMaxPods()
	listOpts := metav1.ListOptions{
		FieldSelector: selector.String(),
		Limit:         listPageSize,
	}

	namespaces, explicit := explicitNamespaces(a.includeNamespaces)
	if !explicit {
		namespaces = []string{metav1.NamespaceAll}
	}

	items := make([]agentsk8s.Pod, 0, maxPods)
	for _, namespace := range namespaces {
		opts := listOpts
		opts.Continue = ""

		for {
			podList, err := a.listPodsPage(ctx, namespace, opts)
			if err != nil {
				return nil, err
			}

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

				items = insertPodSortedBounded(items, agentsk8s.Pod{
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
				}, maxPods)
			}

			if podList.Continue == "" {
				break
			}
			opts.Continue = podList.Continue
		}
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
	maxDeployments := a.effectiveMaxDeployments()
	listOpts := metav1.ListOptions{Limit: listPageSize}

	namespaces, explicit := explicitNamespaces(a.includeNamespaces)
	if !explicit {
		namespaces = []string{metav1.NamespaceAll}
	}

	items := make([]agentsk8s.Deployment, 0, maxDeployments)
	for _, namespace := range namespaces {
		opts := listOpts
		opts.Continue = ""

		for {
			depList, err := a.listDeploymentsPage(ctx, namespace, opts)
			if err != nil {
				return nil, err
			}

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

				items = insertDeploymentSortedBounded(items, agentsk8s.Deployment{
					UID:                string(dep.UID),
					Name:               dep.Name,
					Namespace:          dep.Namespace,
					CreatedAt:          dep.CreationTimestamp.Time,
					DesiredReplicas:    desiredReplicas(dep),
					UpdatedReplicas:    dep.Status.UpdatedReplicas,
					ReadyReplicas:      dep.Status.ReadyReplicas,
					AvailableReplicas:  dep.Status.AvailableReplicas,
					ObservedGeneration: dep.Status.ObservedGeneration,
					Labels:             labelsCopy,
				}, maxDeployments)
			}

			if depList.Continue == "" {
				break
			}
			opts.Continue = depList.Continue
		}
	}

	return items, nil
}

func (a *Agent) collectReplicaSets(ctx context.Context) ([]agentsk8s.ReplicaSet, error) {
	items := make([]agentsk8s.ReplicaSet, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *appsv1.ReplicaSetList
			action := "list replicasets"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list replicasets in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.AppsV1().ReplicaSets(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, replicaSet := range list.Items {
				if !a.namespaceAllowed(replicaSet.Namespace) {
					continue
				}
				ownerKind, ownerName := ownerRef(replicaSet.OwnerReferences)
				items = append(items, agentsk8s.ReplicaSet{
					UID:                  string(replicaSet.UID),
					Name:                 replicaSet.Name,
					Namespace:            replicaSet.Namespace,
					DesiredReplicas:      replicaSetDesiredReplicas(replicaSet),
					ReadyReplicas:        replicaSet.Status.ReadyReplicas,
					AvailableReplicas:    replicaSet.Status.AvailableReplicas,
					FullyLabeledReplicas: replicaSet.Status.FullyLabeledReplicas,
					ObservedGeneration:   replicaSet.Status.ObservedGeneration,
					OwnerKind:            strings.TrimSpace(ownerKind),
					OwnerName:            strings.TrimSpace(ownerName),
					Labels:               copyKubernetesStringMap(replicaSet.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) inventoryNamespaces() []string {
	namespaces, explicit := explicitNamespaces(a.includeNamespaces)
	if !explicit {
		return []string{metav1.NamespaceAll}
	}
	return namespaces
}

func (a *Agent) collectPartialObjectMetadata(ctx context.Context, resource schema.GroupVersionResource, actionResource string) ([]metav1.PartialObjectMetadata, error) {
	if a.metadataClient == nil {
		return nil, fmt.Errorf("kubernetes metadata client unavailable")
	}

	items := make([]metav1.PartialObjectMetadata, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *metav1.PartialObjectMetadataList
			action := "list " + actionResource + " metadata"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list %s metadata in namespace %q", actionResource, namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.metadataClient.Resource(resource).Namespace(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, item := range list.Items {
				if !a.namespaceAllowed(item.Namespace) {
					continue
				}
				items = append(items, item)
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func copyKubernetesStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func timePtrFromMeta(t metav1.Time) *time.Time {
	if t.Time.IsZero() {
		return nil
	}
	out := t.Time
	return &out
}

func timePtrFromMicro(t metav1.MicroTime) *time.Time {
	if t.Time.IsZero() {
		return nil
	}
	out := t.Time
	return &out
}

func accessModes(modes []corev1.PersistentVolumeAccessMode) []string {
	if len(modes) == 0 {
		return nil
	}
	out := make([]string, 0, len(modes))
	for _, mode := range modes {
		value := strings.TrimSpace(string(mode))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func storageClassNamePtr(name *string) string {
	if name == nil {
		return ""
	}
	return strings.TrimSpace(*name)
}

func boolPtrCopy(value *bool) *bool {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func sortedStringKeys[T any](src map[string]T) []string {
	if len(src) == 0 {
		return nil
	}
	out := make([]string, 0, len(src))
	for key := range src {
		key = strings.TrimSpace(key)
		if key != "" {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func resourceListToStringMap(src corev1.ResourceList) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for name, quantity := range src {
		key := strings.TrimSpace(string(name))
		if key == "" {
			continue
		}
		out[key] = quantity.String()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func limitRangeTypes(limitRange corev1.LimitRange) []string {
	if len(limitRange.Spec.Limits) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(limitRange.Spec.Limits))
	for _, limit := range limitRange.Spec.Limits {
		limitType := strings.TrimSpace(string(limit.Type))
		if limitType != "" {
			seen[limitType] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for limitType := range seen {
		out = append(out, limitType)
	}
	sort.Strings(out)
	return out
}

func hpaMinReplicas(hpa autoscalingv2.HorizontalPodAutoscaler) int32 {
	if hpa.Spec.MinReplicas == nil {
		return 1
	}
	return *hpa.Spec.MinReplicas
}

func hpaMetricTypes(hpa autoscalingv2.HorizontalPodAutoscaler) []string {
	if len(hpa.Spec.Metrics) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(hpa.Spec.Metrics))
	for _, metric := range hpa.Spec.Metrics {
		label := strings.TrimSpace(string(metric.Type))
		switch metric.Type {
		case autoscalingv2.ResourceMetricSourceType:
			if metric.Resource != nil && metric.Resource.Name != "" {
				label = label + ":" + string(metric.Resource.Name)
			}
		case autoscalingv2.ContainerResourceMetricSourceType:
			if metric.ContainerResource != nil && metric.ContainerResource.Name != "" {
				label = label + ":" + string(metric.ContainerResource.Name)
			}
		case autoscalingv2.ExternalMetricSourceType:
			if metric.External != nil && metric.External.Metric.Name != "" {
				label = label + ":" + metric.External.Metric.Name
			}
		case autoscalingv2.PodsMetricSourceType:
			if metric.Pods != nil && metric.Pods.Metric.Name != "" {
				label = label + ":" + metric.Pods.Metric.Name
			}
		case autoscalingv2.ObjectMetricSourceType:
			if metric.Object != nil && metric.Object.Metric.Name != "" {
				label = label + ":" + metric.Object.Metric.Name
			}
		}
		label = strings.TrimSpace(label)
		if label != "" {
			seen[label] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for label := range seen {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

func statefulSetDesiredReplicas(statefulSet appsv1.StatefulSet) int32 {
	if statefulSet.Spec.Replicas == nil {
		return 0
	}
	return *statefulSet.Spec.Replicas
}

func jobDesiredCompletions(job batchv1.Job) int32 {
	if job.Spec.Completions == nil {
		return 0
	}
	return *job.Spec.Completions
}

func cronJobSuspended(cronJob batchv1.CronJob) bool {
	return cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend
}

func replicaSetDesiredReplicas(replicaSet appsv1.ReplicaSet) int32 {
	if replicaSet.Spec.Replicas == nil {
		return 0
	}
	return *replicaSet.Spec.Replicas
}

func ingressClassName(ingress networkingv1.Ingress) string {
	if ingress.Spec.IngressClassName == nil {
		return ""
	}
	return strings.TrimSpace(*ingress.Spec.IngressClassName)
}

func ingressHosts(ingress networkingv1.Ingress) []string {
	hosts := make([]string, 0, len(ingress.Spec.Rules))
	seen := make(map[string]struct{}, len(ingress.Spec.Rules))
	for _, rule := range ingress.Spec.Rules {
		host := strings.TrimSpace(rule.Host)
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	return hosts
}

func ingressAddresses(ingress networkingv1.Ingress) []string {
	addresses := make([]string, 0, len(ingress.Status.LoadBalancer.Ingress))
	seen := make(map[string]struct{}, len(ingress.Status.LoadBalancer.Ingress))
	for _, entry := range ingress.Status.LoadBalancer.Ingress {
		for _, value := range []string{entry.IP, entry.Hostname} {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			addresses = append(addresses, value)
		}
	}
	return addresses
}

func networkPolicyTypes(policy networkingv1.NetworkPolicy) []string {
	if len(policy.Spec.PolicyTypes) == 0 {
		return nil
	}
	out := make([]string, 0, len(policy.Spec.PolicyTypes))
	for _, policyType := range policy.Spec.PolicyTypes {
		value := strings.TrimSpace(string(policyType))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func endpointSliceServiceName(slice discoveryv1.EndpointSlice) string {
	if slice.Labels == nil {
		return ""
	}
	return strings.TrimSpace(slice.Labels[discoveryv1.LabelServiceName])
}

func endpointSlicePorts(slice discoveryv1.EndpointSlice) []agentsk8s.EndpointPort {
	if len(slice.Ports) == 0 {
		return nil
	}
	out := make([]agentsk8s.EndpointPort, 0, len(slice.Ports))
	for _, port := range slice.Ports {
		value := int32(0)
		if port.Port != nil {
			value = *port.Port
		}
		protocol := ""
		if port.Protocol != nil {
			protocol = strings.TrimSpace(string(*port.Protocol))
		}
		out = append(out, agentsk8s.EndpointPort{
			Name:        stringPtrValue(port.Name),
			Protocol:    protocol,
			Port:        value,
			AppProtocol: stringPtrValue(port.AppProtocol),
		})
	}
	return out
}

func endpointSliceReadyCount(slice discoveryv1.EndpointSlice) int {
	ready := 0
	for _, endpoint := range slice.Endpoints {
		if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
			ready++
		}
	}
	return ready
}

func serviceAccountImagePullSecrets(account corev1.ServiceAccount) []string {
	if len(account.ImagePullSecrets) == 0 {
		return nil
	}
	out := make([]string, 0, len(account.ImagePullSecrets))
	for _, ref := range account.ImagePullSecrets {
		name := strings.TrimSpace(ref.Name)
		if name != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (a *Agent) collectNamespaces(ctx context.Context) ([]agentsk8s.Namespace, error) {
	items := make([]agentsk8s.Namespace, 0)
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		var list *corev1.NamespaceList
		err := a.runKubernetesCallWithRetry(ctx, "list namespaces", func(callCtx context.Context) error {
			var err error
			list, err = a.kubeClient.CoreV1().Namespaces().List(callCtx, opts)
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, namespace := range list.Items {
			if !a.namespaceAllowed(namespace.Name) {
				continue
			}
			items = append(items, agentsk8s.Namespace{
				UID:       string(namespace.UID),
				Name:      namespace.Name,
				Phase:     string(namespace.Status.Phase),
				CreatedAt: namespace.CreationTimestamp.Time,
				Labels:    copyKubernetesStringMap(namespace.Labels),
			})
			if len(items) >= maxInventoryItems {
				return items, nil
			}
		}
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	return items, nil
}

func (a *Agent) collectServices(ctx context.Context) ([]agentsk8s.Service, error) {
	items := make([]agentsk8s.Service, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *corev1.ServiceList
			action := "list services"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list services in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.CoreV1().Services(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, service := range list.Items {
				if !a.namespaceAllowed(service.Namespace) {
					continue
				}
				ports := make([]agentsk8s.ServicePort, 0, len(service.Spec.Ports))
				for _, port := range service.Spec.Ports {
					ports = append(ports, agentsk8s.ServicePort{
						Name:       strings.TrimSpace(port.Name),
						Protocol:   string(port.Protocol),
						Port:       port.Port,
						TargetPort: port.TargetPort.String(),
						NodePort:   port.NodePort,
					})
				}
				items = append(items, agentsk8s.Service{
					UID:         string(service.UID),
					Name:        service.Name,
					Namespace:   service.Namespace,
					Type:        string(service.Spec.Type),
					ClusterIP:   service.Spec.ClusterIP,
					ExternalIPs: append([]string(nil), service.Spec.ExternalIPs...),
					Ports:       ports,
					Selector:    copyKubernetesStringMap(service.Spec.Selector),
					CreatedAt:   service.CreationTimestamp.Time,
					Labels:      copyKubernetesStringMap(service.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectStatefulSets(ctx context.Context) ([]agentsk8s.StatefulSet, error) {
	items := make([]agentsk8s.StatefulSet, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *appsv1.StatefulSetList
			action := "list statefulsets"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list statefulsets in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.AppsV1().StatefulSets(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, statefulSet := range list.Items {
				if !a.namespaceAllowed(statefulSet.Namespace) {
					continue
				}
				items = append(items, agentsk8s.StatefulSet{
					UID:               string(statefulSet.UID),
					Name:              statefulSet.Name,
					Namespace:         statefulSet.Namespace,
					DesiredReplicas:   statefulSetDesiredReplicas(statefulSet),
					ReadyReplicas:     statefulSet.Status.ReadyReplicas,
					CurrentReplicas:   statefulSet.Status.CurrentReplicas,
					UpdatedReplicas:   statefulSet.Status.UpdatedReplicas,
					AvailableReplicas: statefulSet.Status.AvailableReplicas,
					ServiceName:       statefulSet.Spec.ServiceName,
					Labels:            copyKubernetesStringMap(statefulSet.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectDaemonSets(ctx context.Context) ([]agentsk8s.DaemonSet, error) {
	items := make([]agentsk8s.DaemonSet, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *appsv1.DaemonSetList
			action := "list daemonsets"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list daemonsets in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.AppsV1().DaemonSets(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, daemonSet := range list.Items {
				if !a.namespaceAllowed(daemonSet.Namespace) {
					continue
				}
				items = append(items, agentsk8s.DaemonSet{
					UID:                    string(daemonSet.UID),
					Name:                   daemonSet.Name,
					Namespace:              daemonSet.Namespace,
					DesiredNumberScheduled: daemonSet.Status.DesiredNumberScheduled,
					CurrentNumberScheduled: daemonSet.Status.CurrentNumberScheduled,
					NumberReady:            daemonSet.Status.NumberReady,
					UpdatedNumberScheduled: daemonSet.Status.UpdatedNumberScheduled,
					NumberAvailable:        daemonSet.Status.NumberAvailable,
					NumberUnavailable:      daemonSet.Status.NumberUnavailable,
					NumberMisscheduled:     daemonSet.Status.NumberMisscheduled,
					Labels:                 copyKubernetesStringMap(daemonSet.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectJobs(ctx context.Context) ([]agentsk8s.Job, error) {
	items := make([]agentsk8s.Job, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *batchv1.JobList
			action := "list jobs"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list jobs in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.BatchV1().Jobs(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, job := range list.Items {
				if !a.namespaceAllowed(job.Namespace) {
					continue
				}
				items = append(items, agentsk8s.Job{
					UID:                string(job.UID),
					Name:               job.Name,
					Namespace:          job.Namespace,
					DesiredCompletions: jobDesiredCompletions(job),
					Succeeded:          job.Status.Succeeded,
					Failed:             job.Status.Failed,
					Active:             job.Status.Active,
					StartTime:          timePtrFromMetaPtr(job.Status.StartTime),
					CompletionTime:     timePtrFromMetaPtr(job.Status.CompletionTime),
					Labels:             copyKubernetesStringMap(job.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectCronJobs(ctx context.Context) ([]agentsk8s.CronJob, error) {
	items := make([]agentsk8s.CronJob, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *batchv1.CronJobList
			action := "list cronjobs"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list cronjobs in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.BatchV1().CronJobs(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, cronJob := range list.Items {
				if !a.namespaceAllowed(cronJob.Namespace) {
					continue
				}
				items = append(items, agentsk8s.CronJob{
					UID:                string(cronJob.UID),
					Name:               cronJob.Name,
					Namespace:          cronJob.Namespace,
					Schedule:           cronJob.Spec.Schedule,
					Suspend:            cronJobSuspended(cronJob),
					Active:             len(cronJob.Status.Active),
					LastScheduleTime:   timePtrFromMetaPtr(cronJob.Status.LastScheduleTime),
					LastSuccessfulTime: timePtrFromMetaPtr(cronJob.Status.LastSuccessfulTime),
					Labels:             copyKubernetesStringMap(cronJob.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectIngresses(ctx context.Context) ([]agentsk8s.Ingress, error) {
	items := make([]agentsk8s.Ingress, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *networkingv1.IngressList
			action := "list ingresses"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list ingresses in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.NetworkingV1().Ingresses(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, ingress := range list.Items {
				if !a.namespaceAllowed(ingress.Namespace) {
					continue
				}
				items = append(items, agentsk8s.Ingress{
					UID:       string(ingress.UID),
					Name:      ingress.Name,
					Namespace: ingress.Namespace,
					ClassName: ingressClassName(ingress),
					Hosts:     ingressHosts(ingress),
					Addresses: ingressAddresses(ingress),
					CreatedAt: ingress.CreationTimestamp.Time,
					Labels:    copyKubernetesStringMap(ingress.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectEndpointSlices(ctx context.Context) ([]agentsk8s.EndpointSlice, error) {
	items := make([]agentsk8s.EndpointSlice, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *discoveryv1.EndpointSliceList
			action := "list endpointslices"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list endpointslices in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.DiscoveryV1().EndpointSlices(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, slice := range list.Items {
				if !a.namespaceAllowed(slice.Namespace) {
					continue
				}
				items = append(items, agentsk8s.EndpointSlice{
					UID:                string(slice.UID),
					Name:               slice.Name,
					Namespace:          slice.Namespace,
					AddressType:        string(slice.AddressType),
					ServiceName:        endpointSliceServiceName(slice),
					Ports:              endpointSlicePorts(slice),
					EndpointCount:      len(slice.Endpoints),
					ReadyEndpointCount: endpointSliceReadyCount(slice),
					CreatedAt:          slice.CreationTimestamp.Time,
					Labels:             copyKubernetesStringMap(slice.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectNetworkPolicies(ctx context.Context) ([]agentsk8s.NetworkPolicy, error) {
	items := make([]agentsk8s.NetworkPolicy, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *networkingv1.NetworkPolicyList
			action := "list network policies"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list network policies in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.NetworkingV1().NetworkPolicies(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, policy := range list.Items {
				if !a.namespaceAllowed(policy.Namespace) {
					continue
				}
				items = append(items, agentsk8s.NetworkPolicy{
					UID:              string(policy.UID),
					Name:             policy.Name,
					Namespace:        policy.Namespace,
					PolicyTypes:      networkPolicyTypes(policy),
					IngressRuleCount: len(policy.Spec.Ingress),
					EgressRuleCount:  len(policy.Spec.Egress),
					CreatedAt:        policy.CreationTimestamp.Time,
					Labels:           copyKubernetesStringMap(policy.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectPersistentVolumes(ctx context.Context) ([]agentsk8s.PersistentVolume, error) {
	items := make([]agentsk8s.PersistentVolume, 0)
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		var list *corev1.PersistentVolumeList
		err := a.runKubernetesCallWithRetry(ctx, "list persistent volumes", func(callCtx context.Context) error {
			var err error
			list, err = a.kubeClient.CoreV1().PersistentVolumes().List(callCtx, opts)
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, volume := range list.Items {
			claimNamespace := ""
			claimName := ""
			if volume.Spec.ClaimRef != nil {
				claimNamespace = volume.Spec.ClaimRef.Namespace
				claimName = volume.Spec.ClaimRef.Name
			}
			if claimNamespace != "" && !a.namespaceAllowed(claimNamespace) {
				continue
			}
			items = append(items, agentsk8s.PersistentVolume{
				UID:            string(volume.UID),
				Name:           volume.Name,
				Phase:          string(volume.Status.Phase),
				StorageClass:   volume.Spec.StorageClassName,
				CapacityBytes:  volume.Spec.Capacity.Storage().Value(),
				AccessModes:    accessModes(volume.Spec.AccessModes),
				ReclaimPolicy:  string(volume.Spec.PersistentVolumeReclaimPolicy),
				ClaimNamespace: claimNamespace,
				ClaimName:      claimName,
				CreatedAt:      volume.CreationTimestamp.Time,
				Labels:         copyKubernetesStringMap(volume.Labels),
			})
			if len(items) >= maxInventoryItems {
				return items, nil
			}
		}
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	return items, nil
}

func (a *Agent) collectPersistentVolumeClaims(ctx context.Context) ([]agentsk8s.PersistentVolumeClaim, error) {
	items := make([]agentsk8s.PersistentVolumeClaim, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *corev1.PersistentVolumeClaimList
			action := "list persistent volume claims"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list persistent volume claims in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, claim := range list.Items {
				if !a.namespaceAllowed(claim.Namespace) {
					continue
				}
				requested := int64(0)
				if storageRequest, ok := claim.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
					requested = storageRequest.Value()
				}
				items = append(items, agentsk8s.PersistentVolumeClaim{
					UID:            string(claim.UID),
					Name:           claim.Name,
					Namespace:      claim.Namespace,
					Phase:          string(claim.Status.Phase),
					StorageClass:   storageClassNamePtr(claim.Spec.StorageClassName),
					RequestedBytes: requested,
					CapacityBytes:  claim.Status.Capacity.Storage().Value(),
					AccessModes:    accessModes(claim.Spec.AccessModes),
					VolumeName:     claim.Spec.VolumeName,
					CreatedAt:      claim.CreationTimestamp.Time,
					Labels:         copyKubernetesStringMap(claim.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectStorageClasses(ctx context.Context) ([]agentsk8s.StorageClass, error) {
	items := make([]agentsk8s.StorageClass, 0)
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		var list *storagev1.StorageClassList
		err := a.runKubernetesCallWithRetry(ctx, "list storage classes", func(callCtx context.Context) error {
			var err error
			list, err = a.kubeClient.StorageV1().StorageClasses().List(callCtx, opts)
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, class := range list.Items {
			reclaimPolicy := ""
			if class.ReclaimPolicy != nil {
				reclaimPolicy = strings.TrimSpace(string(*class.ReclaimPolicy))
			}
			volumeBindingMode := ""
			if class.VolumeBindingMode != nil {
				volumeBindingMode = strings.TrimSpace(string(*class.VolumeBindingMode))
			}
			items = append(items, agentsk8s.StorageClass{
				UID:                  string(class.UID),
				Name:                 class.Name,
				Provisioner:          strings.TrimSpace(class.Provisioner),
				ReclaimPolicy:        reclaimPolicy,
				VolumeBindingMode:    volumeBindingMode,
				AllowVolumeExpansion: boolPtrCopy(class.AllowVolumeExpansion),
				ParameterKeys:        sortedStringKeys(class.Parameters),
				CreatedAt:            class.CreationTimestamp.Time,
				Labels:               copyKubernetesStringMap(class.Labels),
			})
			if len(items) >= maxInventoryItems {
				return items, nil
			}
		}
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	return items, nil
}

func (a *Agent) collectConfigMaps(ctx context.Context) ([]agentsk8s.ConfigMap, error) {
	metadataItems, err := a.collectPartialObjectMetadata(ctx, configMapsGVR, "configmaps")
	if err != nil {
		return nil, err
	}
	items := make([]agentsk8s.ConfigMap, 0, len(metadataItems))
	for _, configMap := range metadataItems {
		items = append(items, agentsk8s.ConfigMap{
			UID:          string(configMap.UID),
			Name:         configMap.Name,
			Namespace:    configMap.Namespace,
			MetadataOnly: true,
			CreatedAt:    configMap.CreationTimestamp.Time,
			Labels:       copyKubernetesStringMap(configMap.Labels),
		})
	}
	return items, nil
}

func (a *Agent) collectServiceAccounts(ctx context.Context) ([]agentsk8s.ServiceAccount, error) {
	items := make([]agentsk8s.ServiceAccount, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *corev1.ServiceAccountList
			action := "list serviceaccounts"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list serviceaccounts in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.CoreV1().ServiceAccounts(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, account := range list.Items {
				if !a.namespaceAllowed(account.Namespace) {
					continue
				}
				items = append(items, agentsk8s.ServiceAccount{
					UID:                          string(account.UID),
					Name:                         account.Name,
					Namespace:                    account.Namespace,
					AutomountServiceAccountToken: boolPtrCopy(account.AutomountServiceAccountToken),
					SecretCount:                  len(account.Secrets),
					ImagePullSecrets:             serviceAccountImagePullSecrets(account),
					CreatedAt:                    account.CreationTimestamp.Time,
					Labels:                       copyKubernetesStringMap(account.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

// rbacSubjectKinds returns the distinct subject kinds bound by a Role
// or ClusterRole binding, sorted for stable reporting. Names are
// deliberately not reported so Pulse doesn't become an RBAC enumeration
// surface for individual users / groups / service accounts.
func rbacSubjectKinds(subjects []rbacv1.Subject) []string {
	if len(subjects) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(subjects))
	kinds := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		kind := strings.TrimSpace(subject.Kind)
		if kind == "" {
			continue
		}
		if _, exists := seen[kind]; exists {
			continue
		}
		seen[kind] = struct{}{}
		kinds = append(kinds, kind)
	}
	if len(kinds) == 0 {
		return nil
	}
	sort.Strings(kinds)
	return kinds
}

func (a *Agent) collectRoles(ctx context.Context) ([]agentsk8s.Role, error) {
	items := make([]agentsk8s.Role, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *rbacv1.RoleList
			action := "list roles"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list roles in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.RbacV1().Roles(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, role := range list.Items {
				if !a.namespaceAllowed(role.Namespace) {
					continue
				}
				items = append(items, agentsk8s.Role{
					UID:       string(role.UID),
					Name:      role.Name,
					Namespace: role.Namespace,
					RuleCount: len(role.Rules),
					CreatedAt: role.CreationTimestamp.Time,
					Labels:    copyKubernetesStringMap(role.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectClusterRoles(ctx context.Context) ([]agentsk8s.ClusterRole, error) {
	items := make([]agentsk8s.ClusterRole, 0)
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		var list *rbacv1.ClusterRoleList
		err := a.runKubernetesCallWithRetry(ctx, "list clusterroles", func(callCtx context.Context) error {
			var err error
			list, err = a.kubeClient.RbacV1().ClusterRoles().List(callCtx, opts)
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, role := range list.Items {
			var aggregationLabels map[string]string
			if role.AggregationRule != nil && len(role.AggregationRule.ClusterRoleSelectors) > 0 {
				aggregationLabels = make(map[string]string)
				for _, selector := range role.AggregationRule.ClusterRoleSelectors {
					for k, v := range selector.MatchLabels {
						aggregationLabels[k] = v
					}
				}
				if len(aggregationLabels) == 0 {
					aggregationLabels = nil
				}
			}
			items = append(items, agentsk8s.ClusterRole{
				UID:               string(role.UID),
				Name:              role.Name,
				RuleCount:         len(role.Rules),
				AggregationLabels: aggregationLabels,
				CreatedAt:         role.CreationTimestamp.Time,
				Labels:            copyKubernetesStringMap(role.Labels),
			})
			if len(items) >= maxInventoryItems {
				return items, nil
			}
		}
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	return items, nil
}

func (a *Agent) collectRoleBindings(ctx context.Context) ([]agentsk8s.RoleBinding, error) {
	items := make([]agentsk8s.RoleBinding, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *rbacv1.RoleBindingList
			action := "list rolebindings"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list rolebindings in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.RbacV1().RoleBindings(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, binding := range list.Items {
				if !a.namespaceAllowed(binding.Namespace) {
					continue
				}
				items = append(items, agentsk8s.RoleBinding{
					UID:          string(binding.UID),
					Name:         binding.Name,
					Namespace:    binding.Namespace,
					RoleKind:     binding.RoleRef.Kind,
					RoleName:     binding.RoleRef.Name,
					SubjectCount: len(binding.Subjects),
					SubjectKinds: rbacSubjectKinds(binding.Subjects),
					CreatedAt:    binding.CreationTimestamp.Time,
					Labels:       copyKubernetesStringMap(binding.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectClusterRoleBindings(ctx context.Context) ([]agentsk8s.ClusterRoleBinding, error) {
	items := make([]agentsk8s.ClusterRoleBinding, 0)
	opts := metav1.ListOptions{Limit: listPageSize}
	for {
		var list *rbacv1.ClusterRoleBindingList
		err := a.runKubernetesCallWithRetry(ctx, "list clusterrolebindings", func(callCtx context.Context) error {
			var err error
			list, err = a.kubeClient.RbacV1().ClusterRoleBindings().List(callCtx, opts)
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, binding := range list.Items {
			items = append(items, agentsk8s.ClusterRoleBinding{
				UID:          string(binding.UID),
				Name:         binding.Name,
				RoleKind:     binding.RoleRef.Kind,
				RoleName:     binding.RoleRef.Name,
				SubjectCount: len(binding.Subjects),
				SubjectKinds: rbacSubjectKinds(binding.Subjects),
				CreatedAt:    binding.CreationTimestamp.Time,
				Labels:       copyKubernetesStringMap(binding.Labels),
			})
			if len(items) >= maxInventoryItems {
				return items, nil
			}
		}
		if list.Continue == "" {
			break
		}
		opts.Continue = list.Continue
	}
	return items, nil
}

func (a *Agent) collectSecrets(ctx context.Context) ([]agentsk8s.Secret, error) {
	metadataItems, err := a.collectPartialObjectMetadata(ctx, secretsGVR, "secrets")
	if err != nil {
		return nil, err
	}
	items := make([]agentsk8s.Secret, 0, len(metadataItems))
	for _, secret := range metadataItems {
		items = append(items, agentsk8s.Secret{
			UID:          string(secret.UID),
			Name:         secret.Name,
			Namespace:    secret.Namespace,
			MetadataOnly: true,
			CreatedAt:    secret.CreationTimestamp.Time,
			Labels:       copyKubernetesStringMap(secret.Labels),
		})
	}
	return items, nil
}

func (a *Agent) collectResourceQuotas(ctx context.Context) ([]agentsk8s.ResourceQuota, error) {
	items := make([]agentsk8s.ResourceQuota, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *corev1.ResourceQuotaList
			action := "list resourcequotas"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list resourcequotas in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.CoreV1().ResourceQuotas(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, quota := range list.Items {
				if !a.namespaceAllowed(quota.Namespace) {
					continue
				}
				hard := resourceListToStringMap(quota.Status.Hard)
				if len(hard) == 0 {
					hard = resourceListToStringMap(quota.Spec.Hard)
				}
				items = append(items, agentsk8s.ResourceQuota{
					UID:       string(quota.UID),
					Name:      quota.Name,
					Namespace: quota.Namespace,
					Hard:      hard,
					Used:      resourceListToStringMap(quota.Status.Used),
					CreatedAt: quota.CreationTimestamp.Time,
					Labels:    copyKubernetesStringMap(quota.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectLimitRanges(ctx context.Context) ([]agentsk8s.LimitRange, error) {
	items := make([]agentsk8s.LimitRange, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *corev1.LimitRangeList
			action := "list limitranges"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list limitranges in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.CoreV1().LimitRanges(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, limitRange := range list.Items {
				if !a.namespaceAllowed(limitRange.Namespace) {
					continue
				}
				items = append(items, agentsk8s.LimitRange{
					UID:        string(limitRange.UID),
					Name:       limitRange.Name,
					Namespace:  limitRange.Namespace,
					LimitTypes: limitRangeTypes(limitRange),
					CreatedAt:  limitRange.CreationTimestamp.Time,
					Labels:     copyKubernetesStringMap(limitRange.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectPodDisruptionBudgets(ctx context.Context) ([]agentsk8s.PodDisruptionBudget, error) {
	items := make([]agentsk8s.PodDisruptionBudget, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *policyv1.PodDisruptionBudgetList
			action := "list poddisruptionbudgets"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list poddisruptionbudgets in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, budget := range list.Items {
				if !a.namespaceAllowed(budget.Namespace) {
					continue
				}
				minAvailable := ""
				if budget.Spec.MinAvailable != nil {
					minAvailable = strings.TrimSpace(budget.Spec.MinAvailable.String())
				}
				maxUnavailable := ""
				if budget.Spec.MaxUnavailable != nil {
					maxUnavailable = strings.TrimSpace(budget.Spec.MaxUnavailable.String())
				}
				items = append(items, agentsk8s.PodDisruptionBudget{
					UID:                string(budget.UID),
					Name:               budget.Name,
					Namespace:          budget.Namespace,
					MinAvailable:       minAvailable,
					MaxUnavailable:     maxUnavailable,
					DesiredHealthy:     budget.Status.DesiredHealthy,
					CurrentHealthy:     budget.Status.CurrentHealthy,
					DisruptionsAllowed: budget.Status.DisruptionsAllowed,
					ExpectedPods:       budget.Status.ExpectedPods,
					CreatedAt:          budget.CreationTimestamp.Time,
					Labels:             copyKubernetesStringMap(budget.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectHorizontalPodAutoscalers(ctx context.Context) ([]agentsk8s.HorizontalPodAutoscaler, error) {
	items := make([]agentsk8s.HorizontalPodAutoscaler, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *autoscalingv2.HorizontalPodAutoscalerList
			action := "list horizontalpodautoscalers"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list horizontalpodautoscalers in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, hpa := range list.Items {
				if !a.namespaceAllowed(hpa.Namespace) {
					continue
				}
				items = append(items, agentsk8s.HorizontalPodAutoscaler{
					UID:             string(hpa.UID),
					Name:            hpa.Name,
					Namespace:       hpa.Namespace,
					TargetKind:      hpa.Spec.ScaleTargetRef.Kind,
					TargetName:      hpa.Spec.ScaleTargetRef.Name,
					MinReplicas:     hpaMinReplicas(hpa),
					MaxReplicas:     hpa.Spec.MaxReplicas,
					CurrentReplicas: hpa.Status.CurrentReplicas,
					DesiredReplicas: hpa.Status.DesiredReplicas,
					MetricTypes:     hpaMetricTypes(hpa),
					CreatedAt:       hpa.CreationTimestamp.Time,
					Labels:          copyKubernetesStringMap(hpa.Labels),
				})
				if len(items) >= maxInventoryItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func (a *Agent) collectEvents(ctx context.Context) ([]agentsk8s.Event, error) {
	items := make([]agentsk8s.Event, 0)
	for _, namespace := range a.inventoryNamespaces() {
		opts := metav1.ListOptions{Limit: listPageSize}
		for {
			var list *corev1.EventList
			action := "list events"
			if namespace != metav1.NamespaceAll {
				action = fmt.Sprintf("list events in namespace %q", namespace)
			}
			err := a.runKubernetesCallWithRetry(ctx, action, func(callCtx context.Context) error {
				var err error
				list, err = a.kubeClient.CoreV1().Events(namespace).List(callCtx, opts)
				return err
			})
			if err != nil {
				return nil, err
			}
			for _, event := range list.Items {
				if event.Namespace != "" && !a.namespaceAllowed(event.Namespace) {
					continue
				}
				items = append(items, agentsk8s.Event{
					UID:          string(event.UID),
					Name:         event.Name,
					Namespace:    event.Namespace,
					Type:         event.Type,
					Reason:       event.Reason,
					Message:      event.Message,
					InvolvedKind: event.InvolvedObject.Kind,
					InvolvedName: event.InvolvedObject.Name,
					Count:        event.Count,
					FirstSeen:    timePtrFromMeta(event.FirstTimestamp),
					LastSeen:     timePtrFromMeta(event.LastTimestamp),
					EventTime:    timePtrFromMicro(event.EventTime),
				})
				if len(items) >= maxEventItems {
					return items, nil
				}
			}
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return items, nil
}

func timePtrFromMetaPtr(t *metav1.Time) *time.Time {
	if t == nil {
		return nil
	}
	return timePtrFromMeta(*t)
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

func (a *Agent) sendReport(ctx context.Context, report agentsk8s.Report) (retErr error) {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	compressed, err := utils.CompressJSON(payload)
	if err != nil {
		return fmt.Errorf("compress report: %w", err)
	}

	reportURL := fmt.Sprintf("%s/api/agents/kubernetes/report", a.pulseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reportURL, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("create request for %s: %w", reportURL, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIToken)
	req.Header.Set("X-API-Token", a.cfg.APIToken)
	req.Header.Set("User-Agent", reportUserAgent+a.agentVersion)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request to %s: %w", reportURL, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close response body: %w", closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}
			retErr = wrappedCloseErr
		}
	}()

	if resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		if readErr != nil {
			return fmt.Errorf("read error response body for status %s: %w", resp.Status, readErr)
		}
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("pulse responded with status %s for %s: %s", resp.Status, reportURL, msg)
		}
		return fmt.Errorf("pulse responded with status %s for %s", resp.Status, reportURL)
	}

	return nil
}
