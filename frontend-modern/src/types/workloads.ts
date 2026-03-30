import type { VM, Container, DockerContainerUpdateStatus } from './api';
import type { ResourceDiscoveryTarget } from './resource';

export type WorkloadType = 'vm' | 'system-container' | 'app-container' | 'pod';
export type ViewMode = 'all' | WorkloadType;

export type WorkloadGuest = (VM | Container) & {
  workloadType?: WorkloadType;
  displayId?: string;
  image?: string;
  // Provider/runtime-native identifier for app-container actions such as Docker image updates.
  // Canonical workload identity remains `id`.
  containerId?: string;
  namespace?: string;
  contextLabel?: string;
  /** Cluster name from Proxmox (for badge display in workloads table). */
  clusterName?: string;
  platformType?: string;
  // For app-container workloads, this is underlying runtime telemetry
  // ("docker", "podman", etc.), not the owning platform.
  containerRuntime?: string;
  updateStatus?: DockerContainerUpdateStatus;
  // Agent-managed Docker runtime host ID for Docker-specific update actions.
  // API-backed platforms such as TrueNAS must not populate this field.
  dockerHostId?: string;
  // Kubernetes agent ID (when available) — preferred for actionable operations.
  kubernetesAgentId?: string;
  // Canonical discovery ownership. API-backed platforms such as TrueNAS should
  // only expose Discovery when the unified resource contract supplies a target.
  discoveryTarget?: ResourceDiscoveryTarget;
};
