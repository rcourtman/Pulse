import type { ResourceActionReadiness } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

const normalizeToken = (value: unknown): string => (asTrimmedString(value) ?? '').toLowerCase();

/**
 * Resolve the server-reported refusal for a capability from the unified
 * resource's actionReadiness facet. The server strips capabilities it would
 * refuse from `capabilities` and records the refusal here, so a match means
 * POST /api/actions/plan would reject the action. Prefers the server-provided
 * reason; falls back to a canned message per reason code.
 */
export const getActionReadinessRefusal = (
  readinessList: ResourceActionReadiness[] | undefined,
  capabilityName: string,
): string | undefined => {
  const target = normalizeToken(capabilityName);
  const readiness = readinessList?.find(
    (item) => normalizeToken(item.name) === target && item.available === false,
  );
  if (!readiness) return undefined;
  const reason = asTrimmedString(readiness.reason);
  if (reason) return reason;
  switch (normalizeToken(readiness.reasonCode)) {
    case 'command_agent_disconnected':
      return 'Docker / Podman command agent is not connected.';
    case 'command_agent_unavailable':
      return 'Docker / Podman command execution is not available.';
    case 'stale_inventory':
      return 'Docker / Podman inventory is not fresh enough to run lifecycle actions.';
    case 'host_policy_blocked':
      return 'Docker / Podman host policy blocks mutating lifecycle actions.';
    case 'unsupported_handler':
      return 'This container action is not routed through the supported lifecycle executor.';
    default:
      return undefined;
  }
};
