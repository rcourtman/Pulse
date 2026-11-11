import type { ResourceActionReadiness } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

const normalizeToken = (value: unknown): string => (asTrimmedString(value) ?? '').toLowerCase();

/**
 * Resolve the server-reported refusal for a capability from the unified
 * resource's actionReadiness facet. Websocket and REST rows both carry it, so a
 * match means POST /api/actions/plan would reject the action. Consumers read
 * this facet and not the absence of a capability: REST strips refused
 * capabilities and websocket rows keep them. Prefers the server-provided
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
    case 'host_commands_disabled':
      return 'Commands are not enabled on the Pulse agent on this host.';
    case 'action_runner_unavailable':
      return 'Pulse cannot stop or restart guests on this node: that needs the Pulse action runner there. Use Proxmox for this.';
    case 'guest_start_agent_not_connected':
      return 'Pulse cannot start guests on this node: no Pulse agent that accepts commands is connected there. Use Proxmox for this.';
    case 'guest_start_agent_update_required':
      return 'Pulse cannot start guests on this node yet: its Pulse agent is too old. Update the agent, then try again.';
    case 'guest_start_unavailable':
      return 'Pulse cannot start guests on this node: its Pulse agent cannot run a start. Use Proxmox for this.';
    case 'command_agent_disconnected':
      return 'Docker / Podman command agent is not connected.';
    case 'no_command_agent':
      return 'Install the Pulse agent inside this guest to manage its containers.';
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
