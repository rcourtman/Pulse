import type { ConnectionType } from '@/api/connections';
import {
  getSourcePlatformManifestEntry,
  type PlatformGovernanceState,
  type PlatformPrimaryMode,
  type PlatformReadinessStage,
  type PlatformSupportFloor,
} from '@/utils/platformSupportManifest';

export type InfrastructureOnboardingConnectionType = Extract<
  ConnectionType,
  'agent' | 'pve' | 'pbs' | 'pmg' | 'truenas' | 'vmware'
>;

export interface InfrastructureOnboardingProductPresentation {
  type: InfrastructureOnboardingConnectionType;
  label: string;
  bestFor: string;
  coverage: string;
  catalogDescription: string;
  sourceStrategy: InfrastructureSourceStrategy;
  autoDetect: boolean;
  governanceState: PlatformGovernanceState;
  readinessStage: PlatformReadinessStage;
  primaryMode: PlatformPrimaryMode;
  canonicalProjections: readonly string[];
  supportFloor: PlatformSupportFloor;
  defaultSurfaceKeys: readonly string[];
}

export interface InfrastructureSourceManagerProductPresentation extends InfrastructureOnboardingProductPresentation {
  actionLabel: string;
}

export interface InfrastructureSourcePickerGroupPresentation {
  id: 'virtualization' | 'storage' | 'backup-mail' | 'host-monitoring';
  label: string;
  description: string;
  types: InfrastructureOnboardingConnectionType[];
}

interface BaseProductPresentation {
  label: string;
  bestFor: string;
  coverage: string;
  catalogDescription: string;
  sourceStrategy: InfrastructureSourceStrategy;
  autoDetect: boolean;
  sourcePlatformId?: string;
  defaultSurfaceKeys: readonly string[];
}

export type InfrastructureSourceStrategy = 'api' | 'agent' | 'api-agent';

export interface InfrastructureSourceStrategyPresentation {
  label: string;
  summary: string;
  detail: string;
}

export interface InfrastructureOnboardingPathPresentation {
  title: string;
  description: string;
  bestFor: string;
  coverage: string;
}

export interface InfrastructureCoverageCompleteActionPresentation {
  label: string;
  detail: string;
}

const SOURCE_STRATEGY_PRESENTATION: Record<
  InfrastructureSourceStrategy,
  InfrastructureSourceStrategyPresentation
> = {
  api: {
    label: 'API inventory',
    summary: 'Platform API',
    detail: 'Uses the platform API for inventory, health, and managed resources.',
  },
  agent: {
    label: 'Agent telemetry',
    summary: 'Pulse Agent',
    detail: 'Installs Pulse Agent for host telemetry, local services, Docker, and Kubernetes.',
  },
  'api-agent': {
    label: 'API first',
    summary: 'Platform API, agent optional',
    detail:
      'Starts with platform API inventory and adds Pulse Agent only where node-local telemetry is needed.',
  },
};

const PRODUCT_PRESENTATION: Record<
  InfrastructureOnboardingConnectionType,
  BaseProductPresentation
> = {
  agent: {
    label: 'Pulse Agent',
    bestFor:
      'Linux, macOS, Windows, FreeBSD, and compatible hosts such as Unraid. Recommended on each machine where you want full node-local telemetry.',
    coverage: 'Low-overhead host telemetry, SMART, services, Docker, and Kubernetes',
    catalogDescription: 'Low-overhead host telemetry, services, Docker, Kubernetes',
    sourceStrategy: 'agent',
    autoDetect: false,
    defaultSurfaceKeys: ['host'],
  },
  vmware: {
    label: 'VMware vCenter',
    bestFor: 'vCenter-managed VMware environments',
    coverage: 'VM inventory, ESXi host health, datastore status',
    catalogDescription: 'VM inventory, ESXi hosts, datastores',
    sourceStrategy: 'api',
    autoDetect: true,
    sourcePlatformId: 'vmware-vsphere',
    defaultSurfaceKeys: ['vms', 'hosts', 'datastores'],
  },
  truenas: {
    label: 'TrueNAS SCALE',
    bestFor: 'TrueNAS appliances with API-backed management',
    coverage: 'Pools, datasets, apps, replications',
    catalogDescription: 'Pools, datasets, apps, replications',
    sourceStrategy: 'api',
    autoDetect: true,
    sourcePlatformId: 'truenas',
    defaultSurfaceKeys: ['datasets', 'pools', 'replication'],
  },
  pve: {
    label: 'Proxmox VE',
    bestFor: 'Virtualization clusters and standalone hypervisors',
    coverage:
      'VMs, containers, storage, and cluster health through the Proxmox API. Install Pulse Agent only on nodes where you want full node-local telemetry such as temperatures and SMART data',
    catalogDescription: 'VMs, containers, storage, cluster health',
    sourceStrategy: 'api-agent',
    autoDetect: true,
    sourcePlatformId: 'proxmox-pve',
    defaultSurfaceKeys: ['vms', 'containers', 'storage', 'backups'],
  },
  pbs: {
    label: 'Proxmox Backup Server',
    bestFor: 'Backup infrastructure and protected storage',
    coverage:
      'Backup jobs, sync, verify, prune, and GC through the Proxmox API. Install Pulse Agent only where host-local telemetry is needed',
    catalogDescription: 'Backup jobs, sync, verify, prune, GC',
    sourceStrategy: 'api-agent',
    autoDetect: true,
    sourcePlatformId: 'proxmox-pbs',
    defaultSurfaceKeys: [
      'backups',
      'datastores',
      'syncJobs',
      'verifyJobs',
      'pruneJobs',
      'garbageJobs',
    ],
  },
  pmg: {
    label: 'Proxmox Mail Gateway',
    bestFor: 'Mail filtering and delivery operations',
    coverage: 'Mail stats, queues, quarantine, relay health',
    catalogDescription: 'Mail stats, queues, quarantine, relay health',
    sourceStrategy: 'api',
    autoDetect: true,
    sourcePlatformId: 'proxmox-pmg',
    defaultSurfaceKeys: ['mailStats', 'queues', 'quarantine', 'domainStats'],
  },
};

const governanceStateForType = (
  type: InfrastructureOnboardingConnectionType,
): PlatformGovernanceState => {
  const sourcePlatformId = PRODUCT_PRESENTATION[type].sourcePlatformId;
  if (!sourcePlatformId) return 'supported';
  return getSourcePlatformManifestEntry(sourcePlatformId)?.governanceState ?? 'supported';
};

const manifestEntryForType = (type: InfrastructureOnboardingConnectionType) => {
  const sourcePlatformId = PRODUCT_PRESENTATION[type].sourcePlatformId;
  return sourcePlatformId ? getSourcePlatformManifestEntry(sourcePlatformId) : null;
};

const API_PRODUCT_ORDER: InfrastructureOnboardingConnectionType[] = [
  'vmware',
  'truenas',
  'pve',
  'pbs',
  'pmg',
];

const SOURCE_MANAGER_PRODUCT_ORDER: InfrastructureOnboardingConnectionType[] = [
  ...API_PRODUCT_ORDER,
  'agent',
];

const SOURCE_MANAGER_LABEL_OVERRIDES: Partial<
  Record<
    InfrastructureOnboardingConnectionType,
    {
      label: string;
      actionLabel: string;
    }
  >
> = {
  agent: {
    label: 'Standalone hosts',
    actionLabel: 'Install Pulse Agent',
  },
};

export const INFRASTRUCTURE_ONBOARDING_PATHS: Record<
  'api' | 'agent',
  InfrastructureOnboardingPathPresentation
> = {
  api: {
    title: 'Connect a supported platform',
    description:
      'Use a management API when the platform exposes one. Pulse validates the endpoint, requests credentials, and then starts collecting platform inventory and health.',
    bestFor: 'TrueNAS, Proxmox, and the current VMware vCenter integration path',
    coverage: 'Platform inventory, workloads, storage, backups, and health',
  },
  agent: {
    title: 'Install Pulse Agent',
    description:
      'Use the agent when you want low-overhead machine telemetry, or when the system does not expose a management API Pulse can connect to directly.',
    bestFor:
      'Linux, macOS, Windows, FreeBSD, and compatible hosts such as Unraid. Recommended on each machine where you want full node-local telemetry.',
    coverage:
      'Low-overhead CPU temperature, disk SMART, services, network metrics, Docker, and Kubernetes telemetry',
  },
};

export const INFRASTRUCTURE_AGENT_DISCOVERY_LABELS = [
  'Pulse Agent hosts',
  'Docker',
  'Kubernetes',
] as const;

export const INFRASTRUCTURE_AGENT_HOST_LABELS = [
  'Linux',
  'macOS',
  'Windows',
  'FreeBSD',
  'Unraid',
] as const;

const SOURCE_PICKER_GROUPS: InfrastructureSourcePickerGroupPresentation[] = [
  {
    id: 'virtualization',
    label: 'Virtualization',
    description: 'Hypervisors, VM inventory, and cluster health.',
    types: ['vmware', 'pve'],
  },
  {
    id: 'storage',
    label: 'Storage',
    description: 'Storage appliances and dataset visibility.',
    types: ['truenas'],
  },
  {
    id: 'backup-mail',
    label: 'Backup and Mail',
    description: 'Backup infrastructure and mail-gateway operations.',
    types: ['pbs', 'pmg'],
  },
  {
    id: 'host-monitoring',
    label: 'Host monitoring',
    description: 'Low-overhead machine telemetry and local service discovery.',
    types: ['agent'],
  },
];

export const getInfrastructureOnboardingProductPresentation = (
  type: InfrastructureOnboardingConnectionType,
): InfrastructureOnboardingProductPresentation => {
  const manifestEntry = manifestEntryForType(type);
  return {
    type,
    ...PRODUCT_PRESENTATION[type],
    governanceState: manifestEntry?.governanceState ?? governanceStateForType(type),
    readinessStage: manifestEntry?.readinessStage ?? 'supported',
    primaryMode: manifestEntry?.primaryMode ?? 'agent-backed',
    canonicalProjections: manifestEntry?.canonicalProjections ?? ['agent'],
    supportFloor:
      manifestEntry?.supportFloor ??
      ({
        setup: 'supported',
        visibility: 'supported',
        workloads: 'n/a',
        storage: 'supported',
        recovery: 'n/a',
        alerts: 'supported',
        assistantRead: 'supported',
        assistantControl: 'supported',
      } satisfies PlatformSupportFloor),
  };
};

export const getInfrastructureSourceStrategyPresentation = (
  strategy: InfrastructureSourceStrategy,
): InfrastructureSourceStrategyPresentation => SOURCE_STRATEGY_PRESENTATION[strategy];

export const getInfrastructureApiProductPresentations =
  (): InfrastructureOnboardingProductPresentation[] =>
    API_PRODUCT_ORDER.map((type) => getInfrastructureOnboardingProductPresentation(type));

export const getInfrastructureSourceManagerProducts =
  (): InfrastructureSourceManagerProductPresentation[] =>
    SOURCE_MANAGER_PRODUCT_ORDER.map((type) => {
      const product = getInfrastructureOnboardingProductPresentation(type);
      const override = SOURCE_MANAGER_LABEL_OVERRIDES[type];
      return {
        ...product,
        label: override?.label ?? product.label,
        actionLabel: override?.actionLabel ?? `Add ${product.label}`,
      };
    });

export const getInfrastructureApiProductsByGovernanceState = (
  governanceState: PlatformGovernanceState,
): InfrastructureOnboardingProductPresentation[] =>
  getInfrastructureApiProductPresentations().filter(
    (product) => product.governanceState === governanceState,
  );

export const getInfrastructureSourcePickerGroups = (): Array<
  InfrastructureSourcePickerGroupPresentation & {
    products: InfrastructureOnboardingProductPresentation[];
  }
> =>
  SOURCE_PICKER_GROUPS.map((group) => ({
    ...group,
    products: group.types.map((type) => getInfrastructureOnboardingProductPresentation(type)),
  }));

export const getInfrastructureAutoDetectLabels = (): string[] =>
  getInfrastructureApiProductPresentations()
    .filter((product) => product.autoDetect)
    .map((product) => product.label);

export const getInfrastructureSupportSummaryBadges = (): {
  supportedToday: string[];
  currentAdmissionPath: string[];
  installPath: string[];
} => ({
  supportedToday: [
    ...getInfrastructureApiProductsByGovernanceState('supported').map((product) => product.label),
    ...INFRASTRUCTURE_AGENT_DISCOVERY_LABELS,
  ],
  currentAdmissionPath: getInfrastructureApiProductsByGovernanceState('admitted').map(
    (product) => product.label,
  ),
  installPath: [...INFRASTRUCTURE_AGENT_HOST_LABELS, ...INFRASTRUCTURE_AGENT_DISCOVERY_LABELS],
});

export const getInfrastructureEmptyStateSummary = (): string =>
  'Choose an infrastructure source to start monitoring your environment.';

export const getInfrastructureEmptyStateDetail = (): string =>
  'Supported source types include VMware vCenter, TrueNAS SCALE, Proxmox VE, Proxmox Backup Server, Proxmox Mail Gateway, and standalone hosts through Pulse Agent. Docker and Kubernetes are discovered from supported agent hosts.';

export const getInfrastructureCoverageCompleteActionPresentation =
  (): InfrastructureCoverageCompleteActionPresentation => ({
    label: 'Coverage coherent',
    detail: 'Coverage looks coherent for the connected systems.',
  });
