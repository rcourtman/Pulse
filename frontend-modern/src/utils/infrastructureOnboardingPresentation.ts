import type { ConnectionType } from '@/api/connections';
import {
  getSourcePlatformManifestEntry,
  type PlatformGovernanceState,
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
  autoDetect: boolean;
  governanceState: PlatformGovernanceState;
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
  autoDetect: boolean;
  sourcePlatformId?: string;
}

export interface InfrastructureOnboardingPathPresentation {
  title: string;
  description: string;
  bestFor: string;
  coverage: string;
}

const PRODUCT_PRESENTATION: Record<
  InfrastructureOnboardingConnectionType,
  BaseProductPresentation
> = {
  agent: {
    label: 'Pulse Agent',
    bestFor: 'Linux, FreeBSD, and compatible hosts such as Unraid',
    coverage: 'Host telemetry, SMART, services, Docker, Kubernetes',
    catalogDescription: 'Host telemetry, services, Docker, Kubernetes',
    autoDetect: false,
  },
  vmware: {
    label: 'VMware vCenter',
    bestFor: 'vCenter-managed VMware environments',
    coverage: 'VM inventory, ESXi host health, datastore status',
    catalogDescription: 'VM inventory, ESXi hosts, datastores',
    autoDetect: true,
    sourcePlatformId: 'vmware-vsphere',
  },
  truenas: {
    label: 'TrueNAS SCALE',
    bestFor: 'TrueNAS appliances with API-backed management',
    coverage: 'Pools, datasets, apps, replications',
    catalogDescription: 'Pools, datasets, apps, replications',
    autoDetect: true,
    sourcePlatformId: 'truenas',
  },
  pve: {
    label: 'Proxmox VE',
    bestFor: 'Virtualization clusters and standalone hypervisors',
    coverage: 'VMs, containers, storage, cluster health',
    catalogDescription: 'VMs, containers, storage, cluster health',
    autoDetect: true,
    sourcePlatformId: 'proxmox-pve',
  },
  pbs: {
    label: 'Proxmox Backup Server',
    bestFor: 'Backup infrastructure and protected storage',
    coverage: 'Backup jobs, sync, verify, prune, GC',
    catalogDescription: 'Backup jobs, sync, verify, prune, GC',
    autoDetect: true,
    sourcePlatformId: 'proxmox-pbs',
  },
  pmg: {
    label: 'Proxmox Mail Gateway',
    bestFor: 'Mail filtering and delivery operations',
    coverage: 'Mail stats, queues, quarantine, relay health',
    catalogDescription: 'Mail stats, queues, quarantine, relay health',
    autoDetect: true,
    sourcePlatformId: 'proxmox-pmg',
  },
};

const governanceStateForType = (
  type: InfrastructureOnboardingConnectionType,
): PlatformGovernanceState => {
  const sourcePlatformId = PRODUCT_PRESENTATION[type].sourcePlatformId;
  if (!sourcePlatformId) return 'supported';
  return getSourcePlatformManifestEntry(sourcePlatformId)?.governanceState ?? 'supported';
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
    actionLabel: 'Add host',
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
    bestFor: 'TrueNAS, Proxmox, and the current VMware vCenter admission path',
    coverage: 'Platform inventory, workloads, storage, backups, and health',
  },
  agent: {
    title: 'Install Pulse Agent',
    description:
      'Use the agent when you want machine telemetry, or when the system does not expose a management API Pulse can connect to directly.',
    bestFor: 'Linux, FreeBSD, and compatible hosts such as Unraid',
    coverage: 'CPU temperature, disk SMART, services, network metrics, Docker, Kubernetes',
  },
};

export const INFRASTRUCTURE_ONBOARDING_STEPS = [
  'Probe address',
  'Identify platform',
  'Request credentials',
  'Validate access',
  'Start monitoring',
] as const;

export const INFRASTRUCTURE_AGENT_DISCOVERY_LABELS = [
  'Pulse Agent hosts',
  'Docker',
  'Kubernetes',
] as const;

export const INFRASTRUCTURE_AGENT_HOST_LABELS = ['Linux', 'FreeBSD', 'Unraid'] as const;

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
    description: 'Machine telemetry and local service discovery.',
    types: ['agent'],
  },
];

export const getInfrastructureOnboardingProductPresentation = (
  type: InfrastructureOnboardingConnectionType,
): InfrastructureOnboardingProductPresentation => ({
  type,
  ...PRODUCT_PRESENTATION[type],
  governanceState: governanceStateForType(type),
});

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
  'Add infrastructure systems to start monitoring your environment.';

export const getInfrastructureEmptyStateDetail = (): string =>
  'Available system types: VMware vCenter, TrueNAS SCALE, Proxmox VE, Proxmox Backup Server, Proxmox Mail Gateway, and standalone hosts through Pulse Agent. Docker and Kubernetes are discovered from supported agent hosts. VMware vCenter is also available now.';
