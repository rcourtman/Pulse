import type { LegacyRouteSource } from './navigation';

export type MigrationNoticeTarget = 'infrastructure' | 'workloads';

export interface LegacyRouteMigrationMetadata {
  id: LegacyRouteSource;
  target: MigrationNoticeTarget;
  title: string;
  message: string;
  rationale: string;
  status: string;
}

const DEPRECATION_STATUS = 'Deprecated compatibility alias (update bookmarks)';

export const LEGACY_ROUTE_MIGRATION_METADATA: Record<
  LegacyRouteSource,
  LegacyRouteMigrationMetadata
> = {
  'proxmox-overview': {
    id: 'proxmox-overview',
    target: 'infrastructure',
    title: 'Overview moved to Infrastructure',
    message:
      'Hosts and nodes now live in Infrastructure. VMs, containers, and pods are in Workloads.',
    rationale: 'Infrastructure now contains all hosts and nodes.',
    status: 'Legacy compatibility alias',
  },
  hosts: {
    id: 'hosts',
    target: 'infrastructure',
    title: 'Hosts moved to Infrastructure',
    message: 'Agent hosts are now shown in Infrastructure under the Agent source.',
    rationale: 'Host agents are first-class infrastructure resources.',
    status: 'Legacy compatibility alias',
  },
  docker: {
    id: 'docker',
    target: 'infrastructure',
    title: 'Containers moved to Infrastructure + Workloads',
    message: 'Container hosts are in Infrastructure. Containers are in Workloads.',
    rationale: 'Container hosts moved into infrastructure; containers are in workloads.',
    status: 'Legacy compatibility alias',
  },
  mail: {
    id: 'mail',
    target: 'infrastructure',
    title: 'Mail Gateway moved to Infrastructure',
    message: 'Mail Gateway now appears in Infrastructure under the PMG source.',
    rationale: 'Mail Gateway moved under infrastructure sources.',
    status: 'Legacy compatibility alias',
  },
  services: {
    id: 'services',
    target: 'infrastructure',
    title: 'Services moved to Infrastructure',
    message:
      'Service-level PMG infrastructure now appears in Infrastructure. The legacy /services route is deprecated; update bookmarks to the canonical Infrastructure URL.',
    rationale: 'Service-level PMG infrastructure now shares one infrastructure surface.',
    status: DEPRECATION_STATUS,
  },
  kubernetes: {
    id: 'kubernetes',
    target: 'workloads',
    title: 'Kubernetes moved to Workloads',
    message:
      'Kubernetes pods now use Workloads with unified filters and grouping. The legacy /kubernetes route is deprecated; update bookmarks to the canonical Workloads URL.',
    rationale:
      'Kubernetes pods now use the unified workload table; cluster and node health is in Infrastructure.',
    status: DEPRECATION_STATUS,
  },
};
