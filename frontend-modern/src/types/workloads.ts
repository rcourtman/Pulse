import type { VM, Container, DockerContainerUpdateStatus } from './api';

export type WorkloadType = 'vm' | 'system-container' | 'app-container' | 'pod';
export type ViewMode = 'all' | WorkloadType;

export type WorkloadGuest = (VM | Container) & {
  workloadType?: WorkloadType;
  displayId?: string;
  image?: string;
  namespace?: string;
  contextLabel?: string;
  /** Cluster name from Proxmox (for badge display in workloads table). */
  clusterName?: string;
  platformType?: string;
  // For app-container workloads, this is the underlying runtime ("docker", "podman", etc.)
  containerRuntime?: string;
  updateStatus?: DockerContainerUpdateStatus;
  // Docker host ID — needed for update button (= resource.docker.hostSourceId)
  dockerHostId?: string;
  // Kubernetes agent ID (when available) — preferred for actionable operations.
  kubernetesAgentId?: string;
};
