import type { ConnectionType } from '@/api/connections';
import {
  SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES,
  getSourcePlatformManifestEntry,
  type PlatformGovernanceState,
  type PlatformPrimaryMode,
  type PlatformReadinessStage,
  type PlatformSupportFloor,
} from '@/utils/platformSupportManifest';

export type InfrastructureOnboardingConnectionType = Extract<
  ConnectionType,
  'agent' | 'availability' | 'pve' | 'pbs' | 'pmg' | 'truenas' | 'vmware'
>;

export interface InfrastructureOnboardingProductPresentation {
  type: InfrastructureOnboardingConnectionType;
  label: string;
  bestFor: string;
  coverage: string;
  catalogDescription: string;
  searchAliases: readonly string[];
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

export type InfrastructureSourcePickerItemId =
  | 'vmware'
  | 'pve'
  | 'truenas'
  | 'unraid'
  | 'pbs'
  | 'pmg'
  | 'linux-host'
  | 'docker'
  | 'kubernetes'
  | 'availability';

export type InfrastructureSourcePickerRouteStep = InfrastructureSourcePickerItemId;

export interface InfrastructureSourcePickerItemPresentation {
  id: InfrastructureSourcePickerItemId;
  routeStep: InfrastructureSourcePickerRouteStep;
  connectionType: InfrastructureOnboardingConnectionType;
  label: string;
  bestFor: string;
  coverage: string;
  catalogDescription: string;
  searchAliases: readonly string[];
  sourceStrategy: InfrastructureSourceStrategy;
  autoDetect: boolean;
  governanceState: PlatformGovernanceState;
  readinessStage: PlatformReadinessStage;
  primaryMode: PlatformPrimaryMode;
  canonicalProjections: readonly string[];
  supportFloor: PlatformSupportFloor;
}

interface BaseProductPresentation {
  label: string;
  bestFor: string;
  coverage: string;
  catalogDescription: string;
  searchAliases?: readonly string[];
  sourceStrategy: InfrastructureSourceStrategy;
  autoDetect: boolean;
  sourcePlatformId?: string;
  primaryMode?: PlatformPrimaryMode;
  canonicalProjections?: readonly string[];
  defaultSurfaceKeys: readonly string[];
}

export type InfrastructureSourceStrategy = 'api' | 'agent' | 'api-agent' | 'probe';

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

const INFRASTRUCTURE_AGENT_RUNTIME_PLATFORM_LABELS = [
  'Linux',
  'macOS',
  'Windows',
  'FreeBSD',
] as const;

const formatJoinedLabelList = (labels: readonly string[]): string => {
  if (labels.length === 0) return '';
  if (labels.length === 1) return labels[0];
  if (labels.length === 2) return `${labels[0]} and ${labels[1]}`;
  return `${labels.slice(0, -1).join(', ')}, and ${labels[labels.length - 1]}`;
};

const getInfrastructureAgentHostProfileLabels = (): readonly string[] => {
  const labels = [
    ...INFRASTRUCTURE_AGENT_RUNTIME_PLATFORM_LABELS,
    ...SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES.filter(
      (profile) => profile.governanceState === 'supported',
    ).map((profile) => profile.family),
  ];
  return Array.from(new Set(labels));
};

export const getInfrastructureAgentHostProfileSupportText = (): string =>
  `${formatJoinedLabelList(getInfrastructureAgentHostProfileLabels())} host/appliance profiles`;

export const getInfrastructureGovernanceBadgeLabel = (
  governanceState: PlatformGovernanceState,
  readinessStage: PlatformReadinessStage,
): string | null => {
  if (governanceState === 'supported') return null;
  if (governanceState === 'presentation-only') return 'Presentation only';
  if (readinessStage === 'first-lab-ready') return 'Preview';
  return 'Preview';
};

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
    detail: `Installs Pulse Agent for ${getInfrastructureAgentHostProfileSupportText()}, local services, Docker, and Kubernetes.`,
  },
  'api-agent': {
    label: 'API first',
    summary: 'Platform API, agent optional',
    detail:
      'Starts with platform API inventory and adds Pulse Agent only where node-local telemetry is needed.',
  },
  probe: {
    label: 'Availability probe',
    summary: 'Agentless probe',
    detail: 'Uses ICMP, TCP, or HTTP checks for devices that cannot run Pulse Agent.',
  },
};

const PRODUCT_PRESENTATION: Record<
  InfrastructureOnboardingConnectionType,
  BaseProductPresentation
> = {
  agent: {
    label: 'Pulse Agent',
    bestFor: `${getInfrastructureAgentHostProfileSupportText()} where you want low-overhead node-local telemetry.`,
    coverage: 'Low-overhead host telemetry, SMART, services, Docker, and Kubernetes',
    catalogDescription: 'Low-overhead host telemetry, services, Docker, Kubernetes',
    searchAliases: ['host', 'server', 'machine', 'node', 'ubuntu', 'debian', 'windows', 'mac'],
    sourceStrategy: 'agent',
    autoDetect: false,
    defaultSurfaceKeys: ['host'],
  },
  vmware: {
    label: 'VMware vCenter',
    bestFor: 'vCenter-managed VMware environments',
    coverage: 'VM inventory, ESXi host health, datastore status',
    catalogDescription: 'VM inventory, ESXi hosts, datastores',
    searchAliases: ['vsphere', 'esxi', 'vcenter', 'vmware cluster'],
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
    searchAliases: ['nas', 'storage', 'zfs', 'truenas scale'],
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
    searchAliases: ['proxmox', 'pve', 'hypervisor', 'vm host', 'cluster'],
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
    searchAliases: ['backup', 'proxmox backup', 'pbs'],
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
    searchAliases: ['mail', 'email', 'gateway', 'proxmox mail', 'pmg'],
    sourceStrategy: 'api',
    autoDetect: true,
    sourcePlatformId: 'proxmox-pmg',
    defaultSurfaceKeys: ['mailStats', 'queues', 'quarantine', 'domainStats'],
  },
  availability: {
    label: 'Network endpoint',
    bestFor: 'Devices that expose ICMP, TCP, or HTTP but cannot run Pulse Agent',
    coverage: 'Agentless availability checks and downtime alerts',
    catalogDescription: 'Ping, TCP port, and HTTP availability checks',
    searchAliases: [
      'ping',
      'icmp',
      'tcp',
      'http',
      'endpoint',
      'probe',
      'port',
      'website',
      'ip address',
      'mqtt',
      'esphome',
    ],
    sourceStrategy: 'probe',
    autoDetect: false,
    primaryMode: 'api-backed',
    canonicalProjections: ['network-endpoint'],
    defaultSurfaceKeys: ['availability'],
  },
};

const DEFAULT_AGENT_SUPPORT_FLOOR = {
  setup: 'supported',
  visibility: 'supported',
  workloads: 'n/a',
  storage: 'supported',
  recovery: 'n/a',
  alerts: 'supported',
  assistantRead: 'supported',
  assistantControl: 'supported',
} satisfies PlatformSupportFloor;

const AGENT_CATALOG_ITEMS: Record<
  'linux-host' | 'unraid' | 'docker' | 'kubernetes',
  Omit<
    InfrastructureSourcePickerItemPresentation,
    | 'id'
    | 'connectionType'
    | 'governanceState'
    | 'readinessStage'
    | 'primaryMode'
    | 'canonicalProjections'
    | 'supportFloor'
  >
> = {
  'linux-host': {
    routeStep: 'linux-host',
    label: 'Linux, macOS, Windows host',
    bestFor: 'Standalone machines where you want low-overhead node-local telemetry.',
    coverage: 'Host telemetry, services, SMART, sensors, network metrics, and remote update state',
    catalogDescription: 'Machine telemetry, services, SMART, sensors',
    searchAliases: ['host', 'server', 'machine', 'node', 'ubuntu', 'debian', 'freebsd', 'windows'],
    sourceStrategy: 'agent',
    autoDetect: false,
  },
  unraid: {
    routeStep: 'unraid',
    label: 'Unraid',
    bestFor: 'Unraid servers where array health and host-local telemetry both matter.',
    coverage: 'Array health, disk posture, SMART, services, Docker containers, and host telemetry',
    catalogDescription: 'Array health, disks, Docker, host telemetry',
    searchAliases: ['nas', 'array', 'storage', 'home server', 'docker host'],
    sourceStrategy: 'agent',
    autoDetect: false,
  },
  docker: {
    routeStep: 'docker',
    label: 'Docker',
    bestFor: 'Docker hosts where containers should be discovered from the machine running them.',
    coverage:
      'Docker inventory plus host CPU, memory, disk, network, services, and SMART telemetry',
    catalogDescription: 'Containers plus host telemetry',
    searchAliases: ['containers', 'container host', 'compose'],
    sourceStrategy: 'agent',
    autoDetect: false,
  },
  kubernetes: {
    routeStep: 'kubernetes',
    label: 'Kubernetes',
    bestFor: 'Small clusters and lab nodes where Pulse Agent can collect cluster and node context.',
    coverage: 'Kubernetes workload context plus node-local host telemetry from the agent',
    catalogDescription: 'Workload context plus node telemetry',
    searchAliases: ['k8s', 'pods', 'cluster', 'workloads'],
    sourceStrategy: 'agent',
    autoDetect: false,
  },
};

const SOURCE_PICKER_PRODUCT_ITEMS: Partial<
  Record<InfrastructureSourcePickerItemId, InfrastructureOnboardingConnectionType>
> = {
  vmware: 'vmware',
  pve: 'pve',
  truenas: 'truenas',
  pbs: 'pbs',
  pmg: 'pmg',
  availability: 'availability',
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
  'availability',
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
    title: 'Connect platform API',
    description:
      'Use a management API when the platform exposes one. Pulse validates the endpoint, requests credentials, and then starts collecting platform inventory and health.',
    bestFor: 'TrueNAS, Proxmox, and the current VMware vCenter integration path',
    coverage: 'Platform inventory, workloads, storage, backups, and health',
  },
  agent: {
    title: 'Install Pulse Agent',
    description: `Use the agent when you want low-overhead machine telemetry for ${getInfrastructureAgentHostProfileSupportText()}, or when the system does not expose a management API Pulse can connect to directly.`,
    bestFor: `${getInfrastructureAgentHostProfileSupportText()} where you want full node-local telemetry.`,
    coverage:
      'Low-overhead CPU temperature, disk SMART, services, network metrics, Docker, and Kubernetes telemetry',
  },
};

export const INFRASTRUCTURE_AGENT_DISCOVERY_LABELS = [
  'Pulse Agent hosts',
  'Docker',
  'Kubernetes',
] as const;

export const INFRASTRUCTURE_AGENT_HOST_LABELS = getInfrastructureAgentHostProfileLabels();

const SOURCE_PICKER_ITEM_ORDER: InfrastructureSourcePickerItemId[] = [
  // Proxmox suite kept adjacent so a Proxmox-heavy lab finds PVE, PBS, and
  // PMG without scanning past unrelated cards.
  'pve',
  'pbs',
  'pmg',
  // Other platform-API integrations.
  'truenas',
  'vmware',
  // Agent-install paths, named platforms first then the generic host card.
  'unraid',
  'docker',
  'kubernetes',
  'linux-host',
  // Endpoint probe last; it is a fallback for hosts that expose neither an
  // API nor an agent.
  'availability',
];

export const getInfrastructureOnboardingProductPresentation = (
  type: InfrastructureOnboardingConnectionType,
): InfrastructureOnboardingProductPresentation => {
  const manifestEntry = manifestEntryForType(type);
  const presentation = PRODUCT_PRESENTATION[type];
  return {
    type,
    ...presentation,
    searchAliases: presentation.searchAliases ?? [],
    governanceState: manifestEntry?.governanceState ?? governanceStateForType(type),
    readinessStage: manifestEntry?.readinessStage ?? 'supported',
    primaryMode: manifestEntry?.primaryMode ?? presentation.primaryMode ?? 'agent-backed',
    canonicalProjections: manifestEntry?.canonicalProjections ??
      presentation.canonicalProjections ?? ['agent'],
    supportFloor: manifestEntry?.supportFloor ?? DEFAULT_AGENT_SUPPORT_FLOOR,
  };
};

export const getInfrastructureSourcePickerItemPresentation = (
  id: InfrastructureSourcePickerItemId,
): InfrastructureSourcePickerItemPresentation => {
  const productType = SOURCE_PICKER_PRODUCT_ITEMS[id];
  if (productType) {
    const product = getInfrastructureOnboardingProductPresentation(productType);
    return {
      id,
      routeStep: id,
      connectionType: product.type,
      label: product.label,
      bestFor: product.bestFor,
      coverage: product.coverage,
      catalogDescription: product.catalogDescription,
      sourceStrategy: product.sourceStrategy,
      autoDetect: product.autoDetect,
      searchAliases: product.searchAliases,
      governanceState: product.governanceState,
      readinessStage: product.readinessStage,
      primaryMode: product.primaryMode,
      canonicalProjections: product.canonicalProjections,
      supportFloor: product.supportFloor,
    };
  }

  const agentItem = AGENT_CATALOG_ITEMS[id as keyof typeof AGENT_CATALOG_ITEMS];
  return {
    id,
    ...agentItem,
    connectionType: 'agent',
    governanceState: 'supported',
    readinessStage: 'supported',
    primaryMode: 'agent-backed',
    canonicalProjections: ['agent'],
    supportFloor: DEFAULT_AGENT_SUPPORT_FLOOR,
  };
};

export const getInfrastructureSourcePickerItemForRouteStep = (
  step: string | null | undefined,
): InfrastructureSourcePickerItemPresentation | null => {
  const normalized = (step || '').trim();
  if (!normalized) return null;
  for (const id of SOURCE_PICKER_ITEM_ORDER) {
    const item = getInfrastructureSourcePickerItemPresentation(id);
    if (item.routeStep === normalized) return item;
  }
  return null;
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

export const getInfrastructureSourcePickerItems =
  (): InfrastructureSourcePickerItemPresentation[] =>
    SOURCE_PICKER_ITEM_ORDER.map((id) => getInfrastructureSourcePickerItemPresentation(id));

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
  'Supported source types include TrueNAS SCALE, Proxmox VE, Proxmox Backup Server, Proxmox Mail Gateway, network endpoints, and standalone hosts through Pulse Agent. VMware vCenter is available as a preview platform pending live support proof. Docker and Kubernetes are discovered from supported agent hosts.';

export const getInfrastructureCoverageCompleteActionPresentation =
  (): InfrastructureCoverageCompleteActionPresentation => ({
    label: 'Coverage coherent',
    detail: 'Coverage looks coherent for the connected systems.',
  });
