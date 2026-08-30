/**
 * External probe (Pro) presentation and selection helpers.
 *
 * An availability check normally runs from the Pulse server itself. With the
 * `external_probe` entitlement a check can instead be assigned to a connected
 * Pulse Agent host, which runs the probe and reports results back. The empty
 * string is the canonical "local Pulse server" value on the wire, and clearing
 * an assignment means sending that empty string explicitly.
 */

import type { AvailabilityProbeStatus } from '@/api/availabilityTargets';
import type { Resource } from '@/types/resource';
import {
  getActionableAgentIdFromResource,
  isPulseAgentPlatformResource,
} from '@/utils/agentResources';
import { getFeatureMinTierLabel, getLicenseFeatureLabel } from '@/utils/licensePresentation';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import { isConnectedHealthStatus } from '@/utils/status';

/** Runtime capability key that gates probe assignment. */
export const EXTERNAL_PROBE_FEATURE = 'external_probe';

/** Wire value for "run this check on the Pulse server itself". */
export const LOCAL_PROBE_AGENT_VALUE = '';
export const LOCAL_OBSERVATION_LOCATION_ID = 'pulse:local';
export const AGENT_OBSERVATION_LOCATION_PREFIX = 'agent:';

/** Default option label for the local Pulse server. */
export const LOCAL_PROBE_AGENT_LABEL = 'This Pulse server';

/** Canonical server error for a probe assignment whose agent stopped reporting. */
export const PROBE_AGENT_STALE_ERROR = 'no recent report from probe agent';

/** Status label used when a probe-assigned check has gone stale. */
export const PROBE_AGENT_STALE_LABEL = 'No recent probe report';

export interface ProbeAgentOption {
  /** Stable host agent id, matching the id the server validates against. */
  id: string;
  /** Human readable host name. */
  label: string;
}

export interface ObservationLocationOption {
  id: string;
  label: string;
  kind: 'pulse' | 'agent';
  agentId?: string;
}

export const observationLocationIdForAgent = (agentId: string): string =>
  `${AGENT_OBSERVATION_LOCATION_PREFIX}${agentId.trim()}`;

export const agentIdFromObservationLocation = (locationId?: string | null): string => {
  const normalized = (locationId ?? '').trim();
  return normalized.startsWith(AGENT_OBSERVATION_LOCATION_PREFIX)
    ? normalized.slice(AGENT_OBSERVATION_LOCATION_PREFIX.length).trim()
    : '';
};

/**
 * Build the selectable probe agent hosts from unified resources.
 *
 * Only genuine Pulse Agent machines qualify — the server resolves a probe
 * assignment against its live host snapshot, so provider-owned rows (Proxmox,
 * TrueNAS, VMware, Kubernetes) are not valid probe targets. Multiple resource
 * rows can resolve to one agent id, so options are deduplicated.
 */
export function buildProbeAgentOptions(resources: readonly Resource[]): ProbeAgentOption[] {
  const byId = new Map<string, ProbeAgentOption>();

  for (const resource of resources) {
    if (!isPulseAgentPlatformResource(resource)) continue;
    if (!isConnectedHealthStatus(resource.status)) continue;
    const id = getActionableAgentIdFromResource(resource);
    if (!id || byId.has(id)) continue;
    byId.set(id, { id, label: getPreferredInfrastructureDisplayName(resource) || id });
  }

  return [...byId.values()].sort((left, right) => left.label.localeCompare(right.label));
}

export function buildObservationLocationOptions(
  resources: readonly Resource[],
): ObservationLocationOption[] {
  return [
    { id: LOCAL_OBSERVATION_LOCATION_ID, label: LOCAL_PROBE_AGENT_LABEL, kind: 'pulse' },
    ...buildProbeAgentOptions(resources).map((option) => ({
      id: observationLocationIdForAgent(option.id),
      label: option.label,
      kind: 'agent' as const,
      agentId: option.id,
    })),
  ];
}

export function getObservationLocationLabel(
  options: readonly ObservationLocationOption[],
  locationId?: string | null,
): string {
  const normalized = (locationId ?? '').trim();
  if (!normalized || normalized === LOCAL_OBSERVATION_LOCATION_ID) return LOCAL_PROBE_AGENT_LABEL;
  return (
    options.find((option) => option.id === normalized)?.label ??
    (agentIdFromObservationLocation(normalized) || normalized)
  );
}

/**
 * Resolve a probe agent id to its display name, falling back to the raw id when
 * the host is not currently in the list.
 */
export function getProbeAgentLabel(
  options: readonly ProbeAgentOption[],
  probeAgentId?: string | null,
): string {
  const normalized = (probeAgentId ?? '').trim();
  if (!normalized) return LOCAL_PROBE_AGENT_LABEL;
  return options.find((option) => option.id === normalized)?.label ?? normalized;
}

/** Chip copy attributing an observation to the host that produced it. */
export function getProbeSourceChipLabel(
  options: readonly ProbeAgentOption[],
  probeAgentId?: string | null,
): string | null {
  const normalized = (probeAgentId ?? '').trim();
  if (!normalized) return null;
  return `via ${getProbeAgentLabel(options, normalized)}`;
}

/** True when a saved assignment points at a host that is not currently listed. */
export function isProbeAgentMissing(
  options: readonly ProbeAgentOption[],
  probeAgentId?: string | null,
): boolean {
  const normalized = (probeAgentId ?? '').trim();
  if (!normalized) return false;
  return !options.some((option) => option.id === normalized);
}

interface ProbeStaleStatusLike {
  outcome?: string;
  lastError?: string;
}

/**
 * A probe-assigned check whose agent stopped reporting derives to
 * `indeterminate` at read time. It is a warning state, not a failure.
 */
export function isProbeAgentStaleStatus(
  status?: ProbeStaleStatusLike | AvailabilityProbeStatus | null,
): boolean {
  if (!status) return false;
  if (status.outcome !== 'indeterminate') return false;
  return (status.lastError ?? '').trim().toLowerCase().includes(PROBE_AGENT_STALE_ERROR);
}

/** Headline for the external probe upgrade gate. */
export function getExternalProbeGateTitle(): string {
  return getLicenseFeatureLabel(EXTERNAL_PROBE_FEATURE);
}

/** Body copy for the external probe upgrade gate. */
export function getExternalProbeGateBody(): string {
  return `Running an availability check from a remote Pulse Agent host requires ${getFeatureMinTierLabel(
    EXTERNAL_PROBE_FEATURE,
  )}. Checks continue to run from this Pulse server on every plan.`;
}

/** Inline help shown under the locked control, mirroring `getTabLockReason`. */
export function getExternalProbeLockedHelpText(): string {
  return `Remote probe hosts require ${getFeatureMinTierLabel(EXTERNAL_PROBE_FEATURE)}. This check runs from the Pulse server.`;
}

interface LicenseRequiredErrorLike {
  status?: number;
  feature?: string;
  message?: string;
}

/**
 * Detect the canonical 402 `license_required` response, so a race between the
 * cached runtime capabilities and the server still lands on the upgrade gate
 * instead of a raw error string.
 */
export function isExternalProbeLicenseError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false;
  const candidate = error as LicenseRequiredErrorLike;
  if (candidate.status === 402) return true;
  if ((candidate.feature ?? '').trim() === EXTERNAL_PROBE_FEATURE) return true;
  return (candidate.message ?? '').toLowerCase().includes('license_required');
}
