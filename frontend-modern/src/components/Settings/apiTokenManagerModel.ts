import { unwrap } from 'solid-js/store';
import type { APITokenRecord } from '@/api/security';
import {
  AUDIT_READ_SCOPE,
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
import {
  API_TOKEN_DOCKER_MANAGE_PRESET_LABEL,
  API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION,
  API_TOKEN_DOCKER_REPORT_PRESET_LABEL,
  API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION,
} from '@/utils/apiTokenPresentation';
import { API_TOKEN_SCOPES_DOC_URL as SHIPPED_API_TOKEN_SCOPES_DOC_URL } from '@/utils/docsLinks';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import type { Resource } from '@/types/resource';

export const API_TOKEN_SCOPES_DOC_URL = SHIPPED_API_TOKEN_SCOPES_DOC_URL;
export const API_TOKEN_WILDCARD_SCOPE = '*';
export const API_TOKEN_KIOSK_PRESET_ID = 'kiosk_monitoring';
export const API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID = 'pulse_intelligence_agent';
export const API_TOKEN_PATROL_EXTERNAL_AGENT_PRESET_LABEL = 'Patrol external agent';
export const API_TOKEN_AGENT_PRESET_ID = 'agent';
export const API_TOKEN_DOCKER_REPORT_PRESET_ID = 'docker_podman_reporting';
export const API_TOKEN_DOCKER_MANAGE_PRESET_ID = 'docker_podman_lifecycle';
export const API_TOKEN_SETTINGS_READ_PRESET_ID = 'settings_read';
export const API_TOKEN_SETTINGS_ADMIN_PRESET_ID = 'settings_admin';
export const API_TOKEN_AUDIT_READ_PRESET_ID = 'audit_read';

export interface APITokenPreset {
  id: string;
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

const API_TOKEN_SCOPE_GROUP_ORDER: ScopeGroup[] = [
  'Monitoring',
  'AI',
  'Agents',
  'Settings',
  'Security',
];

const normalizePresetScopes = (scopes: readonly string[] | undefined): string[] =>
  Array.from(new Set((scopes ?? []).map((scope) => scope.trim()).filter(Boolean)));

export const getAPITokenScopePresets = (
  pulseIntelligenceRequiredScopes: readonly string[] = [],
): APITokenPreset[] => {
  const pulseIntelligenceScopes = normalizePresetScopes(pulseIntelligenceRequiredScopes);
  const presets: APITokenPreset[] = [
    {
      id: API_TOKEN_KIOSK_PRESET_ID,
      label: 'Kiosk / Monitoring',
      scopes: [MONITORING_READ_SCOPE],
      description:
        'Read-only access for wall displays. Use ?token=xxx&kiosk=1 in the URL to hide navigation and filters.',
    },
  ];

  if (pulseIntelligenceScopes.length > 0) {
    presets.push({
      id: API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID,
      label: API_TOKEN_PATROL_EXTERNAL_AGENT_PRESET_LABEL,
      scopes: pulseIntelligenceScopes,
      description: 'Scopes for connected agents that read Pulse context and request Patrol work.',
    });
  }

  presets.push(
    {
      id: API_TOKEN_AGENT_PRESET_ID,
      label: 'Agent',
      scopes: [AGENT_REPORT_SCOPE],
      description: 'Allow the Pulse agent to submit OS, CPU, and disk metrics.',
    },
    {
      id: API_TOKEN_DOCKER_REPORT_PRESET_ID,
      label: API_TOKEN_DOCKER_REPORT_PRESET_LABEL,
      scopes: [DOCKER_REPORT_SCOPE],
      description: API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION,
    },
    {
      id: API_TOKEN_DOCKER_MANAGE_PRESET_ID,
      label: API_TOKEN_DOCKER_MANAGE_PRESET_LABEL,
      scopes: [DOCKER_REPORT_SCOPE, DOCKER_MANAGE_SCOPE],
      description: API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION,
    },
    {
      id: API_TOKEN_SETTINGS_READ_PRESET_ID,
      label: 'Settings read',
      scopes: [SETTINGS_READ_SCOPE],
      description: 'Read configuration snapshots and diagnostics without modifying anything.',
    },
    {
      id: API_TOKEN_SETTINGS_ADMIN_PRESET_ID,
      label: 'Settings admin',
      scopes: [SETTINGS_READ_SCOPE, SETTINGS_WRITE_SCOPE],
      description: 'Full settings read/write - equivalent to automation with admin privileges.',
    },
    {
      id: API_TOKEN_AUDIT_READ_PRESET_ID,
      label: 'Audit read',
      scopes: [AUDIT_READ_SCOPE],
      description: 'Read audit events, verification history, summaries, and export activity.',
    },
  );

  return presets;
};

export const hasAgentScopeResource = (resource: Resource): boolean => {
  if (resource.type === 'docker-host') return false;
  return (
    resource.type === 'agent' ||
    resource.type === 'pbs' ||
    resource.type === 'pmg' ||
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
      label: getPreferredInfrastructureDisplayName(resource),
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
      label: getPreferredInfrastructureDisplayName(resource),
    });
  }
  return usage;
};

export const sortAPITokensByCreatedAt = (tokens: APITokenRecord[]): APITokenRecord[] => {
  return [...tokens].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  );
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
    AI: [],
    Agents: [],
    Settings: [],
    Security: [],
  };
  for (const option of API_SCOPE_OPTIONS) {
    grouped[option.group].push(option);
  }
  return API_TOKEN_SCOPE_GROUP_ORDER.map(
    (group) => [group, grouped[group]] as [ScopeGroup, ScopeOption[]],
  ).filter(([, options]) => options.length > 0);
};

export const matchesScopePreset = (selectedScopes: string[], presetScopes: string[]): boolean => {
  const selection = [...selectedScopes]
    .filter((scope) => scope !== API_TOKEN_WILDCARD_SCOPE)
    .sort();
  const target = [...presetScopes].sort();
  if (target.length === 0) {
    return selectedScopes.length === 0 || selectedScopes.includes(API_TOKEN_WILDCARD_SCOPE);
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
