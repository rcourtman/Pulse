import type { Resource } from '@/types/resource';
import {
  buildAlertThresholdsPath,
  buildDockerPath,
  buildKubernetesPath,
  buildProxmoxPath,
  buildStandalonePath,
  buildStorageRouteSearch,
  buildTrueNASPath,
  buildVmwarePath,
  buildWorkloadsRouteSearch,
  DOCKER_QUERY_PARAMS,
  KUBERNETES_QUERY_PARAMS,
  STANDALONE_QUERY_PARAMS,
  TRUENAS_QUERY_PARAMS,
  type AlertThresholdsTab,
} from '@/routing/resourceLinks';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import {
  collectResourcePlatformEvidence,
  type PrimaryPlatformNavId,
} from './platformNavigationModel';

/**
 * What a "go to this resource" link knows: the registry resource when it
 * resolved, else the id, name and type a record such as a Patrol attention
 * item or an alert carries.
 */
export interface ResourcePageSubject {
  id: string;
  name?: string;
  type?: string;
  resource?: Resource | null;
}

// Alerts, and the Patrol items built from them, name their resource type in
// display words (`unifiedAlertType` in internal/alerts/unified_eval.go).
const ALERT_DISPLAY_TYPES: Record<string, string> = {
  vm: 'vm',
  container: 'system-container',
  docker: 'app-container',
  agent: 'agent',
  node: 'node',
  pbs: 'pbs',
  storage: 'storage',
  pmg: 'pmg',
  'kubernetes cluster': 'k8s-cluster',
  'kubernetes node': 'k8s-node',
  'kubernetes deployment': 'k8s-deployment',
  'kubernetes namespace': 'k8s-namespace',
  'kubernetes pod': 'pod',
  'truenas system': 'truenas-system',
  'truenas pool': 'truenas-pool',
  'truenas dataset': 'truenas-dataset',
  'truenas disk': 'truenas-disk',
  'vsphere host': 'vmware-host',
  'vsphere vm': 'vmware-vm',
  'vsphere datastore': 'vmware-datastore',
  'vsphere network': 'vmware-network',
};

const normalizeType = (type: string | undefined): string => {
  const lowered = (type ?? '').trim().toLowerCase();
  return ALERT_DISPLAY_TYPES[lowered] ?? lowered.replace(/\s+/g, '-');
};

const GUEST_TYPES = new Set([
  'vm',
  'qemu',
  'lxc',
  'container',
  'system-container',
  'oci-container',
  'vmware-vm',
]);
const STORAGE_TYPES = new Set([
  'storage',
  'datastore',
  'vmware-datastore',
  'truenas-dataset',
  'truenas-pool',
]);
const DISK_TYPES = new Set(['physical_disk', 'proxmox-disk', 'disk', 'truenas-disk']);
const NETWORK_TYPES = new Set(['network', 'vmware-network']);

const DOCKER_TAB_BY_TYPE: Record<string, string> = {
  'docker-service': 'swarm',
  'docker-task': 'swarm',
  'docker-swarm-node': 'swarm',
  'docker-secret': 'swarm',
  'docker-config': 'swarm',
  'docker-image': 'images',
  'docker-volume': 'storage',
  'docker-network': 'networks',
};

const KUBERNETES_TAB_BY_TYPE: Record<string, string> = {
  'k8s-node': 'nodes',
  pod: 'workloads',
  'k8s-deployment': 'workloads',
  'k8s-statefulset': 'workloads',
  'k8s-daemonset': 'workloads',
  'k8s-replicaset': 'workloads',
  'k8s-job': 'workloads',
  'k8s-cronjob': 'workloads',
  'k8s-service': 'services',
  'k8s-ingress': 'services',
  'k8s-endpoint-slice': 'services',
  'k8s-persistent-volume': 'storage',
  'k8s-persistent-volume-claim': 'storage',
  'k8s-storage-class': 'storage',
  'k8s-namespace': 'configuration',
  'k8s-configmap': 'configuration',
  'k8s-secret': 'configuration',
  'k8s-serviceaccount': 'configuration',
  'k8s-role': 'configuration',
  'k8s-cluster-role': 'configuration',
  'k8s-role-binding': 'configuration',
  'k8s-cluster-role-binding': 'configuration',
  'k8s-resource-quota': 'configuration',
  'k8s-network-policy': 'configuration',
  'k8s-limit-range': 'configuration',
  'k8s-pod-disruption-budget': 'configuration',
  'k8s-event': 'events',
};

const isKubernetesType = (type: string) => type === 'pod' || type.startsWith('k8s-');
const isDockerType = (type: string) =>
  type === 'docker-host' || type === 'app-container' || type.startsWith('docker-');

// A resource can carry several platforms (a Proxmox node running the Pulse
// agent, a TrueNAS app that is an app container). The platform that reports
// it wins; the standalone Machines page only takes what nothing else claims.
const platformFromResource = (resource: Resource, type: string): PrimaryPlatformNavId | null => {
  const evidence = new Set(collectResourcePlatformEvidence(resource));
  if (evidence.has('truenas')) return 'truenas';
  if (isDockerType(type)) return 'docker';
  if (isKubernetesType(type)) return 'kubernetes';
  if (evidence.has('vmware-vsphere')) return 'vmware';
  if ([...evidence].some((scope) => scope.startsWith('proxmox-'))) return 'proxmox';
  if (evidence.has('kubernetes')) return 'kubernetes';
  if (evidence.has('docker')) return 'docker';
  if (evidence.has('agent')) return 'standalone';
  return null;
};

// A record the registry could not resolve still names its type.
const platformFromTypeHint = (type: string): PrimaryPlatformNavId | null => {
  if (!type) return null;
  if (type.startsWith('truenas-')) return 'truenas';
  if (type.startsWith('vmware-')) return 'vmware';
  if (isDockerType(type)) return 'docker';
  if (isKubernetesType(type) || type === 'kubernetes') return 'kubernetes';
  if (type === 'agent') return 'standalone';
  if (
    type.startsWith('proxmox-') ||
    type === 'node' ||
    type === 'pbs' ||
    type === 'pmg' ||
    type === 'ceph' ||
    GUEST_TYPES.has(type) ||
    STORAGE_TYPES.has(type) ||
    DISK_TYPES.has(type)
  )
    return 'proxmox';
  return null;
};

const withQuery = (path: string, key: string, value: string | undefined): string => {
  const trimmed = value?.trim();
  return trimmed ? `${path}?${new URLSearchParams({ [key]: trimmed }).toString()}` : path;
};

/** The primary platform whose pages show a resource, or null. */
export function resolveResourcePlatform(subject: ResourcePageSubject): PrimaryPlatformNavId | null {
  const resource = subject.resource ?? null;
  const hintType = normalizeType(subject.type);
  const type = normalizeType(resource?.type) || hintType;
  return resource
    ? (platformFromResource(resource, type) ?? platformFromTypeHint(hintType))
    : platformFromTypeHint(type);
}

const THRESHOLDS_TAB_BY_PLATFORM: Record<PrimaryPlatformNavId, AlertThresholdsTab> = {
  proxmox: 'proxmox',
  docker: 'docker',
  kubernetes: 'kubernetes',
  truenas: 'truenas',
  vmware: 'vmware',
  standalone: 'systems',
};

/**
 * The alert thresholds tab that holds a resource's rows, searched to the
 * resource's name. A registry resource is searched by its display name, the
 * label its thresholds row is built from (`getAlertResourceDisplayLabel`),
 * which can differ from its registry name. Otherwise a record name that is
 * not a composed label such as `pool (local-zfs, sda2)` is used, and physical
 * disks (which have no rows of their own there) get none: a search that
 * matches nothing would hide every row.
 */
export function buildResourceThresholdsPath(subject: ResourcePageSubject): string {
  const platform = resolveResourcePlatform(subject);
  const recordName = subject.name?.trim();
  const isDisk =
    DISK_TYPES.has(normalizeType(subject.resource?.type)) ||
    DISK_TYPES.has(normalizeType(subject.type));
  const shownName = subject.resource
    ? getPreferredInfrastructureDisplayName(subject.resource) || subject.resource.name?.trim()
    : '';
  const query = isDisk
    ? ''
    : shownName || (recordName && !/[()]/.test(recordName) ? recordName : '');
  return buildAlertThresholdsPath(platform ? THRESHOLDS_TAB_BY_PLATFORM[platform] : undefined, {
    query,
  });
}

/**
 * The page that shows a resource, selecting or filtering to it where that
 * page can: `?resource=` on workload and Proxmox storage tables (resolved
 * through the registry), a name filter on Docker containers, Kubernetes,
 * TrueNAS storage, apps, VMs and shares, and Machines, and the owning tab
 * elsewhere. Returns null when nothing names a
 * platform.
 */
export function buildResourcePagePath(subject: ResourcePageSubject): string | null {
  const resource = subject.resource ?? null;
  const hintType = normalizeType(subject.type);
  const type = normalizeType(resource?.type) || hintType;
  const id = (resource?.id ?? subject.id).trim();
  const name = (resource?.name ?? subject.name)?.trim();
  const platform = resolveResourcePlatform(subject);
  // A resolved type can be generic (`storage`, `agent`) where the hint is
  // specific (`truenas-disk`, `vmware-host`); either may name the kind.
  const isGuest = GUEST_TYPES.has(type);
  const isStorage = STORAGE_TYPES.has(type) || STORAGE_TYPES.has(hintType);
  const isDisk = DISK_TYPES.has(type) || DISK_TYPES.has(hintType);
  const isNetwork = NETWORK_TYPES.has(type) || NETWORK_TYPES.has(hintType);

  // Availability checks are endpoints Pulse probes, listed beside Machines.
  if (type === 'network-endpoint') return buildStandalonePath('availability');

  switch (platform) {
    case 'proxmox':
      if (isGuest) return `${buildProxmoxPath()}${buildWorkloadsRouteSearch({ resource: id })}`;
      if (isDisk)
        return `${buildProxmoxPath('storage')}${buildStorageRouteSearch({
          tab: 'disks',
          // Only the registry's own disk name is known to match the table.
          query: resource ? name : undefined,
        })}`;
      if (isStorage)
        return `${buildProxmoxPath('storage')}${buildStorageRouteSearch({
          // ZFS pool and device alerts name `<storage>/zfs-pool:...`, which the
          // registry cannot open; the storage they belong to can.
          resource: id.split('/zfs-pool:')[0],
        })}`;
      if (type === 'pbs') return buildProxmoxPath('backups');
      if (type === 'pmg') return buildProxmoxPath('mail');
      if (type === 'ceph') return buildProxmoxPath('ceph');
      return buildProxmoxPath();
    case 'vmware':
      if (isGuest) return `${buildVmwarePath()}${buildWorkloadsRouteSearch({ resource: id })}`;
      if (isStorage) return buildVmwarePath('storage');
      if (isNetwork) return buildVmwarePath('networks');
      return buildVmwarePath();
    case 'truenas':
      // Each tab's table reads the name filter; the systems table on Overview
      // has no search box, so a system opens the tab unfiltered. As on
      // Proxmox, a disk name from an alert alone is not known to match a row.
      if (isStorage || isDisk)
        return withQuery(
          buildTrueNASPath('storage'),
          TRUENAS_QUERY_PARAMS.query,
          isDisk && !resource ? undefined : name,
        );
      if (type === 'app-container')
        return withQuery(buildTrueNASPath('apps'), TRUENAS_QUERY_PARAMS.query, name);
      if (isGuest) return withQuery(buildTrueNASPath('vms'), TRUENAS_QUERY_PARAMS.query, name);
      if (type === 'network-share')
        return withQuery(buildTrueNASPath('shares'), TRUENAS_QUERY_PARAMS.query, name);
      return buildTrueNASPath();
    case 'docker': {
      const tab = DOCKER_TAB_BY_TYPE[type];
      // Only the containers table on Overview reads the name filter.
      return tab
        ? buildDockerPath(tab)
        : withQuery(buildDockerPath(), DOCKER_QUERY_PARAMS.query, name);
    }
    case 'kubernetes':
      return withQuery(
        buildKubernetesPath(KUBERNETES_TAB_BY_TYPE[type]),
        KUBERNETES_QUERY_PARAMS.query,
        name,
      );
    case 'standalone':
      return withQuery(buildStandalonePath(), STANDALONE_QUERY_PARAMS.query, name);
    default:
      return null;
  }
}
