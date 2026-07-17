import type { VM, Container, DockerContainerUpdateStatus } from './api';
import type {
  ResourceActionReadiness,
  ResourceAvailabilityMeta,
  ResourceDiscoveryReadiness,
  ResourceDiscoveryTarget,
} from './resource';

export type WorkloadType = 'vm' | 'system-container' | 'app-container' | 'pod';
export type WorkloadContainerViewMode = 'container' | 'system-container' | 'app-container';
export type ViewMode = 'all' | 'vm' | WorkloadContainerViewMode | 'pod';

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
  // Canonical platform-page membership. A runtime workload may belong to more
  // than one platform scope, e.g. Docker inside a Proxmox LXC.
  platformScopes?: string[];
  // For app-container workloads, this is underlying runtime telemetry
  // ("docker", "podman", etc.), not the owning platform.
  containerRuntime?: string;
  updateStatus?: DockerContainerUpdateStatus;
  // Server-evaluated capability refusals from the unified resource. The
  // update button reads this to disable itself with the refusal reason
  // (agent disconnected, agent too old, stale inventory) before the click.
  actionReadiness?: ResourceActionReadiness[];
  // Agent-managed Docker runtime host ID for Docker-specific update actions.
  // API-backed platforms such as TrueNAS must not populate this field.
  dockerHostId?: string;
  // Docker/Podman host label used by the Docker runtime page's host facet.
  dockerHostName?: string;
  // Kubernetes agent ID (when available) — preferred for actionable operations.
  kubernetesAgentId?: string;
  // Canonical discovery ownership. API-backed platforms such as TrueNAS should
  // only expose Discovery when the unified resource contract supplies a target.
  discoveryTarget?: ResourceDiscoveryTarget;
  discoveryReadiness?: ResourceDiscoveryReadiness;
  // vSphere placement context surfaced in the workload drawer's vSphere
  // card. Only populated for resources with platformScopes containing
  // `vmware-vsphere`. Kept narrow on purpose — operational state like
  // Tools / snapshots / disks lives in the unified resource payload and
  // hasn't been mapped onto WorkloadGuest because the workload drawer
  // hasn't asked for it yet.
  vmware?: {
    connectionName?: string;
    vcenterHost?: string;
    datacenterName?: string;
    clusterName?: string;
  };
  /** Availability probe facet from the unified resource model. */
  availability?: ResourceAvailabilityMeta;
};
