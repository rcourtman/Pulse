import type { Investigation, PatrolAutonomyLevel } from '@/api/patrol';
import { buildInfrastructureCommandAccessPath } from '@/components/Settings/infrastructureWorkspaceModel';
import type { Resource } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';
import { getUnifiedAgentCommandAccessOptionLabel } from '@/utils/unifiedAgentInventoryPresentation';

/**
 * Which hosts Patrol can read logs on and act on. Both need the host's Pulse
 * agent to accept commands, and agents install with that off, so on a default
 * install Patrol can do neither anywhere and nothing on the page said so.
 */
export interface PatrolCommandAccessHost {
  id: string;
  name: string;
}

export interface PatrolCommandAccessSummary {
  /** Hosts with a reporting Pulse agent, the only hosts this can be judged for. */
  total: number;
  /** Of those, the hosts whose agent accepts commands. */
  ready: number;
  /** Of those, the hosts whose agent reports commands off, by name. */
  commandsOff: PatrolCommandAccessHost[];
}

/**
 * Mirrors the server's judgement (actionlifecycle.HostCommandsDisabled): only a
 * host its own Pulse agent reports for, and only while that agent is reporting.
 * An integration-only host (TrueNAS, vSphere) carries an agent facet so machine
 * surfaces render, but no install command applies to it. The agent facet omits
 * commandsEnabled when false, so on a reporting agent an absent flag means off.
 */
export function isCommandsOffAgentHost(resource: Resource): boolean {
  const agent = resource.agent;
  if (!agent || agent.stale === true || agent.commandsEnabled === true) return false;
  return (resource.sources ?? []).includes('agent');
}

function isReportingAgentHost(resource: Resource): boolean {
  return (
    Boolean(resource.agent) &&
    resource.agent?.stale !== true &&
    (resource.sources ?? []).includes('agent')
  );
}

export function buildPatrolCommandAccessSummary(
  resources: readonly Resource[],
): PatrolCommandAccessSummary {
  const commandsOff: PatrolCommandAccessHost[] = [];
  let total = 0;
  for (const resource of resources) {
    if (!isReportingAgentHost(resource)) continue;
    total += 1;
    if (!isCommandsOffAgentHost(resource)) continue;
    commandsOff.push({
      id: resource.id,
      name:
        asTrimmedString(resource.displayName) ||
        asTrimmedString(resource.name) ||
        asTrimmedString(resource.agent?.hostname) ||
        resource.id,
    });
  }
  commandsOff.sort((left, right) => left.name.localeCompare(right.name));
  return { total, ready: total - commandsOff.length, commandsOff };
}

/**
 * Watch only never reads a host or proposes a fix, so missing command access
 * costs nothing there and the page stays quiet. Every investigating mode needs it.
 */
export function patrolModeNeedsCommandAccess(level: PatrolAutonomyLevel): boolean {
  return level !== 'monitor';
}

export interface PatrolCommandAccessHandover {
  title: string;
  step: string;
  actionLabel: string;
  actionHref: string;
}

/** The install command that grants command access, with the option ticked. */
export const PATROL_COMMAND_ACCESS_ACTION_LABEL = 'Get install command';

export function patrolCommandAccessStep(hostName: string): string {
  return `To allow them, run the Pulse install command on ${hostName} again with "${getUnifiedAgentCommandAccessOptionLabel()}" ticked.`;
}

/**
 * The hand-back Pulse wrote for a run that could not read or act because the
 * host's agent has commands off: one sentence on what that stopped, and the
 * one step that changes it. Shown once the run has ended, since the server
 * drops the fact from a run it did not stop.
 */
export function buildPatrolCommandAccessHandover(
  investigation: Investigation | null | undefined,
  active: boolean,
): PatrolCommandAccessHandover | undefined {
  const host = asTrimmedString(investigation?.commands_disabled?.host_name);
  if (!host || active) return undefined;
  return {
    title: `Patrol could not read logs or run a fix on ${host}: commands are not enabled on its Pulse agent.`,
    // The server writes the step with the run, so the review and a refused
    // action say the same words.
    step: asTrimmedString(investigation?.commands_disabled?.step) || patrolCommandAccessStep(host),
    actionLabel: PATROL_COMMAND_ACCESS_ACTION_LABEL,
    actionHref: buildInfrastructureCommandAccessPath(),
  };
}

/**
 * Core's refusal codes for a guest start on a node that is not set up for it
 * (actionlifecycle.ReasonCodeGuestStart*): no Pulse agent there accepts
 * commands, the agent is too old, or the agent cannot run a start. A node
 * whose agent has commands off is not here. The commands row names it and
 * carries its step.
 */
const GUEST_START_REFUSALS = new Set([
  'guest_start_agent_not_connected',
  'guest_start_agent_update_required',
  'guest_start_unavailable',
]);

/**
 * Which Proxmox nodes Pulse cannot start a stopped guest on. A node's Pulse
 * agent starts one, after an admin approves, when it was installed with
 * command access. Stopping or restarting a guest needs the action runner and
 * is not what this is about: a guest that died is what Patrol hands back.
 */
export interface PatrolGuestActionAccessSummary {
  /** Nodes where the server refuses to start a stopped guest, by name. */
  cannotStart: PatrolCommandAccessHost[];
}

function proxmoxGuestNode(resource: Resource): PatrolCommandAccessHost | undefined {
  const proxmox = resource.proxmox;
  if (!proxmox || proxmox.template === true) return undefined;
  if (resource.type !== 'vm' && resource.type !== 'system-container') return undefined;
  const nativeName = asTrimmedString(proxmox.nodeName) || asTrimmedString(proxmox.node);
  const name = asTrimmedString(proxmox.nodeDisplayName) || nativeName;
  if (!name) return undefined;
  // Node names repeat across connections, so the connection is part of the key.
  const id =
    asTrimmedString(proxmox.nodeIdentity) ||
    `${asTrimmedString(proxmox.instance) ?? ''}/${nativeName || name}`;
  return { id, name };
}

/**
 * Reads the server's own refusals on the guest rows, never the absence of a
 * capability: a node is listed only where planning would refuse to start one
 * of its stopped guests because of how the node is set up.
 */
export function buildPatrolGuestActionAccessSummary(
  resources: readonly Resource[],
): PatrolGuestActionAccessSummary {
  const nodes = new Map<string, string>();
  for (const resource of resources) {
    const node = proxmoxGuestNode(resource);
    if (!node) continue;
    const refusedStart = (resource.actionReadiness ?? []).some(
      (item) =>
        item.available === false &&
        (asTrimmedString(item.name) ?? '').toLowerCase() === 'start' &&
        GUEST_START_REFUSALS.has((asTrimmedString(item.reasonCode) ?? '').toLowerCase()),
    );
    if (refusedStart) nodes.set(node.id, node.name);
  }
  const cannotStart: PatrolCommandAccessHost[] = [];
  for (const [id, name] of nodes) cannotStart.push({ id, name });
  cannotStart.sort((left, right) => left.name.localeCompare(right.name));
  return { cannotStart };
}

export interface PatrolGuestActionsHandover {
  title: string;
  detail: string;
}

/**
 * The hand-back Pulse wrote for a run on a stopped Proxmox guest it could not
 * start. The server's sentence says why and what to do now, so the note has
 * no button of its own. A running guest's refused stop or reboot is rarely
 * why a run handed back, so only a refused start shows it.
 */
export function buildPatrolGuestActionsHandover(
  investigation: Investigation | null | undefined,
  active: boolean,
): PatrolGuestActionsHandover | undefined {
  const fact = investigation?.guest_actions_unavailable;
  if (!fact || active) return undefined;
  const refusedStart = (fact.actions ?? []).some(
    (action) => (asTrimmedString(action) ?? '').toLowerCase() === 'start',
  );
  if (!refusedStart) return undefined;
  const guest = asTrimmedString(fact.guest_name) || 'this guest';
  const node = asTrimmedString(fact.node_name) || 'its node';
  return {
    title: `Patrol could not start ${guest} itself.`,
    // The server writes the sentence with the run, so the review and a
    // refused action say the same words.
    detail:
      asTrimmedString(fact.reason) || `Pulse cannot start guests on ${node}. Use Proxmox for this.`,
  };
}
