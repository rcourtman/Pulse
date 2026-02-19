import { INFRASTRUCTURE_PATH, WORKLOADS_PATH } from './resourceLinks';

export type AppTabId =
  | 'dashboard'
  | 'infrastructure'
  | 'workloads'
  | 'storage'
  | 'recovery'
  | 'alerts'
  | 'ai'
  | 'settings'
  | 'operations';

export type LegacyRouteSource =
  | 'proxmox-overview'
  | 'hosts'
  | 'docker'
  | 'mail'
  | 'services'
  | 'kubernetes';

export const LEGACY_MIGRATION_PARAM = 'migrated';
export const LEGACY_MIGRATION_FROM_PARAM = 'from';

export function getActiveTabForPath(path: string): AppTabId {
  if (path.startsWith('/dashboard')) return 'dashboard';
  if (path.startsWith(INFRASTRUCTURE_PATH)) return 'infrastructure';
  if (path.startsWith(WORKLOADS_PATH)) return 'workloads';
  if (path.startsWith('/storage')) return 'storage';
  if (path.startsWith('/ceph')) return 'storage';
  if (path.startsWith('/recovery')) return 'recovery';
  if (path.startsWith('/backups')) return 'recovery';
  if (path.startsWith('/replication')) return 'recovery';
  if (path.startsWith('/services')) return 'infrastructure';
  if (path.startsWith('/mail')) return 'infrastructure';
  if (path.startsWith('/proxmox/ceph') || path.startsWith('/proxmox/storage')) return 'storage';
  if (path.startsWith('/proxmox/replication') || path.startsWith('/proxmox/backups')) return 'recovery';
  if (path.startsWith('/proxmox/mail')) return 'infrastructure';
  if (path.startsWith('/proxmox')) return 'infrastructure';
  if (path.startsWith('/kubernetes')) return 'workloads';
  if (path.startsWith('/servers')) return 'infrastructure';
  if (path.startsWith('/alerts')) return 'alerts';
  if (path.startsWith('/ai')) return 'ai';
  if (path.startsWith('/settings')) return 'settings';
  if (path.startsWith('/operations')) return 'operations';
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
