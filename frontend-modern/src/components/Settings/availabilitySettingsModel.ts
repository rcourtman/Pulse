import type { AvailabilityTarget, AvailabilityTargetKind } from '@/api/availabilityTargets';

export const AVAILABILITY_SETTINGS_PATH = '/settings/monitoring/availability';
export const AVAILABILITY_ADD_QUERY_PARAM = 'add';
export const AVAILABILITY_ADD_TARGET_VALUE = 'target';
export const AVAILABILITY_TARGET_KIND_QUERY_PARAM = 'targetKind';

export function buildAvailabilitySettingsPath(): string {
  return AVAILABILITY_SETTINGS_PATH;
}

const AVAILABILITY_TARGET_KIND_VALUES: readonly AvailabilityTargetKind[] = [
  'machine',
  'service',
  'device',
];

export function normalizeAvailabilityTargetKind(
  value: string | null | undefined,
): AvailabilityTargetKind | undefined {
  const normalized = value?.trim().toLowerCase();
  return AVAILABILITY_TARGET_KIND_VALUES.find((kind) => kind === normalized);
}

export function buildAvailabilityTargetAddPath(targetKind?: AvailabilityTargetKind): string {
  const params = new URLSearchParams();
  params.set(AVAILABILITY_ADD_QUERY_PARAM, AVAILABILITY_ADD_TARGET_VALUE);
  if (targetKind) {
    params.set(AVAILABILITY_TARGET_KIND_QUERY_PARAM, targetKind);
  }
  return `${AVAILABILITY_SETTINGS_PATH}?${params.toString()}`;
}

export function shouldOpenAvailabilityTargetAddDialog(pathname: string, search: string): boolean {
  if (pathname !== AVAILABILITY_SETTINGS_PATH && pathname !== `${AVAILABILITY_SETTINGS_PATH}/`) {
    return false;
  }
  const params = new URLSearchParams(search);
  if (params.get(AVAILABILITY_ADD_QUERY_PARAM)?.trim() !== AVAILABILITY_ADD_TARGET_VALUE) {
    return false;
  }
  if (!params.has(AVAILABILITY_TARGET_KIND_QUERY_PARAM)) return true;
  return Boolean(normalizeAvailabilityTargetKind(params.get(AVAILABILITY_TARGET_KIND_QUERY_PARAM)));
}

export function getAvailabilityTargetAddKind(
  pathname: string,
  search: string,
): AvailabilityTargetKind | undefined {
  if (!shouldOpenAvailabilityTargetAddDialog(pathname, search)) return undefined;
  return normalizeAvailabilityTargetKind(
    new URLSearchParams(search).get(AVAILABILITY_TARGET_KIND_QUERY_PARAM),
  );
}

export function getAvailabilityTargetMethodLabel(target: AvailabilityTarget): string {
  switch (target.protocol) {
    case 'icmp':
      return 'ICMP ping';
    case 'tcp':
      return target.port ? `TCP ${target.port}` : 'TCP port';
    case 'http':
      return 'HTTP check';
    default:
      return String(target.protocol).toUpperCase();
  }
}

export function getAvailabilityTargetKindLabel(target: AvailabilityTarget): string {
  switch (target.targetKind) {
    case 'machine':
      return 'Machine';
    case 'device':
      return 'Device';
    case 'service':
    case undefined:
      return 'Service';
    default:
      return 'Endpoint';
  }
}

export function getAvailabilityTargetAddressLabel(target: AvailabilityTarget): string {
  if (target.protocol === 'http') {
    const path = target.path?.trim();
    if (path && !target.address.endsWith(path)) {
      const normalizedAddress = target.address.replace(/\/+$/, '');
      const normalizedPath = path.startsWith('/') ? path : `/${path}`;
      return `${normalizedAddress}${normalizedPath}`;
    }
    return target.address;
  }
  if (target.protocol === 'tcp' && target.port) {
    return `${target.address}:${target.port}`;
  }
  return target.address;
}

export function getAvailabilityTargetStatusLabel(target: AvailabilityTarget): string {
  if (!target.enabled) return 'Paused';
  const status = target.status;
  if (!status) return 'Not checked yet';
  if (status.available) {
    return typeof status.latencyMillis === 'number'
      ? `Online · ${status.latencyMillis} ms`
      : 'Online';
  }
  return status.lastError?.trim() || 'Offline';
}

export function getAvailabilityTargetStatusClass(target: AvailabilityTarget): string {
  if (!target.enabled) return 'bg-slate-100 text-slate-700 dark:bg-slate-900 dark:text-slate-300';
  if (!target.status) return 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300';
  if (target.status.available) {
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300';
  }
  return 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300';
}

export function getAvailabilityTargetsSummary(targets: readonly AvailabilityTarget[]): string {
  const enabled = targets.filter((target) => target.enabled).length;
  const down = targets.filter(
    (target) => target.enabled && target.status?.available === false,
  ).length;
  if (targets.length === 0) return 'No availability checks configured';
  if (down > 0) return `${down} down · ${enabled} enabled`;
  return `${enabled} enabled · ${targets.length} total`;
}
