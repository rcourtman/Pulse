import { formatDiscoveryAge } from '@/api/discovery';
import type { ResourceDiscoveryReadiness, ResourceDiscoveryReadinessState } from '@/types/resource';

export type DiscoveryReadinessTone = 'success' | 'warning' | 'info' | 'danger' | 'muted';

export interface DiscoveryReadinessPresentation {
  state: ResourceDiscoveryReadinessState | 'unknown';
  label: string;
  shortLabel: string;
  statusLabel: string;
  title: string;
  detail: string;
  tone: DiscoveryReadinessTone;
}

const STATE_COPY: Record<
  ResourceDiscoveryReadinessState,
  Pick<DiscoveryReadinessPresentation, 'label' | 'shortLabel' | 'statusLabel' | 'tone'>
> = {
  fresh: {
    label: 'Discovery fresh',
    shortLabel: 'Fresh',
    statusLabel: 'Discovery fresh',
    tone: 'success',
  },
  stale: {
    label: 'Discovery stale',
    shortLabel: 'Stale',
    statusLabel: 'Discovery stale',
    tone: 'warning',
  },
  missing: {
    label: 'Not discovered',
    shortLabel: 'None',
    statusLabel: 'No discovery data',
    tone: 'muted',
  },
  running: {
    label: 'Discovery running',
    shortLabel: 'Running',
    statusLabel: 'Discovery running',
    tone: 'info',
  },
  failed: {
    label: 'Discovery failed',
    shortLabel: 'Failed',
    statusLabel: 'Discovery failed',
    tone: 'danger',
  },
  unavailable: {
    label: 'Discovery unavailable',
    shortLabel: 'Unavailable',
    statusLabel: 'Discovery unavailable',
    tone: 'warning',
  },
  unsupported: {
    label: 'Not supported',
    shortLabel: 'N/A',
    statusLabel: 'Discovery unsupported',
    tone: 'muted',
  },
};

export const formatReadinessAge = (seconds?: number): string => {
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds < 0) return '';
  if (seconds < 60) return 'under a minute old';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} old`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} old`;
  const days = Math.floor(hours / 24);
  return `${days} day${days === 1 ? '' : 's'} old`;
};

const observedLabel = (readiness: ResourceDiscoveryReadiness): string => {
  if (readiness.observedAt) return formatDiscoveryAge(readiness.observedAt);
  return formatReadinessAge(readiness.ageSeconds);
};

export const getDiscoveryReadinessPresentation = (
  readiness?: ResourceDiscoveryReadiness | null,
  hasDiscoverySupport = true,
): DiscoveryReadinessPresentation | null => {
  if (!readiness) {
    if (!hasDiscoverySupport) return null;
    return {
      state: 'unknown',
      label: 'Discovery unknown',
      shortLabel: 'Unknown',
      statusLabel: 'Discovery unknown',
      title: 'Discovery status is not available for this resource yet.',
      detail: 'Discovery status unavailable',
      tone: 'muted',
    };
  }

  const copy = STATE_COPY[readiness.state] ?? {
    label: 'Discovery unknown',
    shortLabel: 'Unknown',
    statusLabel: 'Discovery unknown',
    tone: 'muted' as const,
  };
  const details = [
    readiness.reason,
    readiness.serviceName ? `Service: ${readiness.serviceName}` : '',
    readiness.factCount && readiness.factCount > 0 ? `${readiness.factCount} facts` : '',
    observedLabel(readiness) ? `Observed ${observedLabel(readiness)}` : '',
  ].filter((value): value is string => Boolean(value && value.trim()));
  const detail = details.join(' · ');

  return {
    state: readiness.state,
    label: copy.label,
    shortLabel: copy.shortLabel,
    statusLabel: copy.statusLabel,
    title: detail ? `${copy.label}: ${detail}` : copy.label,
    detail,
    tone: copy.tone,
  };
};

export const formatDiscoveryReadinessBriefingLine = (
  readiness?: ResourceDiscoveryReadiness | null,
): string => {
  const presentation = getDiscoveryReadinessPresentation(readiness, Boolean(readiness));
  if (!presentation) return '';
  const details = [
    presentation.statusLabel,
    readiness?.serviceName ? `service ${readiness.serviceName}` : '',
    readiness?.observedAt ? `observed ${formatDiscoveryAge(readiness.observedAt)}` : '',
    readiness?.factCount && readiness.factCount > 0 ? `${readiness.factCount} facts` : '',
  ].filter((value): value is string => Boolean(value && value.trim()));
  return details.length > 0 ? `Discovery data: ${details.join(', ')}` : '';
};
