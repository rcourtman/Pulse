import type { ConnectedInfrastructureItem, ConnectedInfrastructureSurface } from '@/types/api';
import { getMonitoredSystemSourceLabel } from '@/utils/monitoredSystemPresentation';

export type DashboardEstateHealthTone = 'healthy' | 'warning' | 'danger' | 'muted';

export interface DashboardEstateFallback {
  total: number;
  online: number;
}

export interface DashboardEstateSurfaceSummary {
  kind: ConnectedInfrastructureSurface['kind'];
  label: string;
  count: number;
}

export interface DashboardEstateSystemSummary {
  id: string;
  name: string;
  statusLabel: string;
  tone: DashboardEstateHealthTone;
  lastSeen?: number;
  surfaces: DashboardEstateSurfaceSummary[];
}

export interface DashboardEstateSummary {
  hasCanonicalProjection: boolean;
  totalSystems: number;
  activeSystems: number;
  healthySystems: number;
  degradedSystems: number;
  offlineSystems: number;
  unknownSystems: number;
  ignoredSystems: number;
  outdatedSystems: number;
  attentionSystems: number;
  latestSeen?: number;
  headline: string;
  detail: string;
  tone: DashboardEstateHealthTone;
  surfaces: DashboardEstateSurfaceSummary[];
  systems: DashboardEstateSystemSummary[];
}

const OFFLINE_HEALTH_STATUSES = new Set(['offline', 'stopped', 'down', 'unreachable']);
const DEGRADED_HEALTH_STATUSES = new Set(['critical', 'degraded', 'error', 'failed', 'warning']);
const HEALTHY_HEALTH_STATUSES = new Set(['active', 'healthy', 'ok', 'online', 'ready', 'running']);

const normalizeValue = (value: string | undefined): string => value?.trim().toLowerCase() ?? '';

const pluralize = (count: number, singular: string, plural = `${singular}s`): string =>
  `${count} ${count === 1 ? singular : plural}`;

function classifyHealth(item: ConnectedInfrastructureItem): {
  label: string;
  tone: DashboardEstateHealthTone;
  bucket: 'healthy' | 'degraded' | 'offline' | 'unknown' | 'ignored';
} {
  if (item.status === 'ignored') {
    return { label: 'Ignored', tone: 'muted', bucket: 'ignored' };
  }

  const health = normalizeValue(item.healthStatus);
  if (OFFLINE_HEALTH_STATUSES.has(health)) {
    return { label: 'Offline', tone: 'danger', bucket: 'offline' };
  }
  if (DEGRADED_HEALTH_STATUSES.has(health)) {
    return { label: 'Degraded', tone: 'warning', bucket: 'degraded' };
  }
  if (health === '' || HEALTHY_HEALTH_STATUSES.has(health)) {
    return { label: 'Online', tone: 'healthy', bucket: 'healthy' };
  }
  return { label: 'Unknown', tone: 'warning', bucket: 'unknown' };
}

function surfaceLabel(kind: ConnectedInfrastructureSurface['kind']): string {
  const label = getMonitoredSystemSourceLabel(kind);
  return label || kind;
}

function summarizeSurfaces(
  surfaces: ConnectedInfrastructureSurface[],
): DashboardEstateSurfaceSummary[] {
  const counts = new Map<ConnectedInfrastructureSurface['kind'], number>();

  for (const surface of surfaces) {
    counts.set(surface.kind, (counts.get(surface.kind) ?? 0) + 1);
  }

  return Array.from(counts.entries())
    .map(([kind, count]) => ({
      kind,
      label: surfaceLabel(kind),
      count,
    }))
    .sort((left, right) => {
      if (right.count !== left.count) return right.count - left.count;
      return left.label.localeCompare(right.label);
    });
}

function buildFallbackSummary(fallback: DashboardEstateFallback): DashboardEstateSummary {
  const total = Math.max(0, Math.trunc(fallback.total));
  const healthy = Math.max(0, Math.min(total, Math.trunc(fallback.online)));
  const unknown = Math.max(0, total - healthy);
  const tone: DashboardEstateHealthTone =
    total === 0 ? 'muted' : unknown > 0 ? 'warning' : 'healthy';

  return {
    hasCanonicalProjection: false,
    totalSystems: total,
    activeSystems: total,
    healthySystems: healthy,
    degradedSystems: 0,
    offlineSystems: 0,
    unknownSystems: unknown,
    ignoredSystems: 0,
    outdatedSystems: 0,
    attentionSystems: unknown,
    headline:
      total === 0
        ? 'No infrastructure reporting'
        : `${pluralize(total, 'infrastructure resource')} reporting`,
    detail:
      total === 0
        ? 'Connected systems appear here after the first infrastructure source reports.'
        : unknown > 0
          ? `${pluralize(healthy, 'resource')} online, ${pluralize(unknown, 'resource')} not classified yet.`
          : 'All reporting resources are online.',
    tone,
    surfaces: [],
    systems: [],
  };
}

export function buildDashboardEstateSummary(
  items: ConnectedInfrastructureItem[],
  fallback?: DashboardEstateFallback,
): DashboardEstateSummary {
  if (items.length === 0 && fallback) {
    return buildFallbackSummary(fallback);
  }

  let activeSystems = 0;
  let healthySystems = 0;
  let degradedSystems = 0;
  let offlineSystems = 0;
  let unknownSystems = 0;
  let ignoredSystems = 0;
  let outdatedSystems = 0;
  let latestSeen: number | undefined;
  const attentionSystemIds = new Set<string>();
  const allSurfaces: ConnectedInfrastructureSurface[] = [];

  const systems = items
    .map((item): DashboardEstateSystemSummary => {
      const health = classifyHealth(item);
      const name =
        item.displayName?.trim() || item.name?.trim() || item.hostname?.trim() || item.id;
      const itemSurfaces = summarizeSurfaces(item.surfaces ?? []);

      if (item.status === 'ignored') {
        ignoredSystems += 1;
      } else {
        activeSystems += 1;
      }

      switch (health.bucket) {
        case 'healthy':
          healthySystems += 1;
          break;
        case 'degraded':
          degradedSystems += 1;
          break;
        case 'offline':
          offlineSystems += 1;
          break;
        case 'unknown':
          unknownSystems += 1;
          break;
        case 'ignored':
          break;
      }

      if (item.isOutdatedBinary && item.status !== 'ignored') {
        outdatedSystems += 1;
      }
      if (
        item.status !== 'ignored' &&
        (health.bucket === 'degraded' ||
          health.bucket === 'offline' ||
          health.bucket === 'unknown' ||
          item.isOutdatedBinary)
      ) {
        attentionSystemIds.add(item.id);
      }
      if (typeof item.lastSeen === 'number' && Number.isFinite(item.lastSeen)) {
        latestSeen = Math.max(latestSeen ?? 0, item.lastSeen);
      }
      allSurfaces.push(...(item.surfaces ?? []));

      return {
        id: item.id,
        name,
        statusLabel: item.isOutdatedBinary ? `${health.label} · update available` : health.label,
        tone: item.isOutdatedBinary && health.tone === 'healthy' ? 'warning' : health.tone,
        lastSeen: item.lastSeen,
        surfaces: itemSurfaces,
      };
    })
    .sort((left, right) => {
      const toneRank: Record<DashboardEstateHealthTone, number> = {
        danger: 0,
        warning: 1,
        muted: 2,
        healthy: 3,
      };
      if (toneRank[left.tone] !== toneRank[right.tone]) {
        return toneRank[left.tone] - toneRank[right.tone];
      }
      return left.name.localeCompare(right.name);
    });

  const attentionSystems = attentionSystemIds.size;
  const tone: DashboardEstateHealthTone =
    offlineSystems > 0
      ? 'danger'
      : attentionSystems > 0
        ? 'warning'
        : activeSystems > 0
          ? 'healthy'
          : 'muted';

  const headline =
    items.length === 0
      ? 'No infrastructure reporting'
      : attentionSystems > 0
        ? `${pluralize(attentionSystems, 'system')} ${
            attentionSystems === 1 ? 'needs' : 'need'
          } attention`
        : `${pluralize(activeSystems, 'system')} reporting`;

  const statusParts: string[] = [];
  if (healthySystems > 0) statusParts.push(`${healthySystems} online`);
  if (degradedSystems > 0) statusParts.push(`${degradedSystems} degraded`);
  if (offlineSystems > 0) statusParts.push(`${offlineSystems} offline`);
  if (unknownSystems > 0) statusParts.push(`${unknownSystems} unknown`);
  if (ignoredSystems > 0) statusParts.push(`${ignoredSystems} ignored`);
  if (outdatedSystems > 0) statusParts.push(`${outdatedSystems} update available`);

  return {
    hasCanonicalProjection: true,
    totalSystems: items.length,
    activeSystems,
    healthySystems,
    degradedSystems,
    offlineSystems,
    unknownSystems,
    ignoredSystems,
    outdatedSystems,
    attentionSystems,
    latestSeen,
    headline,
    detail: statusParts.length > 0 ? statusParts.join(' · ') : 'Waiting for system status.',
    tone,
    surfaces: summarizeSurfaces(allSurfaces),
    systems,
  };
}
