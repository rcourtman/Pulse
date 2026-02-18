import type { VM, Container, DockerContainerUpdateStatus } from './api';

export type WorkloadType = 'vm' | 'lxc' | 'docker' | 'k8s';
export type ViewMode = 'all' | 'vm' | 'lxc' | 'docker' | 'k8s';

export type WorkloadGuest = (VM | Container) & {
  workloadType?: WorkloadType;
  displayId?: string;
  image?: string;
  namespace?: string;
  contextLabel?: string;
  platformType?: string;
  // For "docker" workloads, this is the underlying runtime ("docker", "podman", etc.)
  containerRuntime?: string;
  updateStatus?: DockerContainerUpdateStatus;
  // Docker host ID — needed for update button (= resource.docker.hostSourceId)
  dockerHostId?: string;
};
