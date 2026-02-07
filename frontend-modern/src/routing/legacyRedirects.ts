import type { LegacyRouteSource } from './navigation';
import { buildInfrastructurePath, buildWorkloadsPath } from './resourceLinks';

export interface LegacyRedirectDefinition {
  path: string;
  destination: string;
  source: LegacyRouteSource;
  toastTitle: string;
  toastMessage: string;
}

export const LEGACY_REDIRECTS = {
  proxmoxOverview: {
    path: '/proxmox/overview',
    destination: buildInfrastructurePath(),
    source: 'proxmox-overview',
    toastTitle: 'Overview moved',
    toastMessage: 'Hosts and nodes are now in Infrastructure. Workloads live under Workloads.',
  },
  hosts: {
    path: '/hosts',
    destination: buildInfrastructurePath({ source: 'agent' }),
    source: 'hosts',
    toastTitle: 'Hosts moved',
    toastMessage: 'Agent hosts are now under Infrastructure.',
  },
  docker: {
    path: '/docker',
    destination: buildInfrastructurePath({ source: 'docker' }),
    source: 'docker',
    toastTitle: 'Docker moved',
    toastMessage: 'Docker hosts are in Infrastructure. Containers are in Workloads.',
  },
  proxmoxMail: {
    path: '/proxmox/mail',
    destination: buildInfrastructurePath({ source: 'pmg' }),
    source: 'mail',
    toastTitle: 'Mail Gateway moved',
    toastMessage: 'Mail Gateway is now part of Infrastructure.',
  },
  mail: {
    path: '/mail',
    destination: buildInfrastructurePath({ source: 'pmg' }),
    source: 'mail',
    toastTitle: 'Mail Gateway moved',
    toastMessage: 'Mail Gateway is now part of Infrastructure.',
  },
  services: {
    path: '/services',
    destination: buildInfrastructurePath({ source: 'pmg' }),
    source: 'services',
    toastTitle: 'Services moved',
    toastMessage:
      'Services are now under Infrastructure (PMG source). This legacy route is deprecated and will be removed after the migration window.',
  },
  kubernetes: {
    path: '/kubernetes',
    destination: buildWorkloadsPath({ type: 'k8s' }),
    source: 'kubernetes',
    toastTitle: 'Kubernetes moved',
    toastMessage:
      'Kubernetes workloads are now under Workloads. This legacy route is deprecated and will be removed after the migration window.',
  },
} as const satisfies Record<string, LegacyRedirectDefinition>;

export function getLegacyRedirectByPath(path: string): LegacyRedirectDefinition | null {
  const redirects = Object.values(LEGACY_REDIRECTS);
  const match = redirects.find((item) => item.path === path);
  return match ?? null;
}
