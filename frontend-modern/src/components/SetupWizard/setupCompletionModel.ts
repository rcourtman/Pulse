import type { PlatformType, Resource } from '@/types/resource';
import {
  getActionableAgentIdFromResource,
  getPlatformDataRecord,
  isAgentFacetInfrastructureResource,
  isTrueNASSystemResource,
} from '@/utils/agentResources';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';

export type SetupCompletionConnectionPath = 'install' | 'platforms';
export type SetupCompletionPrimaryAction = 'dashboard' | 'install';

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
  hasInstallConnectedSystems: boolean;
  hasPlatformConnectedSystems: boolean;
  heroDescription: string;
  heroTitle: string;
  nextStepDetail: string;
  nextStepSummary: string;
  nextStepTitle: string;
  primaryAction: SetupCompletionPrimaryAction;
  showInstallAction: boolean;
  showPlatformConnectionsAction: boolean;
}

const PLATFORM_CONNECTION_PLATFORM_TYPES = new Set<PlatformType>([
  'proxmox-pve',
  'proxmox-pbs',
  'proxmox-pmg',
  'truenas',
  'vmware-vsphere',
]);

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const isSetupCompletionInfrastructureResource = (resource: Resource): boolean =>
  resource.type === 'agent' || resource.type === 'pbs' || resource.type === 'pmg';

const isPlatformConnectedSetupResource = (resource: Resource): boolean =>
  resource.type === 'pbs' ||
  resource.type === 'pmg' ||
  PLATFORM_CONNECTION_PLATFORM_TYPES.has(resource.platformType) ||
  isTrueNASSystemResource(resource);

const isInstallConnectedSetupResource = (resource: Resource): boolean =>
  isAgentFacetInfrastructureResource(resource) && !isPlatformConnectedSetupResource(resource);

const getConnectedSetupSystemTypeLabel = (resource: Resource): string => {
  if (resource.type === 'pbs' || resource.platformType === 'proxmox-pbs') {
    return 'Proxmox Backup Server';
  }
  if (resource.type === 'pmg' || resource.platformType === 'proxmox-pmg') {
    return 'Proxmox Mail Gateway';
  }
  if (resource.platformType === 'proxmox-pve') {
    return 'Proxmox VE';
  }
  if (resource.platformType === 'truenas' || isTrueNASSystemResource(resource)) {
    return 'TrueNAS';
  }
  if (resource.platformType === 'vmware-vsphere') {
    return 'VMware vSphere';
  }
  return 'Agent';
};

const getConnectedSetupSystemHost = (resource: Resource): string => {
  const hostname = getPreferredResourceHostname(resource);
  if (hostname) return hostname;

  const platformData = getPlatformDataRecord(resource);
  const proxmox = asRecord(platformData?.proxmox);
  const truenas = asRecord(platformData?.truenas);
  const vmware = asRecord(platformData?.vmware);

  return (
    asString(proxmox?.instance) ||
    asString(truenas?.hostname) ||
    asString(vmware?.hostname) ||
    ''
  );
};

const toConnectedSetupSystem = (resource: Resource): ConnectedSetupSystem | null => {
  if (!isSetupCompletionInfrastructureResource(resource)) return null;

  const connectionPath = isPlatformConnectedSetupResource(resource)
    ? 'platforms'
    : isInstallConnectedSetupResource(resource)
      ? 'install'
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

    if (existing.connectionPath === 'install' && nextSystem.connectionPath === 'platforms') {
      existing.connectionPath = 'platforms';
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
  hasInstallConnectedSystems: boolean,
  hasPlatformConnectedSystems: boolean,
): string => {
  const prefix =
    'Your admin account is ready and Pulse is already receiving telemetry. Open the dashboard to verify the first overview';

  if (hasInstallConnectedSystems && hasPlatformConnectedSystems) {
    return `${prefix}, then return to Platform connections or Infrastructure Install when you want to add more systems.`;
  }
  if (hasPlatformConnectedSystems) {
    return `${prefix}, then return to Platform connections when you want to add more API-backed systems.`;
  }
  return `${prefix}, then return to Infrastructure Install when you want to add more host-installed systems.`;
};

const buildCredentialsContinuationText = (
  hasInstallConnectedSystems: boolean,
  hasPlatformConnectedSystems: boolean,
): string => {
  if (hasInstallConnectedSystems && hasPlatformConnectedSystems) {
    return 'the dashboard, Platform connections, or Infrastructure Install.';
  }
  if (hasPlatformConnectedSystems) {
    return 'the dashboard, Platform connections, or Infrastructure Install.';
  }
  return 'the dashboard or Infrastructure Install.';
};

const buildConnectedNextStepDetail = (
  hasInstallConnectedSystems: boolean,
  hasPlatformConnectedSystems: boolean,
): string => {
  if (hasInstallConnectedSystems && hasPlatformConnectedSystems) {
    return 'Platform connections and Infrastructure Install both stay available any time you want to expand from this first system.';
  }
  if (hasPlatformConnectedSystems) {
    return 'Platform connections stays available any time you want to add more API-backed systems, and Infrastructure Install is ready when the next system should run the unified agent.';
  }
  return 'Infrastructure Install stays available any time you want to add more host-installed systems.';
};

export function buildSetupCompletionViewModel(
  connectedSystems: readonly ConnectedSetupSystem[],
): SetupCompletionViewModel {
  const hasConnectedSystems = connectedSystems.length > 0;
  const hasInstallConnectedSystems = connectedSystems.some(
    (system) => system.connectionPath === 'install',
  );
  const hasPlatformConnectedSystems = connectedSystems.some(
    (system) => system.connectionPath === 'platforms',
  );

  const connectedSystemNoun = connectedSystems.length === 1 ? 'system' : 'systems';
  const nextStepSummary =
    connectedSystems.length === 1
      ? 'Open the dashboard to review your first connected system.'
      : 'Open the dashboard to review your connected systems.';

  if (!hasConnectedSystems) {
    return {
      connectedSummaryLabel: 'Connected (0 systems)',
      credentialsContinuationText: 'Infrastructure Install or Platform connections.',
      hasConnectedSystems,
      hasInstallConnectedSystems,
      hasPlatformConnectedSystems,
      heroDescription:
        'Your admin account is ready. Next, choose the first infrastructure path: open Infrastructure Install for a host that should run the unified agent, or open Platform connections for API-backed platforms such as Proxmox, TrueNAS, and VMware.',
      heroTitle: 'Connect your first monitored system',
      nextStepDetail:
        'If the first system is API-backed, use Platform connections instead of starting with host install.',
      nextStepSummary: 'Open Infrastructure Install to bring your first monitored system into Pulse.',
      nextStepTitle: 'Choose your first infrastructure path',
      primaryAction: 'install',
      showInstallAction: false,
      showPlatformConnectionsAction: true,
    };
  }

  return {
    connectedSummaryLabel: `Connected (${connectedSystems.length} ${connectedSystemNoun})`,
    credentialsContinuationText: buildCredentialsContinuationText(
      hasInstallConnectedSystems,
      hasPlatformConnectedSystems,
    ),
    hasConnectedSystems,
    hasInstallConnectedSystems,
    hasPlatformConnectedSystems,
    heroDescription: buildConnectedHeroDescription(
      hasInstallConnectedSystems,
      hasPlatformConnectedSystems,
    ),
    heroTitle: 'First monitored system connected',
    nextStepDetail: buildConnectedNextStepDetail(
      hasInstallConnectedSystems,
      hasPlatformConnectedSystems,
    ),
    nextStepSummary,
    nextStepTitle: 'Open your first dashboard view',
    primaryAction: 'dashboard',
    showInstallAction: hasConnectedSystems,
    showPlatformConnectionsAction: hasPlatformConnectedSystems,
  };
}
