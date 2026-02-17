import {
  INFRASTRUCTURE_PATH,
  INFRASTRUCTURE_QUERY_PARAMS,
  WORKLOADS_PATH,
  WORKLOADS_QUERY_PARAMS,
} from './resourceLinks';

export type AppTabId =
  | 'dashboard'
  | 'infrastructure'
  | 'workloads'
  | 'proxmox'
  | 'hosts'
  | 'docker'
  | 'containers'
  | 'kubernetes'
  | 'services'
  | 'storage'
  | 'backups'
  | 'alerts'
  | 'ai'
  | 'settings';

export type NavigationMode = 'unified' | 'classic';

export type LegacyRouteSource =
  | 'proxmox-overview'
  | 'hosts'
  | 'docker'
  | 'mail'
  | 'services'
  | 'kubernetes';

export const LEGACY_MIGRATION_PARAM = 'migrated';
export const LEGACY_MIGRATION_FROM_PARAM = 'from';

function splitPathAndSearch(path: string): { pathname: string; search: string } {
  const [pathname, query = ''] = path.split('?');
  const search = query ? `?${query}` : '';
  return { pathname, search };
}

function normalizeClassicInfrastructureSource(raw: string): AppTabId | null {
  const source = (raw || '').trim().toLowerCase();
  switch (source) {
    case 'pve':
    case 'proxmox':
    case 'proxmox-pve':
      return 'proxmox';
    case 'agent':
    case 'host-agent':
    case 'hosts':
      return 'hosts';
    case 'docker':
      return 'docker';
    case 'pmg':
    case 'proxmox-pmg':
    case 'services':
    case 'mail':
      // PMG surfaces as "services/mail" in legacy URLs. Keep it under a single classic tab.
      return 'services';
    case 'k8s':
    case 'kubernetes':
      return 'kubernetes';
    default:
      return null;
  }
}

function normalizeClassicWorkloadsType(raw: string): AppTabId | null {
  const kind = (raw || '').trim().toLowerCase();
  switch (kind) {
    case 'docker':
    case 'container':
    case 'containers':
      return 'containers';
    case 'k8s':
    case 'kubernetes':
      return 'kubernetes';
    default:
      return null;
  }
}

function getActiveClassicTab(pathname: string, search: string): AppTabId | null {
  if (pathname.startsWith(INFRASTRUCTURE_PATH)) {
    const params = new URLSearchParams(search);
    const source = params.get(INFRASTRUCTURE_QUERY_PARAMS.source) || '';
    const sources = source
      .split(',')
      .map((token) => normalizeClassicInfrastructureSource(token))
      .filter((value): value is AppTabId => Boolean(value));

    // If a single source is selected, highlight its classic tab. Otherwise fall back to Infrastructure.
    if (sources.length === 1) {
      return sources[0];
    }
    if (sources.length > 1) {
      return 'infrastructure';
    }
    return 'infrastructure';
  }

  if (pathname.startsWith(WORKLOADS_PATH)) {
    const params = new URLSearchParams(search);
    const type = params.get(WORKLOADS_QUERY_PARAMS.type) || '';
    const normalized = normalizeClassicWorkloadsType(type);
    return normalized ?? 'workloads';
  }

  // Legacy routes (when redirects are disabled).
  if (pathname.startsWith('/kubernetes')) return 'kubernetes';
  if (pathname.startsWith('/services') || pathname.startsWith('/mail') || pathname.startsWith('/proxmox/mail')) {
    return 'services';
  }
  if (pathname.startsWith('/servers')) return 'hosts';
  if (pathname.startsWith('/proxmox')) return 'proxmox';

  return null;
}

export function getActiveTabForPath(path: string, mode: NavigationMode = 'unified'): AppTabId {
  const { pathname, search } = splitPathAndSearch(path);

  if (mode === 'classic') {
    const classic = getActiveClassicTab(pathname, search);
    if (classic) return classic;
  }

  if (pathname.startsWith('/dashboard')) return 'dashboard';
  if (pathname.startsWith(INFRASTRUCTURE_PATH)) return 'infrastructure';
  if (pathname.startsWith(WORKLOADS_PATH)) return 'workloads';
  if (pathname.startsWith('/storage')) return 'storage';
  if (pathname.startsWith('/ceph')) return 'storage';
  if (pathname.startsWith('/backups')) return 'backups';
  if (pathname.startsWith('/replication')) return 'backups';
  if (pathname.startsWith('/services')) return 'infrastructure';
  if (pathname.startsWith('/mail')) return 'infrastructure';
  if (pathname.startsWith('/proxmox/ceph') || pathname.startsWith('/proxmox/storage')) return 'storage';
  if (pathname.startsWith('/proxmox/replication') || pathname.startsWith('/proxmox/backups')) return 'backups';
  if (pathname.startsWith('/proxmox/mail')) return 'infrastructure';
  if (pathname.startsWith('/proxmox')) return 'infrastructure';
  if (pathname.startsWith('/kubernetes')) return 'workloads';
  if (pathname.startsWith('/servers')) return 'infrastructure';
  if (pathname.startsWith('/alerts')) return 'alerts';
  if (pathname.startsWith('/ai')) return 'ai';
  if (pathname.startsWith('/settings')) return 'settings';
  return 'infrastructure';
}

export function buildLegacyRedirectTarget(targetPath: string, source: LegacyRouteSource): string {
  const [pathname, existingQuery = ''] = targetPath.split('?');
  const params = new URLSearchParams(existingQuery);
  params.set(LEGACY_MIGRATION_PARAM, '1');
  params.set(LEGACY_MIGRATION_FROM_PARAM, source);
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

export function mergeRedirectQueryParams(targetPath: string, incomingSearch: string): string {
  const [pathname, existingQuery = ''] = targetPath.split('?');
  const targetParams = new URLSearchParams(existingQuery);
  const normalizedIncoming = incomingSearch.startsWith('?')
    ? incomingSearch.slice(1)
    : incomingSearch;
  const incomingParams = new URLSearchParams(normalizedIncoming);

  incomingParams.forEach((value, key) => {
    if (targetParams.has(key)) return;
    targetParams.append(key, value);
  });

  const query = targetParams.toString();
  return query ? `${pathname}?${query}` : pathname;
}

export function readLegacyMigrationSource(search: string): LegacyRouteSource | null {
  const params = new URLSearchParams(search);
  const migrated = params.get(LEGACY_MIGRATION_PARAM);
  if (migrated !== '1') return null;

  const from = params.get(LEGACY_MIGRATION_FROM_PARAM);
  if (
    from === 'proxmox-overview' ||
    from === 'hosts' ||
    from === 'docker' ||
    from === 'mail' ||
    from === 'services' ||
    from === 'kubernetes'
  ) {
    return from;
  }

  return null;
}
