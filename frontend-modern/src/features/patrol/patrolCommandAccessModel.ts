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
