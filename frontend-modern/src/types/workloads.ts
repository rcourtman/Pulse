import type { VM, Container, DockerContainerUpdateStatus } from './api';

export type WorkloadType = 'vm' | 'system-container' | 'docker' | 'k8s';
export type ViewMode = 'all' | 'vm' | 'system-container' | 'docker' | 'k8s';

export type WorkloadGuest = (VM | Container) & {
  workloadType?: WorkloadType;
  displayId?: string;
  image?: string;
  namespace?: string;
  contextLabel?: string;
  /** Cluster name from Proxmox (for badge display in workloads table). */
  clusterName?: string;
  platformType?: string;
  // For "docker" workloads, this is the underlying runtime ("docker", "podman", etc.)
  containerRuntime?: string;
  updateStatus?: DockerContainerUpdateStatus;
  // Docker host ID — needed for update button (= resource.docker.hostSourceId)
  dockerHostId?: string;
  // Kubernetes agent ID (when available) — preferred for actionable operations.
  kubernetesAgentId?: string;
};
