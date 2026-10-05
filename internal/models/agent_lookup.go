package models

// GetAgentLookupSnapshot is only for the producer's temporary host-identity
// lookup registry, never for publication, alerting or a complete inventory.
// Keep the ordinary snapshot's lock, ownership and all other source fields;
// omit only these known Kubernetes metadata kinds before deep copying them.
// They cannot merge into a host. Nodes and workloads still contribute the
// same identity, source selection, relationships and freshness evidence.
func (s *State) GetAgentLookupSnapshot() StateSnapshot {
	return s.getSnapshot(cloneKubernetesClustersForAgentLookup)
}

func cloneKubernetesClustersForAgentLookup(src []KubernetesCluster) []KubernetesCluster {
	if len(src) == 0 {
		return nil
	}
	dest := make([]KubernetesCluster, len(src))
	for i, cluster := range src {
		// The struct is a value copy. Do not normalize or mutate its remaining
		// slice members until cloneKubernetesCluster has detached them.
		cluster.Namespaces = nil
		cluster.Services = nil
		cluster.Ingresses = nil
		cluster.EndpointSlices = nil
		cluster.NetworkPolicies = nil
		cluster.PersistentVolumes = nil
		cluster.PersistentVolumeClaims = nil
		cluster.StorageClasses = nil
		cluster.ConfigMaps = nil
		cluster.Secrets = nil
		cluster.ServiceAccounts = nil
		cluster.Roles = nil
		cluster.ClusterRoles = nil
		cluster.RoleBindings = nil
		cluster.ClusterRoleBindings = nil
		cluster.ResourceQuotas = nil
		cluster.LimitRanges = nil
		cluster.PodDisruptionBudgets = nil
		cluster.HorizontalPodAutoscalers = nil
		cluster.Events = nil
		dest[i] = cloneKubernetesCluster(cluster)
	}
	return dest
}
