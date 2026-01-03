import type { DockerHost, DockerService } from '@/types/api';

/**
 * Represents a Docker Swarm cluster with its member hosts
 */
export interface SwarmCluster {
  clusterId: string;
  clusterName: string;
  hosts: DockerHost[];
}

/**
 * Represents an aggregated service across a Swarm cluster
 */
export interface ClusterService {
  service: DockerService;
  clusterId: string;
  clusterName: string;
  nodes: Array<{
    hostId: string;
    hostname: string;
    taskCount: number;
    runningCount: number;
  }>;
  totalDesired: number;
  totalRunning: number;
}

/**
 * Groups Docker hosts by their Swarm cluster ID.
 * Hosts without a clusterId are returned as individual "clusters" with empty clusterId.
 */
export function groupHostsByCluster(hosts: DockerHost[]): SwarmCluster[] {
  const clusterMap = new Map<string, SwarmCluster>();
  const standaloneHosts: DockerHost[] = [];

  for (const host of hosts) {
    const clusterId = host.swarm?.clusterId;

    if (!clusterId) {
      // Non-Swarm host or no cluster info
      standaloneHosts.push(host);
      continue;
    }

    if (!clusterMap.has(clusterId)) {
      clusterMap.set(clusterId, {
        clusterId,
        clusterName: host.swarm?.clusterName || clusterId.slice(0, 12),
        hosts: [],
      });
    }

    clusterMap.get(clusterId)!.hosts.push(host);
  }

  // Sort hosts within each cluster by hostname
  for (const cluster of clusterMap.values()) {
    cluster.hosts.sort((a, b) => {
      const aName = a.customDisplayName || a.displayName || a.hostname || '';
      const bName = b.customDisplayName || b.displayName || b.hostname || '';
      return aName.localeCompare(bName);
    });
  }

  // Convert to array and sort by cluster name
  const clusters = Array.from(clusterMap.values()).sort((a, b) =>
    a.clusterName.localeCompare(b.clusterName)
  );

  return clusters;
}

/**
 * Aggregates services across all hosts in a cluster, deduplicating by service ID.
 * Returns services with node distribution information.
 */
export function aggregateClusterServices(cluster: SwarmCluster): ClusterService[] {
  const serviceMap = new Map<string, ClusterService>();

  for (const host of cluster.hosts) {
    const services = host.services || [];
    const tasks = host.tasks || [];

    for (const service of services) {
      const serviceId = service.id;

      if (!serviceMap.has(serviceId)) {
        serviceMap.set(serviceId, {
          service: { ...service },
          clusterId: cluster.clusterId,
          clusterName: cluster.clusterName,
          nodes: [],
          totalDesired: service.desiredTasks || 0,
          totalRunning: 0,
        });
      }

      const aggregated = serviceMap.get(serviceId)!;

      // Count tasks for this service on this host
      const serviceTasks = tasks.filter(
        (t) => t.serviceId === serviceId || t.serviceName === service.name
      );
      const runningTasks = serviceTasks.filter(
        (t) => t.currentState?.toLowerCase() === 'running'
      );

      if (serviceTasks.length > 0) {
        aggregated.nodes.push({
          hostId: host.id,
          hostname: host.customDisplayName || host.displayName || host.hostname,
          taskCount: serviceTasks.length,
          runningCount: runningTasks.length,
        });
        aggregated.totalRunning += runningTasks.length;
      }

      // Update desired count (take max in case of inconsistencies)
      aggregated.totalDesired = Math.max(
        aggregated.totalDesired,
        service.desiredTasks || 0
      );
    }
  }

  // Sort services by stack then name
  return Array.from(serviceMap.values()).sort((a, b) => {
    const stackA = a.service.stack || '';
    const stackB = b.service.stack || '';
    if (stackA !== stackB) {
      return stackA.localeCompare(stackB);
    }
    return (a.service.name || '').localeCompare(b.service.name || '');
  });
}

/**
 * Checks if there are any Swarm clusters with multiple hosts.
 * Used to determine whether to show the cluster view toggle.
 */
export function hasSwarmClusters(hosts: DockerHost[]): boolean {
  const clusterIds = new Set<string>();

  for (const host of hosts) {
    const clusterId = host.swarm?.clusterId;
    if (clusterId) {
      if (clusterIds.has(clusterId)) {
        // Found at least 2 hosts in the same cluster
        return true;
      }
      clusterIds.add(clusterId);
    }
  }

  return false;
}

/**
 * Gets a display-friendly node list string for a service.
 * Example: "node1 (2), node2 (1), node3 (1)"
 */
export function formatNodeDistribution(
  nodes: ClusterService['nodes'],
  maxNodes: number = 3
): string {
  if (nodes.length === 0) return 'No nodes';

  const sorted = [...nodes].sort((a, b) => b.taskCount - a.taskCount);
  const display = sorted.slice(0, maxNodes);
  const remaining = sorted.length - maxNodes;

  const parts = display.map((n) => {
    if (n.taskCount === 1) return n.hostname;
    return `${n.hostname} (${n.taskCount})`;
  });

  if (remaining > 0) {
    parts.push(`+${remaining} more`);
  }

  return parts.join(', ');
}

/**
 * Gets the overall health status for a cluster service.
 */
export function getServiceHealthStatus(
  service: ClusterService
): 'healthy' | 'degraded' | 'critical' {
  const { totalDesired, totalRunning } = service;

  if (totalDesired === 0) return 'healthy';
  if (totalRunning === 0) return 'critical';
  if (totalRunning < totalDesired) return 'degraded';
  return 'healthy';
}
