import {
  isStorage,
  isWorkload,
  type Resource,
  type ResourceHealthReason,
  type ResourceHealthVerdict,
} from '@/types/resource';
import type { StatusIndicatorVariant } from '@/utils/status';
import {
  buildDockerPath,
  buildKubernetesPath,
  buildProxmoxPath,
  buildStandalonePath,
  buildStorageRouteSearch,
  buildTrueNASPath,
  buildVmwarePath,
  buildWorkloadsRouteSearch,
} from '@/routing/resourceLinks';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';

export const HOME_HEALTHY_GROUP_LIMIT = 60;

export type HomePlatformKey =
  'proxmox' | 'docker' | 'kubernetes' | 'truenas' | 'vmware' | 'standalone' | 'other';

export type HomeResourceTile = {
  resource: Resource;
  name: string;
  verdict: ResourceHealthVerdict;
  reason?: ResourceHealthReason;
  href: string;
};

export type HomeResourceGroup = {
  key: HomePlatformKey;
  tiles: HomeResourceTile[];
  hiddenCount: number;
};

export type HomePosture = Record<ResourceHealthVerdict, number> & {
  total: number;
  needsAttention: number;
};

const PLATFORM_ORDER: readonly HomePlatformKey[] = [
  'proxmox',
  'docker',
  'kubernetes',
  'truenas',
  'vmware',
  'standalone',
  'other',
];

const VERDICT_RANK: Record<ResourceHealthVerdict, number> = {
  critical: 5,
  attention: 4,
  stale: 3,
  unknown: 2,
  off: 1,
  ok: 0,
};

const KUBERNETES_WORKLOAD_TYPES = new Set([
  'k8s-deployment',
  'k8s-replicaset',
  'k8s-statefulset',
  'k8s-daemonset',
  'k8s-job',
  'k8s-cronjob',
]);

export function getHomeVerdictTone(verdict: ResourceHealthVerdict): StatusIndicatorVariant {
  switch (verdict) {
    case 'ok':
      return 'success';
    case 'critical':
      return 'danger';
    case 'attention':
      return 'warning';
    default:
      return 'muted';
  }
}

export function getHomePlatformKey(resource: Resource): HomePlatformKey {
  const platform = String(resource.platformType || '').toLowerCase();
  const sources = (resource.sources ?? []).map((source) => source.toLowerCase());
  if (
    platform.includes('proxmox') ||
    sources.some((source) => ['proxmox', 'pbs', 'pmg'].includes(source))
  )
    return 'proxmox';
  if (platform.includes('docker') || platform.includes('podman') || sources.includes('docker'))
    return 'docker';
  if (platform.includes('kubernetes') || sources.includes('kubernetes')) return 'kubernetes';
  if (platform.includes('truenas') || sources.includes('truenas')) return 'truenas';
  if (platform.includes('vmware') || platform.includes('vsphere') || sources.includes('vmware'))
    return 'vmware';
  if (
    platform.includes('standalone') ||
    platform.includes('availability') ||
    sources.includes('agent') ||
    sources.includes('availability')
  )
    return 'standalone';
  return 'other';
}

export function getHomeResourceHref(resource: Resource): string {
  const platform = getHomePlatformKey(resource);
  const buildPlatformPath = (tab = 'overview') => {
    switch (platform) {
      case 'proxmox':
        return buildProxmoxPath(tab);
      case 'docker':
        return buildDockerPath(tab);
      case 'kubernetes':
        return buildKubernetesPath(tab);
      case 'truenas':
        return buildTrueNASPath(tab);
      case 'vmware':
        return buildVmwarePath(tab);
      case 'standalone':
        return buildStandalonePath(tab === 'overview' ? 'machines' : tab);
      default:
        return '/alerts/overview';
    }
  };

  if (isWorkload(resource) || KUBERNETES_WORKLOAD_TYPES.has(resource.type)) {
    const search = buildWorkloadsRouteSearch({ resource: resource.id });
    switch (platform) {
      case 'proxmox':
        return `${buildProxmoxPath('overview')}${search}`;
      case 'kubernetes':
        return `${buildKubernetesPath('workloads')}${search}`;
      case 'truenas':
        return buildTrueNASPath(resource.type === 'vm' ? 'vms' : 'apps');
      case 'vmware':
        return `${buildVmwarePath('overview')}${search}`;
      case 'standalone':
        return `${buildStandalonePath('machines')}${search}`;
      case 'docker':
        return buildDockerPath('overview');
      default:
        return '/alerts/overview';
    }
  }
  if (isStorage(resource)) {
    if (platform === 'standalone') return buildStandalonePath('machines');
    if (platform === 'other') return '/alerts/overview';
    return `${buildPlatformPath('storage')}${buildStorageRouteSearch({ resource: resource.id })}`;
  }
  if (resource.type === 'network-endpoint') {
    return buildStandalonePath('availability');
  }
  return buildPlatformPath();
}

export function toHomeResourceTile(resource: Resource): HomeResourceTile {
  const verdict = resource.health?.verdict ?? 'unknown';
  return {
    resource,
    name: getPreferredInfrastructureDisplayName(resource),
    verdict,
    reason: resource.health?.reasons?.[0],
    href: getHomeResourceHref(resource),
  };
}

export function compareHomeTiles(left: HomeResourceTile, right: HomeResourceTile): number {
  const verdictDifference = VERDICT_RANK[right.verdict] - VERDICT_RANK[left.verdict];
  if (verdictDifference !== 0) return verdictDifference;
  return left.name.localeCompare(right.name, undefined, { sensitivity: 'base', numeric: true });
}

export function buildHomePosture(resources: readonly Resource[]): HomePosture {
  const posture: HomePosture = {
    ok: 0,
    attention: 0,
    critical: 0,
    stale: 0,
    off: 0,
    unknown: 0,
    total: resources.length,
    needsAttention: 0,
  };
  resources.forEach((resource) => {
    const verdict = resource.health?.verdict ?? 'unknown';
    posture[verdict] += 1;
    if (verdict === 'critical' || verdict === 'attention') posture.needsAttention += 1;
  });
  return posture;
}

export function buildHomeAttentionTiles(resources: readonly Resource[]): HomeResourceTile[] {
  return resources
    .map(toHomeResourceTile)
    .filter((tile) => tile.verdict === 'critical' || tile.verdict === 'attention')
    .sort(compareHomeTiles);
}

export function buildHomeResourceGroups(
  resources: readonly Resource[],
  expandedGroups: ReadonlySet<HomePlatformKey> = new Set(),
): HomeResourceGroup[] {
  const grouped = new Map<HomePlatformKey, HomeResourceTile[]>();
  resources.forEach((resource) => {
    const tile = toHomeResourceTile(resource);
    if (tile.verdict === 'critical' || tile.verdict === 'attention') return;
    const key = getHomePlatformKey(resource);
    grouped.set(key, [...(grouped.get(key) ?? []), tile]);
  });

  return PLATFORM_ORDER.flatMap((key) => {
    const tiles = (grouped.get(key) ?? []).sort(compareHomeTiles);
    if (tiles.length === 0) return [];
    if (expandedGroups.has(key)) return [{ key, tiles, hiddenCount: 0 }];

    const neverHidden = tiles.filter((tile) => tile.verdict !== 'ok' && tile.verdict !== 'off');
    const calm = tiles.filter((tile) => tile.verdict === 'ok' || tile.verdict === 'off');
    const visibleCalm = calm.slice(0, HOME_HEALTHY_GROUP_LIMIT);
    return [
      {
        key,
        tiles: [...neverHidden, ...visibleCalm],
        hiddenCount: calm.length - visibleCalm.length,
      },
    ];
  });
}
