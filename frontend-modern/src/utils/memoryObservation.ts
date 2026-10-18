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
  // The selected Proxmox source counts cached memory the guest can reuse as
  // used, so a high percentage alone is not evidence of memory pressure. This
  // is a property of the source, independent of how fresh the reading is.
  mayIncludeCache: boolean;
  // Why a high reading may be benign, and what the user can do about it. Set
  // exactly when mayIncludeCache is.
  cacheNote?: string;
}

// Proxmox's own listing and status values. Pulse selects them only when no
// cache-aware source (guest agent, linked Pulse agent, node meminfo) answered;
// unifiedresources.QualifyGuestMemoryForKind sets MayIncludeCache for the same
// sources and guest kinds.
const cacheInclusiveMemorySources = new Set([
  'status-mem',
  'status-freemem',
  'cluster-resources',
  'derived-total-minus-used',
]);

export type MemoryGuestKind = 'vm' | 'container';

// A container's cluster-resources mem is its cgroup usage with page cache
// already subtracted (PVE::CGroup::get_memory_stat), so it is not
// cache-inclusive. A QEMU listing, or one whose guest kind is unknown, is the
// host's view of guest RAM and counts the guest's own cache.
const isCacheInclusiveMemorySource = (source: string, kind: MemoryGuestKind | undefined): boolean =>
  cacheInclusiveMemorySources.has(source) &&
  !(source === 'cluster-resources' && kind === 'container');

const getCacheNote = (kind: MemoryGuestKind | undefined): string => {
  const noun = kind === 'vm' ? 'VM' : kind === 'container' ? 'container' : 'guest';
  const caveat = `Proxmox's reading may include cached memory the ${noun} can reuse, so a high percentage alone does not mean it is short of memory.`;
  // A fallback memory value does not establish that it is safe to change the
  // guest's agents. Keep the precaution with the advice, even without a known
  // backup lock: Pulse cannot infer the absence of a guest incident.
  const setupPrecaution =
    'Defer agent setup, restarts and live guest-agent probes during backups or a guest incident.';
  // A Pulse agent inside the guest can supply its own figure on any OS (a
  // container's must report the container's own limit). The QEMU guest agent
  // path reads Linux /proc/meminfo only and needs installing in the guest and
  // enabling in Proxmox. Neither is guaranteed to replace every fallback
  // source, so the advice says "can". Kept short: the drawer column is narrow.
  if (kind === 'vm') {
    return `${caveat} ${setupPrecaution} Installing a Pulse agent in the VM, or setting up the QEMU guest agent on a Linux VM, can let Pulse show its own figure.`;
  }
  if (kind === 'container') {
    return `${caveat} ${setupPrecaution} Installing a Pulse agent in the container can let Pulse show its own figure.`;
  }
  return caveat;
};

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
  agentStale = false,
): MemoryObservationPresentation['state'] =>
  !isMemoryPercent(value) || observation?.state === 'unavailable'
    ? 'unavailable'
    : observation?.state === 'last-known'
      ? 'last-known'
      : observation?.state === 'current' && timestamp !== null
        ? agentStale && observation.source === 'agent'
          ? 'last-known'
          : 'current'
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
      guest.agentStale === true,
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
  options: { includeCurrent?: boolean; guestKind?: MemoryGuestKind; agentStale?: boolean } = {},
): MemoryObservationPresentation | null => {
  // Unannotated unrelated platforms retain their existing behaviour. Legacy
  // Proxmox readings lack authority to assert current freshness.
  if (!requiresObservation && !observation) return null;
  const timestamp = memoryObservationTime(observation);
  // An agent-facet update can arrive without replacing the selected memory
  // carrier. Honour the server's explicit source invalidation, but only for
  // that agent's own reading: QGA and Proxmox remain independent. Do not add a
  // client lease, renew the observation time or mutate its wire metadata.
  const state = memoryObservationState(value, observation, timestamp, options.agentStale);
  // An unavailable reading shows no number, so there is nothing to qualify.
  const mayIncludeCache =
    state !== 'unavailable' &&
    observation !== undefined &&
    isCacheInclusiveMemorySource(observation.source, options.guestKind);
  // Most table rows are current and need no cue. Qualify them identically,
  // but avoid building source/date strings that will never be rendered. A
  // current reading from a cache-inclusive source still carries its caveat.
  if (state === 'current' && options.includeCurrent === false && !mayIncludeCache) return null;
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
  const cacheNote = mayIncludeCache ? getCacheNote(options.guestKind) : undefined;
  if (state === 'unavailable') {
    // No number is shown, so say why when the producer knows (the guest is
    // off) and otherwise name only what is known, never "Unknown source".
    const poweredOff = observation?.source === 'powered-off';
    const knownSource = source !== 'Unknown source';
    return {
      state,
      summary: poweredOff
        ? 'Unavailable · guest powered off'
        : `Unavailable${knownSource ? ` · ${source}` : ''}${observed ? ` · ${observed}` : ''}`,
      // Consumers prefix their own metric name ("Memory reading: ...").
      message: poweredOff
        ? 'Unavailable while the guest is powered off.'
        : `Unavailable.${knownSource ? ` Source: ${source}.` : ''}${observed ? ` ${time}` : ''}`,
      mayIncludeCache,
    };
  }
  const sourceStatus =
    options.agentStale && observation?.source === 'agent'
      ? ' The Pulse Agent that supplied this reading stopped reporting.'
      : '';
  return {
    state,
    summary: `${label} · ${source}${sourceStatus ? ' (stopped reporting)' : ''}${mayIncludeCache ? ' (may include cache)' : ''}${observed ? ` · ${observed}` : ' · time unknown'}`,
    message: `${label}. Source: ${source}. ${time}${state === 'last-known' || state === 'unknown' ? ' Not a current measurement.' : ''}${sourceStatus}${cacheNote ? ` ${cacheNote}` : ''}`,
    mayIncludeCache,
    ...(cacheNote ? { cacheNote } : {}),
  };
};

// A VM can gain a guest-agent figure; a container has no such agent. Legacy
// rows say qemu/lxc, canonical rows say vm/system-container.
export const getWorkloadMemoryGuestKind = (guest: WorkloadGuest): MemoryGuestKind | undefined =>
  guest.type === 'qemu' || guest.workloadType === 'vm'
    ? 'vm'
    : guest.type === 'lxc' || guest.workloadType === 'system-container'
      ? 'container'
      : undefined;

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
    {
      ...options,
      guestKind: getWorkloadMemoryGuestKind(guest),
      agentStale: guest.agentStale === true,
    },
  );
};
