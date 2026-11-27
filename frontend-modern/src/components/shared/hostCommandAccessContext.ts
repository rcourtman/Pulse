import type { Connection } from '@/api/connections';
import type { Resource } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';

export type EnableRemoteCommandsInstallerStep = 'agent' | 'unraid' | 'docker' | 'kubernetes';

export type EnableRemoteCommandsActionKind = 'update' | 'start' | 'stop' | 'restart';

export interface HostCommandAccessContext {
  hostResourceId?: string;
  hostName: string;
  agentId?: string;
  hostname?: string;
  platform?: string;
  hostProfile?: string;
  collectionMode?: string;
  commandsEnabled?: boolean;
}

export interface EnableRemoteCommandsTarget extends HostCommandAccessContext {
  containerName?: string;
  actionKind?: EnableRemoteCommandsActionKind;
}

export const stripAgentPrefix = (value: string | undefined): string => {
  const trimmed = asTrimmedString(value) ?? '';
  return trimmed.startsWith('agent:') ? trimmed.slice('agent:'.length) : trimmed;
};

export function extractHostCommandAccessContext(
  host?: Resource | null,
  fallback?: { hostName?: string; agentId?: string; hostCommandsEnabled?: boolean },
): HostCommandAccessContext {
  const hostName =
    asTrimmedString(host?.displayName) ||
    asTrimmedString(host?.name) ||
    asTrimmedString(fallback?.hostName) ||
    'this host';
  const agentId =
    stripAgentPrefix(host?.agent?.agentId) ||
    stripAgentPrefix(host?.docker?.agentId) ||
    stripAgentPrefix(fallback?.agentId) ||
    undefined;
  const hostname =
    asTrimmedString(host?.agent?.hostname) ||
    asTrimmedString(host?.docker?.hostname) ||
    asTrimmedString(host?.name) ||
    undefined;
  const commandsEnabled = host?.agent
    ? host.agent.commandsEnabled === true
    : fallback?.hostCommandsEnabled;

  return {
    hostResourceId: asTrimmedString(host?.id) || undefined,
    hostName,
    agentId,
    hostname,
    platform:
      asTrimmedString(host?.agent?.platform) || asTrimmedString(host?.docker?.os) || undefined,
    hostProfile: asTrimmedString(host?.agent?.hostProfile) || undefined,
    collectionMode: asTrimmedString(host?.docker?.collectionMode) || undefined,
    commandsEnabled,
  };
}

export function extractConnectionCommandAccessContext(
  connection: Connection,
): EnableRemoteCommandsTarget {
  const agentId = stripAgentPrefix(connection.id) || undefined;
  const hostName =
    asTrimmedString(connection.name) || asTrimmedString(connection.address) || 'this host';
  const hostname =
    asTrimmedString(connection.agentIdentity?.hostname) ||
    asTrimmedString(connection.address) ||
    asTrimmedString(connection.name) ||
    undefined;

  return {
    hostResourceId: asTrimmedString(connection.id) || undefined,
    hostName,
    agentId,
    hostname,
    platform: asTrimmedString(connection.agentIdentity?.platform) || undefined,
    hostProfile: asTrimmedString(connection.agentIdentity?.hostProfile) || undefined,
    commandsEnabled: connection.agentIdentity?.commandsEnabled === true,
  };
}
