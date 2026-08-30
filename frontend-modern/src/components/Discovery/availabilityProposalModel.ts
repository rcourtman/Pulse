import type { AvailabilityTarget } from '@/api/availabilityTargets';
import type { AvailabilityProbeSuggestion, DiscoverySummary } from '@/types/discovery';

export const DEFAULT_DISCOVERY_ASSURANCE_INTERVAL_SECONDS = 60;

export type AvailabilityProposalDuplicateKind = 'endpoint' | 'resource';

export interface AvailabilityProposalDuplicate {
  kind: AvailabilityProposalDuplicateKind;
  target: AvailabilityTarget;
}

const defaultPort = (protocol: string): number => {
  if (protocol === 'http') return 80;
  if (protocol === 'https') return 443;
  return 0;
};

const normalizedPath = (value?: string): string => {
  const trimmed = (value ?? '').trim();
  if (!trimmed || trimmed === '/') return '/';
  return `/${trimmed.replace(/^\/+/, '').replace(/\/+$/, '')}`;
};

const normalizedEndpoint = (target: {
  protocol: string;
  address: string;
  port?: number;
  path?: string;
}): string => {
  const protocol = target.protocol.trim().toLowerCase();
  let host = target.address.trim().toLowerCase().replace(/\.$/, '');
  let port = target.port || defaultPort(protocol);
  let endpointPath = normalizedPath(target.path);

  if ((protocol === 'http' || protocol === 'https') && /^https?:\/\//i.test(host)) {
    try {
      const parsed = new URL(host);
      host = parsed.hostname.toLowerCase().replace(/\.$/, '');
      port = Number(parsed.port) || target.port || defaultPort(protocol);
      if (!target.path) endpointPath = normalizedPath(parsed.pathname);
    } catch {
      // Keep the literal address. The canonical API validates it before save.
    }
  }

  return [protocol, host.replace(/^\[|\]$/g, ''), String(port), endpointPath].join('|');
};

export function findAvailabilityProposalDuplicate(
  proposal: AvailabilityProbeSuggestion,
  canonicalResourceId: string,
  targets: readonly AvailabilityTarget[],
): AvailabilityProposalDuplicate | null {
  const endpoint = normalizedEndpoint(proposal);
  const exact = targets.find((target) => normalizedEndpoint(target) === endpoint);
  if (exact) return { kind: 'endpoint', target: exact };

  const resourceId = canonicalResourceId.trim();
  const attached = resourceId
    ? targets.find((target) => (target.linkedResourceId ?? '').trim() === resourceId)
    : undefined;
  return attached ? { kind: 'resource', target: attached } : null;
}

export function buildAvailabilityTargetFromProposal(input: {
  proposal: AvailabilityProbeSuggestion;
  canonicalResourceId: string;
  name: string;
  intervalSeconds?: number;
  probeAgentId?: string;
}): AvailabilityTarget {
  const { proposal } = input;
  const protocol = proposal.protocol.trim().toLowerCase() as AvailabilityTarget['protocol'];
  const isHTTP = protocol === 'http' || protocol === 'https';

  return {
    id: '',
    name: input.name.trim() || proposal.service_name.trim() || proposal.address.trim(),
    targetKind: 'service',
    address: proposal.address.trim(),
    protocol,
    port: proposal.port || undefined,
    path: proposal.path?.trim() || undefined,
    linkedResourceId: input.canonicalResourceId.trim(),
    enabled: true,
    pollIntervalSeconds:
      input.intervalSeconds && input.intervalSeconds >= 10
        ? input.intervalSeconds
        : DEFAULT_DISCOVERY_ASSURANCE_INTERVAL_SECONDS,
    timeoutMillis: 2000,
    failureThreshold: 2,
    probeAgentId: input.probeAgentId?.trim() || undefined,
    certificateExpiryWarningDays: protocol === 'https' ? 30 : undefined,
    http: isHTTP
      ? {
          method: 'GET',
          authentication: { type: 'none' },
          expectedStatusMin: 200,
          expectedStatusMax: 399,
        }
      : undefined,
  };
}

export function isAvailabilityProposalDismissed(discovery: {
  suggested_availability_probe?: AvailabilityProbeSuggestion;
  dismissed_availability_probe_fingerprint?: string;
}): boolean {
  const fingerprint = discovery.suggested_availability_probe?.evidence_fingerprint?.trim();
  return Boolean(
    fingerprint && fingerprint === discovery.dismissed_availability_probe_fingerprint?.trim(),
  );
}

export function reviewableAvailabilitySummaries(
  discoveries: readonly DiscoverySummary[],
): DiscoverySummary[] {
  return discoveries
    .filter((discovery) => Boolean(discovery.suggested_availability_probe))
    .sort((left, right) => {
      const leftDismissed = isAvailabilityProposalDismissed(left);
      const rightDismissed = isAvailabilityProposalDismissed(right);
      if (leftDismissed !== rightDismissed) return leftDismissed ? 1 : -1;
      return (left.service_name || left.hostname).localeCompare(
        right.service_name || right.hostname,
      );
    });
}
