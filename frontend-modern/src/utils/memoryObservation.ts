import type { MemoryObservation } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';

// Keep only the existing server-owned wire fields. Missing/future state is not
// replaced by a resource poll time or a platform facet from another metric.
export const readMemoryObservation = (value: unknown): MemoryObservation | undefined => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  return {
    state: typeof record.state === 'string' ? record.state : '',
    source: typeof record.source === 'string' ? record.source : '',
    ...(typeof record.observedAt === 'string' ? { observedAt: record.observedAt } : {}),
  };
};

export interface MemoryObservationPresentation {
  state: 'current' | 'last-known' | 'unavailable' | 'unknown';
  summary: string;
  message: string;
}

const isMemoryPercent = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 100;

const memoryObservationTime = (observation: MemoryObservation | undefined): number | null => {
  const timestamp =
    typeof observation?.observedAt === 'string' ? Date.parse(observation.observedAt) : NaN;
  return Number.isFinite(timestamp) && timestamp > 0 && timestamp <= Date.now() ? timestamp : null;
};

// Selection and labels classify the same source-owned observation. Numeric
// selection does not need to allocate a source/date label for every comparison.
const memoryObservationState = (
  value: number | undefined,
  observation: MemoryObservation | undefined,
  timestamp: number | null,
): MemoryObservationPresentation['state'] =>
  !isMemoryPercent(value) || observation?.state === 'unavailable'
    ? 'unavailable'
    : observation?.state === 'last-known'
      ? 'last-known'
      : observation?.state === 'current' && timestamp !== null
        ? 'current'
        : 'unknown';

const requiresWorkloadMemoryObservation = (guest: WorkloadGuest): boolean => {
  const nonProxmoxVMware =
    guest.platformScopes?.includes('vmware-vsphere') &&
    !guest.platformScopes.includes('proxmox-pve');
  return (
    !nonProxmoxVMware &&
    (guest.type === 'qemu' ||
      guest.type === 'lxc' ||
      guest.platformScopes?.includes('proxmox-pve') ||
      (guest.vmid > 0 && Boolean(guest.node && guest.instance)))
  );
};

// A retained or unknown reading remains visible in the inventory, but is not
// current capacity. Last seen, disk deferral and a host-share callback cannot
// supply missing memory provenance. Unannotated unrelated platforms keep their
// numeric behaviour, including measured zero.
export const getCurrentWorkloadMemoryUsage = (guest: WorkloadGuest): number | null => {
  const memory = guest.memory;
  if (
    guest.telemetryAvailability?.memory === false ||
    memory?.usageUnavailable ||
    !isMemoryPercent(memory?.usage)
  ) {
    return null;
  }
  if (
    (memory.observation || requiresWorkloadMemoryObservation(guest)) &&
    memoryObservationState(
      memory.usage,
      memory.observation,
      memoryObservationTime(memory.observation),
    ) !== 'current'
  ) {
    return null;
  }
  return memory.usage;
};

const memorySourceLabels: Record<string, string> = {
  'guest-agent-meminfo': 'QEMU guest agent',
  'guest-agent-meminfo-derived': 'QEMU guest agent',
  agent: 'Pulse Agent',
  'available-field': 'Proxmox',
  'derived-free-buffers-cached': 'Proxmox',
  'derived-total-minus-used': 'Proxmox',
  'status-mem': 'Proxmox',
  'status-freemem': 'Proxmox',
  'cluster-resources': 'Proxmox',
  'previous-snapshot': 'Previous snapshot',
};

// Both guest and canonical resource drawers must honour the original observation,
// not a snapshot refresh time or another metric's read state.
export const getMemoryObservationPresentation = (
  value: number | undefined,
  observation: MemoryObservation | undefined,
  requiresObservation = false,
  options: { includeCurrent?: boolean } = {},
): MemoryObservationPresentation | null => {
  // Unannotated unrelated platforms retain their existing behaviour. Legacy
  // Proxmox readings lack authority to assert current freshness.
  if (!requiresObservation && !observation) return null;
  const timestamp = memoryObservationTime(observation);
  const state = memoryObservationState(value, observation, timestamp);
  // Most table rows are current and need no cue. Qualify them identically,
  // but avoid building source/date strings that will never be rendered.
  if (state === 'current' && options.includeCurrent === false) return null;
  const observed =
    timestamp !== null
      ? new Date(timestamp)
          .toISOString()
          .replace('T', ' ')
          .replace('.000Z', 'Z')
          .replace('Z', ' UTC')
      : null;
  const label =
    state === 'current'
      ? 'Current'
      : state === 'last-known'
        ? 'Last known'
        : state === 'unavailable'
          ? 'Unavailable'
          : 'Freshness unknown';
  const source =
    observation && Object.hasOwn(memorySourceLabels, observation.source)
      ? memorySourceLabels[observation.source]
      : 'Unknown source';
  const time = observed ? `Observed: ${observed}.` : 'Observation time unknown.';
  return {
    state,
    summary: `${label} · ${source}${observed ? ` · ${observed}` : ' · time unknown'}`,
    message: `${label}. Source: ${source}. ${time}${state === 'last-known' || state === 'unknown' ? ' Not a current measurement.' : ''}`,
  };
};

// Rows and drawers qualify the same selected reading. Neither a disk-read
// deferral nor a refreshed Last seen can renew the memory observation.
export const getWorkloadMemoryObservationPresentation = (
  guest: WorkloadGuest,
  options: { includeCurrent?: boolean } = {},
): MemoryObservationPresentation | null => {
  const usage = guest.memory?.usage;
  const value =
    guest.telemetryAvailability?.memory !== false && !guest.memory?.usageUnavailable
      ? usage
      : undefined;
  return getMemoryObservationPresentation(
    value,
    guest.memory?.observation,
    requiresWorkloadMemoryObservation(guest),
    options,
  );
};
