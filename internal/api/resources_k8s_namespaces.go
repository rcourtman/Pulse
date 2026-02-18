package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type k8sNamespaceCounts struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Warning int `json:"warning"`
	Offline int `json:"offline"`
	Unknown int `json:"unknown"`
}

type k8sNamespaceSummary struct {
	Namespace   string             `json:"namespace"`
	Pods        k8sNamespaceCounts `json:"pods"`
	Deployments k8sNamespaceCounts `json:"deployments"`
}

type k8sNamespacesResponse struct {
	Cluster string                `json:"cluster"`
	Data    []k8sNamespaceSummary `json:"data"`
}

// HandleK8sNamespaces handles GET /api/resources/k8s/namespaces?cluster=<clusterName>
// and returns namespace-level counts for Pods and Deployments.
func (h *ResourceHandlers) HandleK8sNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cluster := strings.TrimSpace(r.URL.Query().Get("cluster"))
	if cluster == "" {
		http.Error(w, "cluster query param required", http.StatusBadRequest)
		return
	}

	orgID := GetOrgID(r.Context())
	registry, err := h.buildRegistry(orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	normalize := func(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
	wantCluster := normalize(cluster)

	type bucket struct {
		pods        k8sNamespaceCounts
		deployments k8sNamespaceCounts
	}
	byNamespace := make(map[string]*bucket)

	addCount := func(counts *k8sNamespaceCounts, status unified.ResourceStatus) {
		counts.Total++
		switch status {
		case unified.StatusOnline:
			counts.Online++
		case unified.StatusWarning:
			counts.Warning++
		case unified.StatusOffline:
			counts.Offline++
		default:
			counts.Unknown++
		}
	}

	ensureBucket := func(namespace string) *bucket {
		b := byNamespace[namespace]
		if b == nil {
			b = &bucket{}
			byNamespace[namespace] = b
		}
		return b
	}

	ingest := func(resource unified.Resource, isDeployment bool) {
		if resource.Kubernetes == nil {
			return
		}
		if normalize(resource.Kubernetes.ClusterName) != wantCluster {
			return
		}
		ns := strings.TrimSpace(resource.Kubernetes.Namespace)
		if ns == "" {
			ns = "default"
		}
		b := ensureBucket(ns)
		if isDeployment {
			addCount(&b.deployments, resource.Status)
		} else {
			addCount(&b.pods, resource.Status)
		}
	}

	for _, pod := range registry.ListByType(unified.ResourceTypePod) {
		ingest(pod, false)
	}
	for _, dep := range registry.ListByType(unified.ResourceTypeK8sDeployment) {
		ingest(dep, true)
	}

	namespaces := make([]string, 0, len(byNamespace))
	for ns := range byNamespace {
		namespaces = append(namespaces, ns)
	}
	sort.Slice(namespaces, func(i, j int) bool { return namespaces[i] < namespaces[j] })

	out := make([]k8sNamespaceSummary, 0, len(namespaces))
	for _, ns := range namespaces {
		b := byNamespace[ns]
		if b == nil {
			continue
		}
		out = append(out, k8sNamespaceSummary{
			Namespace:   ns,
			Pods:        b.pods,
			Deployments: b.deployments,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(k8sNamespacesResponse{
		Cluster: cluster,
		Data:    out,
	})
}
