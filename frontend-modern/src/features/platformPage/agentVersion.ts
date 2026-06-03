import type { Resource } from '@/types/resource';
export {
  compareAgentVersions,
  formatAgentVersionDisplay,
  parseAgentVersion,
  type ParsedAgentVersion,
} from '@/utils/agentVersion';
import {
  compareAgentVersions,
  formatAgentVersionDisplay,
  parseAgentVersion,
} from '@/utils/agentVersion';

// Agent staleness helpers for platform pages.
//
// Platform pages gate their detail tabs (Docker images/networks/storage/swarm,
// and the equivalent on other runtimes) purely on whether the matching
// inventory is present in the resource snapshot. An agent that predates a given
// inventory feature simply never reports it, so those tabs silently disappear
// with no way for the user to tell "this host genuinely has none" from "this
// host's agent is too old to report it". These helpers let a page detect the
// second case by comparing each host's reported agent version against the
// server that is rendering the page.

// The agent version reported for a host. Docker hosts carry it on the docker
// meta; plain host agents carry it on the agent meta.
export function hostAgentVersion(host: Resource): string | undefined {
  return host.docker?.agentVersion?.trim() || host.agent?.agentVersion?.trim() || undefined;
}

export function hostAgentConnectionID(host: Resource): string | undefined {
  const agentId =
    host.agent?.agentId?.trim() ||
    host.kubernetes?.agentId?.trim() ||
    (host.type === 'agent' ? host.id?.trim() : '');
  if (!agentId) return undefined;
  return agentId.startsWith('agent:') ? agentId : `agent:${agentId}`;
}

export type OutdatedAgentHost = { name: string; version: string; agentId?: string };

// Hosts whose agent version is strictly behind the server version. Hosts with
// no reported version (or an unparseable one) are skipped rather than flagged,
// so we never raise a false alarm we cannot substantiate. Returns an empty list
// when the server version itself is unknown or unparseable (e.g. a build that
// does not report one), so a page never nags without a basis for comparison.
export function collectOutdatedAgentHosts(
  hosts: Resource[],
  serverVersion?: string | null,
): OutdatedAgentHost[] {
  if (!parseAgentVersion(serverVersion)) return [];

  const outdated: OutdatedAgentHost[] = [];
  for (const host of hosts) {
    const version = hostAgentVersion(host);
    if (!version) continue;
    const cmp = compareAgentVersions(version, serverVersion);
    if (cmp !== null && cmp < 0) {
      const outdatedHost: OutdatedAgentHost = {
        name: host.name?.trim() || host.id || 'host',
        version: formatAgentVersionDisplay(version),
      };
      const agentId = hostAgentConnectionID(host);
      if (agentId) {
        outdatedHost.agentId = agentId;
      }
      outdated.push(outdatedHost);
    }
  }
  return outdated;
}
