import type { Resource } from '@/types/resource';
import { t } from '@/i18n';
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
export type SetupCompletionPrimaryAction = 'infrastructure' | 'sources';

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

const getUnknownSetupSystemName = (): string => t('setup.completion.resource.unknownName');

const isUnknownSetupSystemName = (name: string): boolean =>
  name === 'Unknown' || name === getUnknownSetupSystemName();

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
    name:
      getPreferredInfrastructureDisplayName(resource) ||
      resource.name ||
      getUnknownSetupSystemName(),
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
    if (isUnknownSetupSystemName(existing.name) && !isUnknownSetupSystemName(nextSystem.name)) {
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
  if (hasAgentConnectedSystems && hasApiConnectedSystems) {
    return t('setup.completion.hero.connected.description.both');
  }
  if (hasApiConnectedSystems) {
    return t('setup.completion.hero.connected.description.api');
  }
  return t('setup.completion.hero.connected.description.agent');
};

const buildCredentialsContinuationText = (
  hasAgentConnectedSystems: boolean,
  hasApiConnectedSystems: boolean,
): string => {
  return hasAgentConnectedSystems || hasApiConnectedSystems
    ? t('setup.completion.credentials.continuation.connected')
    : t('setup.completion.credentials.continuation.empty');
};

const buildConnectedNextStepDetail = (
  hasAgentConnectedSystems: boolean,
  hasApiConnectedSystems: boolean,
): string => {
  if (hasAgentConnectedSystems && hasApiConnectedSystems) {
    return t('setup.completion.nextStep.detail.both');
  }
  if (hasApiConnectedSystems) {
    return t('setup.completion.nextStep.detail.api');
  }
  return t('setup.completion.nextStep.detail.agent');
};

export function buildSetupCompletionViewModel(
  connectedSystems: readonly ConnectedSetupSystem[],
): SetupCompletionViewModel {
  const hasConnectedSystems = connectedSystems.length > 0;
  const hasAgentConnectedSystems = connectedSystems.some(
    (system) => system.connectionPath === 'agent',
  );
  const hasApiConnectedSystems = connectedSystems.some((system) => system.connectionPath === 'api');

  const nextStepSummary =
    connectedSystems.length === 1
      ? t('setup.completion.nextStep.summary.connected.singular')
      : t('setup.completion.nextStep.summary.connected.plural');

  if (!hasConnectedSystems) {
    return {
      connectedSummaryLabel: t('setup.completion.connectedSummary.plural', { count: 0 }),
      credentialsContinuationText: buildCredentialsContinuationText(
        hasAgentConnectedSystems,
        hasApiConnectedSystems,
      ),
      hasConnectedSystems,
      hasAgentConnectedSystems,
      hasApiConnectedSystems,
      heroDescription: t('setup.completion.hero.empty.description'),
      heroTitle: t('setup.completion.hero.empty.title'),
      nextStepDetail: t('setup.completion.nextStep.detail.empty'),
      nextStepSummary: t('setup.completion.nextStep.summary.empty'),
      nextStepTitle: t('setup.completion.nextStep.title.empty'),
      primaryAction: 'sources',
      showAddInfrastructureAction: false,
      showAgentInstallAction: true,
    };
  }

  return {
    connectedSummaryLabel: t(
      connectedSystems.length === 1
        ? 'setup.completion.connectedSummary.singular'
        : 'setup.completion.connectedSummary.plural',
      { count: connectedSystems.length },
    ),
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
    heroTitle: t('setup.completion.hero.connected.title'),
    nextStepDetail: buildConnectedNextStepDetail(hasAgentConnectedSystems, hasApiConnectedSystems),
    nextStepSummary,
    nextStepTitle: t('setup.completion.nextStep.title.connected'),
    primaryAction: 'infrastructure',
    showAddInfrastructureAction: hasConnectedSystems,
    showAgentInstallAction: false,
  };
}
