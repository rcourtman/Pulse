import type { VM, Container } from './api';

export type WorkloadType = 'vm' | 'lxc' | 'docker' | 'k8s';
export type ViewMode = 'all' | 'vm' | 'lxc' | 'docker' | 'k8s';

export type WorkloadGuest = (VM | Container) & {
  workloadType?: WorkloadType;
  displayId?: string;
  image?: string;
  namespace?: string;
  contextLabel?: string;
  platformType?: string;
};
