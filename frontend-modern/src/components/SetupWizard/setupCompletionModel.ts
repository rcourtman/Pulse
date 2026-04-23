import type { Resource } from '@/types/resource';
import {
  getActionableAgentIdFromResource,
  getPlatformDataRecord,
  isAgentFacetInfrastructureResource,
  isTrueNASSystemResource,
} from '@/utils/agentResources';
import {
  getSourcePlatformManifestEntry,
  sourcePlatformSupportsOnboardingPath,
} from '@/utils/platformSupportManifest';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';

export type SetupCompletionConnectionPath = 'agent' | 'api';
export type SetupCompletionPrimaryAction = 'dashboard' | 'sources';

export interface ConnectedSetupSystem {
  id: string;
  name: string;
  typeLabel: string;
  host: string;
  connectionPath: SetupCompletionConnectionPath;
}

export interface SetupCompletionViewModel {
  connectedSummaryLabel: string;
  credentialsContinuationText: string;
  hasConnectedSystems: boolean;
  hasAgentConnectedSystems: boolean;
  hasApiConnectedSystems: boolean;
  heroDescription: string;
  heroTitle: string;
  nextStepDetail: string;
  nextStepSummary: string;
  nextStepTitle: string;
  primaryAction: SetupCompletionPrimaryAction;
  showAddInfrastructureAction: boolean;
  showAgentInstallAction: boolean;
}

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const isSetupCompletionInfrastructureResource = (resource: Resource): boolean =>
  resource.type === 'agent' || resource.type === 'pbs' || resource.type === 'pmg';

const getSetupCompletionPlatformKey = (resource: Resource): string | null => {
  if (resource.type === 'pbs') return 'proxmox-pbs';
  if (resource.type === 'pmg') return 'proxmox-pmg';
  if (isTrueNASSystemResource(resource)) return 'truenas';
  return resource.platformType || null;
};

const getSetupCompletionPlatformLabel = (resource: Resource): string | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(getSetupCompletionPlatformKey(resource));
  if (!manifestPlatform) return null;

  const displayTokens = manifestPlatform.displayTokens as readonly string[];
  return displayTokens[displayTokens.length - 1] || manifestPlatform.uiLabel;
};

const isApiConnectedSetupResource = (resource: Resource): boolean =>
  sourcePlatformSupportsOnboardingPath(
    getSetupCompletionPlatformKey(resource),
    'platform-connections',
  );

const isAgentConnectedSetupResource = (resource: Resource): boolean =>
  isAgentFacetInfrastructureResource(resource) && !isApiConnectedSetupResource(resource);

const getConnectedSetupSystemTypeLabel = (resource: Resource): string => {
  return getSetupCompletionPlatformLabel(resource) || 'Agent';
};

const getConnectedSetupSystemHost = (resource: Resource): string => {
  const hostname = getPreferredResourceHostname(resource);
  if (hostname) return hostname;

  const platformData = getPlatformDataRecord(resource);
  const proxmox = asRecord(platformData?.proxmox);
  const truenas = asRecord(platformData?.truenas);
  const vmware = asRecord(platformData?.vmware);

  return (
    asString(proxmox?.instance) || asString(truenas?.hostname) || asString(vmware?.hostname) || ''
  );
};

const toConnectedSetupSystem = (resource: Resource): ConnectedSetupSystem | null => {
  if (!isSetupCompletionInfrastructureResource(resource)) return null;

  const connectionPath = isApiConnectedSetupResource(resource)
    ? 'api'
    : isAgentConnectedSetupResource(resource)
      ? 'agent'
      : null;
  if (!connectionPath) return null;

  return {
    id: resource.platformId || getActionableAgentIdFromResource(resource) || resource.id,
    name: getPreferredInfrastructureDisplayName(resource) || resource.name || 'Unknown',
    typeLabel: getConnectedSetupSystemTypeLabel(resource),
    host: getConnectedSetupSystemHost(resource),
    connectionPath,
  };
};

export function buildSetupCompletionConnectedSystems(
  resources: readonly Resource[],
): ConnectedSetupSystem[] {
  const systems = new Map<string, ConnectedSetupSystem>();

  for (const resource of resources) {
    const nextSystem = toConnectedSetupSystem(resource);
    if (!nextSystem) continue;

    const key = resource.platformId || nextSystem.id || nextSystem.name;
    const existing = systems.get(key);
    if (!existing) {
      systems.set(key, nextSystem);
      continue;
    }

    if (existing.connectionPath === 'agent' && nextSystem.connectionPath === 'api') {
      existing.connectionPath = 'api';
      existing.typeLabel = nextSystem.typeLabel;
    }
    if (!existing.host && nextSystem.host) {
      existing.host = nextSystem.host;
    }
    if (existing.name === 'Unknown' && nextSystem.name !== 'Unknown') {
      existing.name = nextSystem.name;
    }
  }

  return Array.from(systems.values()).sort(
    (left, right) => left.name.localeCompare(right.name) || left.id.localeCompare(right.id),
  );
}

const buildConnectedHeroDescription = (
  hasAgentConnectedSystems: boolean,
  hasApiConnectedSystems: boolean,
): string => {
  const prefix =
    'Your admin account is ready and Pulse is already receiving telemetry. Open the dashboard to verify the first overview';

  if (hasAgentConnectedSystems && hasApiConnectedSystems) {
    return `${prefix}, then return to Add infrastructure when you want another platform API or Agent source.`;
  }
  if (hasApiConnectedSystems) {
    return `${prefix}, then return to Add infrastructure when you want another platform API or Pulse Agent source.`;
  }
  return `${prefix}, then return to Add infrastructure when you want another Pulse Agent or platform API source.`;
};

const buildCredentialsContinuationText = (
  _hasAgentConnectedSystems: boolean,
  _hasApiConnectedSystems: boolean,
): string => {
  return 'the dashboard or Add infrastructure.';
};

const buildConnectedNextStepDetail = (
  hasAgentConnectedSystems: boolean,
  hasApiConnectedSystems: boolean,
): string => {
  if (hasAgentConnectedSystems && hasApiConnectedSystems) {
    return 'Add infrastructure stays available any time you want to expand from this first system with another API source, Agent source, or both.';
  }
  if (hasApiConnectedSystems) {
    return 'Add infrastructure stays available for more API-backed systems or Pulse Agent telemetry when a system needs node-local coverage.';
  }
  return 'Add infrastructure stays available for more Pulse Agent systems or platform API inventory when a platform manages the estate.';
};

export function buildSetupCompletionViewModel(
  connectedSystems: readonly ConnectedSetupSystem[],
): SetupCompletionViewModel {
  const hasConnectedSystems = connectedSystems.length > 0;
  const hasAgentConnectedSystems = connectedSystems.some(
    (system) => system.connectionPath === 'agent',
  );
  const hasApiConnectedSystems = connectedSystems.some((system) => system.connectionPath === 'api');

  const connectedSystemNoun = connectedSystems.length === 1 ? 'system' : 'systems';
  const nextStepSummary =
    connectedSystems.length === 1
      ? 'Open the dashboard to review your first connected system.'
      : 'Open the dashboard to review your connected systems.';

  if (!hasConnectedSystems) {
    return {
      connectedSummaryLabel: 'Connected (0 systems)',
      credentialsContinuationText: 'Add infrastructure.',
      hasConnectedSystems,
      hasAgentConnectedSystems,
      hasApiConnectedSystems,
      heroDescription:
        'Your admin account is ready. Next, choose how the first system should enter the unified infrastructure model: platform API inventory, Pulse Agent telemetry, or both.',
      heroTitle: 'Choose your first infrastructure source',
      nextStepDetail:
        'Start with a platform API when a platform manages the estate. Install Pulse Agent when the system itself should report node-local telemetry.',
      nextStepSummary: 'Open Add infrastructure to choose a platform API, Pulse Agent, or both.',
      nextStepTitle: 'Choose the first source strategy',
      primaryAction: 'sources',
      showAddInfrastructureAction: false,
      showAgentInstallAction: true,
    };
  }

  return {
    connectedSummaryLabel: `Connected (${connectedSystems.length} ${connectedSystemNoun})`,
    credentialsContinuationText: buildCredentialsContinuationText(
      hasAgentConnectedSystems,
      hasApiConnectedSystems,
    ),
    hasConnectedSystems,
    hasAgentConnectedSystems,
    hasApiConnectedSystems,
    heroDescription: buildConnectedHeroDescription(
      hasAgentConnectedSystems,
      hasApiConnectedSystems,
    ),
    heroTitle: 'First monitored system connected',
    nextStepDetail: buildConnectedNextStepDetail(hasAgentConnectedSystems, hasApiConnectedSystems),
    nextStepSummary,
    nextStepTitle: 'Open your first dashboard view',
    primaryAction: 'dashboard',
    showAddInfrastructureAction: hasConnectedSystems,
    showAgentInstallAction: false,
  };
}
