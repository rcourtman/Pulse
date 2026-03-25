import { unwrap } from 'solid-js/store';
import type { APITokenRecord } from '@/api/security';
import {
  AGENT_REPORT_SCOPE,
  API_SCOPE_OPTIONS,
  DOCKER_MANAGE_SCOPE,
  DOCKER_REPORT_SCOPE,
  MONITORING_READ_SCOPE,
  SETTINGS_READ_SCOPE,
  SETTINGS_WRITE_SCOPE,
} from '@/constants/apiScopes';
import {
  getActionableAgentIdFromResource,
  getActionableDockerRuntimeIdFromResource,
  hasAgentFacet as resourceHasAgentFacet,
} from '@/utils/agentResources';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import type { Resource } from '@/types/resource';

export const API_TOKEN_SCOPES_DOC_URL =
  'https://github.com/rcourtman/Pulse/blob/main/docs/CONFIGURATION.md#token-scopes';

export const API_TOKEN_WILDCARD_SCOPE = '*';

export interface APITokenPreset {
  label: string;
  scopes: string[];
  description: string;
}

export interface APITokenUsageEntry {
  count: number;
  items: { id: string; label: string }[];
}

type ScopeGroup = (typeof API_SCOPE_OPTIONS)[number]['group'];
type ScopeOption = (typeof API_SCOPE_OPTIONS)[number];

const API_TOKEN_SCOPE_GROUP_ORDER: ScopeGroup[] = ['Monitoring', 'Agents', 'Settings'];

export const getAPITokenScopePresets = (): APITokenPreset[] => [
  {
    label: 'Kiosk / Dashboard',
    scopes: [MONITORING_READ_SCOPE],
    description:
      'Read-only access for wall displays. Use ?token=xxx&kiosk=1 in the URL to hide navigation and filters.',
  },
  {
    label: 'Agent',
    scopes: [AGENT_REPORT_SCOPE],
    description: 'Allow the Pulse agent to submit OS, CPU, and disk metrics.',
  },
  {
    label: 'Container report',
    scopes: [DOCKER_REPORT_SCOPE],
    description:
      'Permits container agents (Docker or Podman) to stream runtime and container telemetry only.',
  },
  {
    label: 'Container manage',
    scopes: [DOCKER_REPORT_SCOPE, DOCKER_MANAGE_SCOPE],
    description: 'Extends container reporting with lifecycle actions (restart, stop, etc.).',
  },
  {
    label: 'Settings read',
    scopes: [SETTINGS_READ_SCOPE],
    description: 'Read configuration snapshots and diagnostics without modifying anything.',
  },
  {
    label: 'Settings admin',
    scopes: [SETTINGS_READ_SCOPE, SETTINGS_WRITE_SCOPE],
    description: 'Full settings read/write – equivalent to automation with admin privileges.',
  },
];

export const hasAgentScopeResource = (resource: Resource): boolean => {
  if (resource.type === 'docker-host') return false;
  return (
    resource.type === 'agent' ||
    resource.type === 'pbs' ||
    resource.type === 'pmg' ||
    resource.type === 'truenas' ||
    resourceHasAgentFacet(resource)
  );
};

const readPlatformData = (resource: Resource): Record<string, unknown> | undefined => {
  return resource.platformData
    ? (unwrap(resource.platformData) as Record<string, unknown>)
    : undefined;
};

const readPlatformString = (value: unknown): string | undefined => {
  return typeof value === 'string' && value.length > 0 ? value : undefined;
};

const readPlatformNumber = (value: unknown): number | undefined => {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
};

const readNestedPlatformField = (
  platformData: Record<string, unknown> | undefined,
  field: string,
): unknown => {
  if (!platformData) return undefined;
  if (field in platformData) return platformData[field];
  const agent = platformData.agent;
  if (agent && typeof agent === 'object' && field in (agent as Record<string, unknown>)) {
    return (agent as Record<string, unknown>)[field];
  }
  const docker = platformData.docker;
  if (docker && typeof docker === 'object' && field in (docker as Record<string, unknown>)) {
    return (docker as Record<string, unknown>)[field];
  }
  return undefined;
};

export const tokenIdForResource = (resource: Resource): string | undefined => {
  return readPlatformString(readNestedPlatformField(readPlatformData(resource), 'tokenId'));
};

export const agentActionIdForResource = (resource: Resource): string => {
  return getActionableAgentIdFromResource(resource) || resource.id;
};

export const dockerActionIdForResource = (resource: Resource): string => {
  return getActionableDockerRuntimeIdFromResource(resource) || resource.id;
};

export const revokedTokenIdForResource = (resource: Resource): string | undefined => {
  return readPlatformString(readNestedPlatformField(readPlatformData(resource), 'revokedTokenId'));
};

export const tokenRevokedAtForResource = (resource: Resource): number | undefined => {
  return readPlatformNumber(readNestedPlatformField(readPlatformData(resource), 'tokenRevokedAt'));
};

const appendUsageEntry = (
  usage: Map<string, APITokenUsageEntry>,
  tokenId: string,
  item: { id: string; label: string },
) => {
  const previous = usage.get(tokenId);
  if (!previous) {
    usage.set(tokenId, { count: 1, items: [item] });
    return;
  }
  if (previous.items.some((existing) => existing.id === item.id)) return;
  usage.set(tokenId, {
    count: previous.count + 1,
    items: [...previous.items, item],
  });
};

export const buildDockerTokenUsage = (resources: Resource[]): Map<string, APITokenUsageEntry> => {
  const usage = new Map<string, APITokenUsageEntry>();
  for (const resource of resources) {
    const tokenId = tokenIdForResource(resource);
    if (!tokenId) continue;
    appendUsageEntry(usage, tokenId, {
      id: dockerActionIdForResource(resource),
      label: getPreferredResourceDisplayName(resource),
    });
  }
  return usage;
};

export const buildAgentTokenUsage = (resources: Resource[]): Map<string, APITokenUsageEntry> => {
  const usage = new Map<string, APITokenUsageEntry>();
  for (const resource of resources) {
    const tokenId = tokenIdForResource(resource);
    if (!tokenId) continue;
    appendUsageEntry(usage, tokenId, {
      id: agentActionIdForResource(resource),
      label: getPreferredResourceDisplayName(resource),
    });
  }
  return usage;
};

export const sortAPITokensByCreatedAt = (tokens: APITokenRecord[]): APITokenRecord[] => {
  return [...tokens].sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
};

export const countWildcardTokens = (tokens: APITokenRecord[]): number => {
  return tokens.filter((token) => {
    const scopes = token.scopes;
    return !scopes || scopes.length === 0 || scopes.includes(API_TOKEN_WILDCARD_SCOPE);
  }).length;
};

export const groupAPITokenScopes = (): [ScopeGroup, ScopeOption[]][] => {
  const grouped: Record<ScopeGroup, ScopeOption[]> = {
    Monitoring: [],
    Agents: [],
    Settings: [],
  };
  for (const option of API_SCOPE_OPTIONS) {
    grouped[option.group].push(option);
  }
  return API_TOKEN_SCOPE_GROUP_ORDER.map((group) => [group, grouped[group]] as [ScopeGroup, ScopeOption[]]).filter(
    ([, options]) => options.length > 0,
  );
};

export const matchesScopePreset = (selectedScopes: string[], presetScopes: string[]): boolean => {
  const selection = [...selectedScopes]
    .filter((scope) => scope !== API_TOKEN_WILDCARD_SCOPE)
    .sort();
  const target = [...presetScopes].sort();
  if (target.length === 0) {
    return (
      selectedScopes.length === 0 || selectedScopes.includes(API_TOKEN_WILDCARD_SCOPE)
    );
  }
  if (selection.length !== target.length) return false;
  return target.every((scope) => selection.includes(scope));
};

export const getAPITokenHint = (record: APITokenRecord | null | undefined): string => {
  if (!record) return '—';
  if (record.prefix && record.suffix) return `${record.prefix}…${record.suffix}`;
  if (record.prefix) return `${record.prefix}…`;
  return '—';
};

export const getAPITokenDialogName = (record: APITokenRecord): string => {
  if (record.name?.trim()) return record.name.trim();
  if (record.prefix && record.suffix) return `${record.prefix}…${record.suffix}`;
  if (record.prefix) return `${record.prefix}…`;
  return 'untitled token';
};
