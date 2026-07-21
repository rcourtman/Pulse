package unifiedresources

import (
	"testing"
)

// TestBranchcov0721amRegistryK8sSeed mechanically exercises every branch of the
// five pure unexported helpers in registry.go that the k8s seeding path relies
// on. Each subtest asserts exact expected values computed by hand from the
// source logic; it is intentionally exhaustive about nil/blank/whitespace
// guards so the branch coverage of these helpers goes to 100%. This file does
// not modify any source or pre-existing test.
func TestBranchcov0721amRegistryK8sSeed(t *testing.T) {
	t.Run("seededKubernetesClusterSourceID", func(t *testing.T) {
		cases := []struct {
			name string
			in   *K8sData
			want string
		}{
			{name: "nil receiver", in: nil, want: ""},
			{name: "all zero value", in: &K8sData{}, want: ""},
			{name: "all whitespace only", in: &K8sData{
				ClusterID:   "   ",
				AgentID:     "\t",
				SourceName:  " \n ",
				ClusterName: " ",
				Context:     " ",
			}, want: ""},
			{name: "ClusterID alone wins", in: &K8sData{ClusterID: "cid-1"}, want: "cid-1"},
			{name: "ClusterID wins over all later set", in: &K8sData{
				ClusterID: "cid-first", AgentID: "ag", SourceName: "sn",
				ClusterName: "cn", Context: "ctx",
			}, want: "cid-first"},
			{name: "AgentID wins when ClusterID blank", in: &K8sData{
				AgentID: "  ag-1  ", ClusterName: "cn",
			}, want: "ag-1"},
			{name: "SourceName wins when ClusterID+AgentID blank", in: &K8sData{
				ClusterID: "  ", AgentID: "", SourceName: "sn-1",
			}, want: "sn-1"},
			{name: "ClusterName wins when earlier blank", in: &K8sData{
				ClusterID: "", AgentID: " \t ", SourceName: "", ClusterName: "cn-1",
			}, want: "cn-1"},
			{name: "Context wins when all earlier blank", in: &K8sData{
				ClusterName: "  ", Context: "ctx-1",
			}, want: "ctx-1"},
			{name: "trims surrounding whitespace from ClusterID", in: &K8sData{
				ClusterID: "  cid-trim  ",
			}, want: "cid-trim"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := seededKubernetesClusterSourceID(tc.in); got != tc.want {
					t.Fatalf("seededKubernetesClusterSourceID(%+v): want %q, got %q", tc.in, tc.want, got)
				}
			})
		}
	})

	t.Run("seededKubernetesNodeIdentity", func(t *testing.T) {
		cases := []struct {
			name string
			in   *K8sData
			want string
		}{
			{name: "nil receiver", in: nil, want: ""},
			{name: "NodeUID present wins over NodeName", in: &K8sData{
				NodeUID: "uid-node-1", NodeName: "nm",
			}, want: "uid-node-1"},
			{name: "NodeUID whitespace falls back to NodeName", in: &K8sData{
				NodeUID: "  ", NodeName: "nm-1",
			}, want: "nm-1"},
			{name: "NodeUID blank trims NodeName", in: &K8sData{
				NodeName: "  nm-trim  ",
			}, want: "nm-trim"},
			{name: "both blank", in: &K8sData{}, want: ""},
			{name: "both whitespace only", in: &K8sData{
				NodeUID: "\t", NodeName: "  ",
			}, want: ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := seededKubernetesNodeIdentity(tc.in); got != tc.want {
					t.Fatalf("seededKubernetesNodeIdentity(%+v): want %q, got %q", tc.in, tc.want, got)
				}
			})
		}
	})

	t.Run("seededKubernetesPodIdentity", func(t *testing.T) {
		cases := []struct {
			name string
			res  *Resource
			want string
		}{
			{name: "nil resource", res: nil, want: ""},
			{name: "nil Kubernetes", res: &Resource{Name: "n"}, want: ""},
			{name: "PodUID present wins", res: &Resource{
				Name: "pod", Kubernetes: &K8sData{PodUID: "  pod-uid-1  ", Namespace: "ns"},
			}, want: "pod-uid-1"},
			{name: "PodUID blank -> namespace/name", res: &Resource{
				Name: "  pod-nm  ", Kubernetes: &K8sData{Namespace: "  ns-1  "},
			}, want: "ns-1/pod-nm"},
			{name: "PodUID blank, name blank, ns set -> empty", res: &Resource{
				Kubernetes: &K8sData{Namespace: "ns-1"},
			}, want: ""},
			{name: "PodUID blank, ns blank, name set -> empty", res: &Resource{
				Name: "pod-nm", Kubernetes: &K8sData{},
			}, want: ""},
			{name: "PodUID blank, both blank -> empty", res: &Resource{
				Kubernetes: &K8sData{},
			}, want: ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := seededKubernetesPodIdentity(tc.res); got != tc.want {
					t.Fatalf("seededKubernetesPodIdentity: want %q, got %q", tc.want, got)
				}
			})
		}
	})

	t.Run("seededKubernetesDeploymentIdentity", func(t *testing.T) {
		cases := []struct {
			name string
			res  *Resource
			want string
		}{
			{name: "nil resource", res: nil, want: ""},
			{name: "nil Kubernetes", res: &Resource{Name: "n"}, want: ""},
			{name: "DeploymentUID present wins", res: &Resource{
				Name: "dep", Kubernetes: &K8sData{DeploymentUID: "  dep-uid-1  ", Namespace: "ns"},
			}, want: "dep-uid-1"},
			{name: "DeploymentUID blank -> namespace/name", res: &Resource{
				Name: "  dep-nm  ", Kubernetes: &K8sData{Namespace: "  ns-1  "},
			}, want: "ns-1/dep-nm"},
			{name: "DeploymentUID blank, name blank, ns set -> empty", res: &Resource{
				Kubernetes: &K8sData{Namespace: "ns-1"},
			}, want: ""},
			{name: "DeploymentUID blank, ns blank, name set -> empty", res: &Resource{
				Name: "dep-nm", Kubernetes: &K8sData{},
			}, want: ""},
			{name: "DeploymentUID blank, both blank -> empty", res: &Resource{
				Kubernetes: &K8sData{},
			}, want: ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := seededKubernetesDeploymentIdentity(tc.res); got != tc.want {
					t.Fatalf("seededKubernetesDeploymentIdentity: want %q, got %q", tc.want, got)
				}
			})
		}
	})

	t.Run("seededKubernetesTypedSourceID", func(t *testing.T) {
		cases := []struct {
			name            string
			clusterSourceID string
			kind            string
			res             *Resource
			uid             string
			want            string
		}{
			{name: "nil resource", clusterSourceID: "c", kind: "k", res: nil, uid: "u", want: ""},
			{name: "nil Kubernetes", clusterSourceID: "c", kind: "k", res: &Resource{Name: "n"}, uid: "u", want: ""},
			{name: "uid provided wins (trimmed)", clusterSourceID: "c-1", kind: "k8s-pod", res: &Resource{
				Name: "n", Kubernetes: &K8sData{Namespace: "ns"},
			}, uid: "  uid-1  ", want: "c-1:k8s-pod:uid-1"},
			{name: "uid blank -> ns/name id", clusterSourceID: "c-1", kind: "k8s-pod", res: &Resource{
				Name: "  nm  ", Kubernetes: &K8sData{Namespace: "  ns  "},
			}, uid: "", want: "c-1:k8s-pod:ns/nm"},
			{name: "uid blank + ns blank -> name-only id", clusterSourceID: "c-1", kind: "k8s-pod", res: &Resource{
				Name: "  nm  ", Kubernetes: &K8sData{},
			}, uid: "  ", want: "c-1:k8s-pod:nm"},
			{name: "uid blank + both ns/name blank -> empty id guard", clusterSourceID: "c-1", kind: "k8s-pod", res: &Resource{
				Kubernetes: &K8sData{},
			}, uid: "", want: ""},
			{name: "uid blank + ns set + name blank -> empty id guard", clusterSourceID: "c-1", kind: "k8s-pod", res: &Resource{
				Kubernetes: &K8sData{Namespace: "ns"},
			}, uid: "", want: ""},
			{name: "empty clusterSourceID guard", clusterSourceID: "  ", kind: "k", res: &Resource{
				Name: "n", Kubernetes: &K8sData{},
			}, uid: "u", want: ""},
			{name: "empty kind guard", clusterSourceID: "c", kind: "", res: &Resource{
				Name: "n", Kubernetes: &K8sData{},
			}, uid: "u", want: ""},
			{name: "clusterSourceID and kind trimmed on success", clusterSourceID: "  cluster-a  ", kind: "  k8s-node  ", res: &Resource{
				Name: "ignored", Kubernetes: &K8sData{Namespace: "ignored-too"},
			}, uid: "uid-final", want: "cluster-a:k8s-node:uid-final"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := seededKubernetesTypedSourceID(tc.clusterSourceID, tc.kind, tc.res, tc.uid); got != tc.want {
					t.Fatalf("seededKubernetesTypedSourceID(%q,%q,%+v,%q): want %q, got %q",
						tc.clusterSourceID, tc.kind, tc.res, tc.uid, tc.want, got)
				}
			})
		}
	})
}
