import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, fireEvent, screen, cleanup, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { ThresholdsTable } from '../ThresholdsTable';
import { normalizeDockerIgnoredInput } from '@/features/alerts/thresholds/helpers';
import type { PMGThresholdDefaults, SnapshotAlertConfig, BackupAlertConfig } from '@/types/alerts';
import type { Agent, Alert } from '@/types/api';

const [getPathname, setPathname] = createSignal('/alerts/thresholds/docker');
const mockNavigate = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => ({
    get pathname() {
      return getPathname();
    },
  }),
}));

vi.mock('../ResourceTable', () => ({
  ResourceTable: (props: {
    title?: string;
    resources?: any[];
    groupedResources?: Record<string, any[]>;
    groupHeaderMeta?: Record<string, { displayName?: string; rawName?: string }>;
    formatMetricValue?: (metric: string, value: number | undefined) => string;
    onToggleDisabled?: (id: string, forceState?: boolean) => void;
    onToggleNodeConnectivity?: (id: string, forceState?: boolean) => void;
  }) => {
    const resources =
      props.resources ||
      (props.groupedResources ? Object.values(props.groupedResources).flat() : []);
    const title = props.title ?? 'unnamed';
    const renderRow = (r: any) => (
      <div data-testid={`resource-row-${r.id}`}>
        <div data-testid={`resource-name-${r.id}`}>{r.name}</div>
        {r.node ? <div data-testid={`resource-node-${r.id}`}>{r.node}</div> : null}
        <div data-testid={`resource-cpu-${r.id}`}>
          {props.formatMetricValue && r.thresholds
            ? props.formatMetricValue('cpu', r.thresholds.cpu)
            : (r.thresholds?.cpu ?? '')}
        </div>
        {props.onToggleDisabled ? (
          <button
            data-testid={`toggle-disabled-${r.id}`}
            onClick={() => props.onToggleDisabled?.(r.id, true)}
          >
            Disable
          </button>
        ) : null}
        {props.onToggleNodeConnectivity ? (
          <button
            data-testid={`toggle-connectivity-${r.id}`}
            onClick={() => props.onToggleNodeConnectivity?.(r.id, true)}
          >
            Disable connectivity
          </button>
        ) : null}
      </div>
    );
    return (
      <div data-testid={`resource-table-${title}`}>
        <div data-testid={`resource-count-${title}`}>{resources.length}</div>
        {props.groupedResources
          ? Object.entries(props.groupedResources).map(([group, groupResources], index) => (
              <div data-testid={`resource-group-${index}`}>
                <div data-testid={`group-header-${index}`}>
                  {props.groupHeaderMeta?.[group]?.displayName ||
                    props.groupHeaderMeta?.[group]?.rawName ||
                    group}
                </div>
                {groupResources.map(renderRow)}
              </div>
            ))
          : resources.map(renderRow)}
      </div>
    );
  },
  Resource: () => null,
  GroupHeaderMeta: () => null,
}));

vi.mock('../Thresholds/sections/CollapsibleSection', () => ({
  CollapsibleSection: (props: any) => (
    <div data-testid={`section-${props.title}`}>{props.children}</div>
  ),
}));

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  setPathname('/alerts/thresholds/docker');
  vi.clearAllMocks();
});

const DEFAULT_PMG_THRESHOLDS: PMGThresholdDefaults = {
  queueTotalWarning: 100,
  queueTotalCritical: 200,
  oldestMessageWarnMins: 30,
  oldestMessageCritMins: 60,
  deferredQueueWarn: 50,
  deferredQueueCritical: 75,
  holdQueueWarn: 25,
  holdQueueCritical: 50,
  quarantineSpamWarn: 10,
  quarantineSpamCritical: 20,
  quarantineVirusWarn: 5,
  quarantineVirusCritical: 10,
  quarantineGrowthWarnPct: 25,
  quarantineGrowthWarnMin: 10,
  quarantineGrowthCritPct: 50,
  quarantineGrowthCritMin: 20,
};

const DEFAULT_DOCKER_DEFAULTS = {
  cpu: 80,
  memory: 85,
  disk: 85,
  restartCount: 3,
  restartWindow: 300,
  memoryWarnPct: 90,
  memoryCriticalPct: 95,
  serviceWarnGapPercent: 10,
  serviceCriticalGapPercent: 50,
};

const baseProps = () => ({
  overrides: () => [],
  setOverrides: vi.fn(),
  rawOverridesConfig: () => ({}),
  setRawOverridesConfig: vi.fn(),
  allGuests: () => [],
  nodes: [],
  agents: [],
  storage: [],
  containerRuntimes: [],
  dockerHosts: [],
  allResources: [],
  pbsInstances: [],
  pmgInstances: [],
  pmgThresholds: () => DEFAULT_PMG_THRESHOLDS,
  setPMGThresholds: vi.fn(),
  guestDefaults: {},
  setGuestDefaults: vi.fn(),
  guestDisableConnectivity: () => false,
  setGuestDisableConnectivity: vi.fn(),
  guestPoweredOffSeverity: () => 'warning' as const,
  setGuestPoweredOffSeverity: vi.fn(),
  nodeDefaults: {},
  setNodeDefaults: vi.fn(),
  pbsDefaults: { cpu: 80, memory: 85 },
  setPBSDefaults: vi.fn(),
  kubernetesDefaults: {
    cpu: 80,
    memory: 85,
    disk: 90,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  },
  setKubernetesDefaults: vi.fn(),
  trueNASDefaults: {
    cpu: 80,
    memory: 85,
    disk: 85,
    usage: 85,
    temperature: 80,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  },
  setTrueNASDefaults: vi.fn(),
  trueNASDiskDefaults: { temperature: 55 },
  setTrueNASDiskDefaults: vi.fn(),
  vmwareDefaults: {
    cpu: 80,
    memory: 85,
    disk: 90,
    usage: 85,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  },
  setVMwareDefaults: vi.fn(),
  agentDefaults: { cpu: 80, memory: 85, disk: 90 },
  setAgentDefaults: vi.fn(),
  diskTempByType: { nvme: 70, sas: 65, sata: 55 },
  setDiskTempByType: vi.fn(),
  dockerDefaults: DEFAULT_DOCKER_DEFAULTS,
  dockerDisableConnectivity: () => false,
  setDockerDisableConnectivity: vi.fn(),
  dockerPoweredOffSeverity: () => 'warning' as const,
  setDockerPoweredOffSeverity: vi.fn(),
  setDockerDefaults: vi.fn(),
  storageDefault: () => 85,
  setStorageDefault: vi.fn(),
  resetGuestDefaults: vi.fn(),
  resetNodeDefaults: vi.fn(),
  resetPBSDefaults: vi.fn(),
  resetKubernetesDefaults: vi.fn(),
  resetTrueNASDefaults: vi.fn(),
  resetTrueNASDiskDefaults: vi.fn(),
  resetVMwareDefaults: vi.fn(),
  resetAgentDefaults: vi.fn(),
  resetDockerDefaults: vi.fn(),
  resetDockerIgnoredPrefixes: vi.fn(),
  resetStorageDefault: vi.fn(),
  factoryGuestDefaults: {},
  factoryNodeDefaults: {},
  factoryPBSDefaults: { cpu: 80, memory: 85 },
  factoryKubernetesDefaults: {
    cpu: 80,
    memory: 85,
    disk: 90,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  },
  factoryTrueNASDefaults: {
    cpu: 80,
    memory: 85,
    disk: 85,
    usage: 85,
    temperature: 80,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  },
  factoryTrueNASDiskDefaults: { temperature: 55 },
  factoryVMwareDefaults: {
    cpu: 80,
    memory: 85,
    disk: 90,
    usage: 85,
    diskRead: -1,
    diskWrite: -1,
    networkIn: -1,
    networkOut: -1,
  },
  factoryAgentDefaults: { cpu: 80, memory: 85, disk: 90 },
  factoryDockerDefaults: DEFAULT_DOCKER_DEFAULTS,
  factoryStorageDefault: 85,
  backupDefaults: () => ({ enabled: false, warningDays: 7, criticalDays: 14 }),
  setBackupDefaults: vi.fn(),
  backupFactoryDefaults: { enabled: false, warningDays: 7, criticalDays: 14 } as BackupAlertConfig,
  resetBackupDefaults: vi.fn(),
  snapshotDefaults: () => ({
    enabled: false,
    warningDays: 30,
    criticalDays: 45,
    warningSizeGiB: 0,
    criticalSizeGiB: 0,
  }),
  setSnapshotDefaults: vi.fn(),
  snapshotFactoryDefaults: {
    enabled: false,
    warningDays: 30,
    criticalDays: 45,
    warningSizeGiB: 0,
    criticalSizeGiB: 0,
  } as SnapshotAlertConfig,
  resetSnapshotDefaults: vi.fn(),
  timeThresholds: () => ({
    guest: 5,
    node: 5,
    storage: 5,
    pbs: 5,
    agent: 5,
    'k8s-cluster': 5,
    'k8s-node': 5,
    'k8s-deployment': 5,
    'k8s-namespace': 5,
    pod: 5,
    'truenas-system': 5,
    'truenas-pool': 5,
    'truenas-dataset': 5,
    'truenas-disk': 5,
    'vmware-host': 5,
    'vmware-vm': 5,
    'vmware-datastore': 5,
    'vmware-network': 5,
  }),
  metricTimeThresholds: () => ({}),
  setMetricTimeThresholds: vi.fn(),
  activeAlerts: {},
  removeAlerts: vi.fn(),
  disableAllNodes: () => false,
  setDisableAllNodes: vi.fn(),
  disableAllGuests: () => false,
  setDisableAllGuests: vi.fn(),
  disableAllAgents: () => false,
  setDisableAllAgents: vi.fn(),
  disableAllStorage: () => false,
  setDisableAllStorage: vi.fn(),
  disableAllPBS: () => false,
  setDisableAllPBS: vi.fn(),
  disableAllPMG: () => false,
  setDisableAllPMG: vi.fn(),
  disableAllDockerHosts: () => false,
  setDisableAllDockerHosts: vi.fn(),
  disableAllDockerServices: () => false,
  setDisableAllDockerServices: vi.fn(),
  disableAllDockerContainers: () => false,
  setDisableAllDockerContainers: vi.fn(),
  disableAllKubernetes: () => false,
  setDisableAllKubernetes: vi.fn(),
  disableAllTrueNAS: () => false,
  setDisableAllTrueNAS: vi.fn(),
  disableAllVMware: () => false,
  setDisableAllVMware: vi.fn(),
  disableAllNodesOffline: () => false,
  setDisableAllNodesOffline: vi.fn(),
  disableAllGuestsOffline: () => false,
  setDisableAllGuestsOffline: vi.fn(),
  disableAllAgentsOffline: () => false,
  setDisableAllAgentsOffline: vi.fn(),
  disableAllPBSOffline: () => false,
  setDisableAllPBSOffline: vi.fn(),
  disableAllPMGOffline: () => false,
  setDisableAllPMGOffline: vi.fn(),
  disableAllDockerHostsOffline: () => false,
  setDisableAllDockerHostsOffline: vi.fn(),
  ignoredGuestPrefixes: () => [] as string[],
  setIgnoredGuestPrefixes: vi.fn(),
  guestTagWhitelist: () => [] as string[],
  setGuestTagWhitelist: vi.fn(),
  guestTagBlacklist: () => [] as string[],
  setGuestTagBlacklist: vi.fn(),
  dockerIgnoredPrefixes: () => [] as string[],
  setDockerIgnoredPrefixes: vi.fn(),
  setHasUnsavedChanges: vi.fn(),
});

describe('normalizeDockerIgnoredInput', () => {
  it('correctly normalizes input string', () => {
    expect(normalizeDockerIgnoredInput('  a  \n b \n\n c ')).toEqual(['a', 'b', 'c']);
  });
});

describe('ThresholdsTable basics', () => {
  it('renders the search input', () => {
    render(() => <ThresholdsTable {...(baseProps() as any)} />);
    expect(screen.getByPlaceholderText(/Search resources/i)).toBeInTheDocument();
  });

  it('allows dismissing the help banner', () => {
    render(() => <ThresholdsTable {...(baseProps() as any)} />);
    const dismissButton = screen.getByLabelText(/Dismiss tips/i);
    fireEvent.click(dismissButton);
    expect(screen.queryByText(/Quick tips:/i)).not.toBeInTheDocument();
  });
});

describe('ThresholdsTable navigation and redirection', () => {
  it('redirects from base path to Proxmox', () => {
    setPathname('/alerts/thresholds');
    render(() => <ThresholdsTable {...(baseProps() as any)} />);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/proxmox', {
      replace: true,
    });
  });

  it('redirects legacy thresholds sub-routes onto canonical platform paths', () => {
    setPathname('/alerts/thresholds/infrastructure');
    render(() => <ThresholdsTable {...(baseProps() as any)} />);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/proxmox', {
      replace: true,
    });

    cleanup();
    mockNavigate.mockReset();

    setPathname('/alerts/thresholds/agents');
    render(() => <ThresholdsTable {...(baseProps() as any)} />);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/systems', { replace: true });
  });

  it('loads systems tab from canonical route', async () => {
    setPathname('/alerts/thresholds/systems');
    const host: Agent = {
      id: 'legacy-h1',
      hostname: 'legacy-host',
      displayName: 'Legacy Host',
      status: 'online',
      lastSeen: 123,
      memory: { total: 100, used: 50, free: 50, usage: 50 },
    };

    render(() => <ThresholdsTable {...(baseProps() as any)} agents={[host]} />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-table-Machines')).toBeInTheDocument();
    });
  });

  it('navigates to correct route when tabs are clicked', () => {
    render(() => <ThresholdsTable {...(baseProps() as any)} />);

    const proxmoxTab = screen
      .getAllByRole('button')
      .find((el) => el.textContent?.includes('Proxmox'));
    if (proxmoxTab) fireEvent.click(proxmoxTab);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/proxmox');

    const machinesTab = screen
      .getAllByRole('button')
      .find((el) => el.textContent?.includes('Machines'));
    if (machinesTab) fireEvent.click(machinesTab);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/systems');

    const vmwareTab = screen
      .getAllByRole('button')
      .find((el) => el.textContent?.includes('vSphere'));
    if (vmwareTab) fireEvent.click(vmwareTab);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/vmware');
  });
});

describe('ThresholdsTable Resource Rendering', () => {
  it('renders systems correctly', async () => {
    setPathname('/alerts/thresholds/systems');
    const host: Agent = {
      id: 'h1',
      hostname: 'host1',
      displayName: 'Host 1',
      status: 'online',
      lastSeen: 123,
      memory: { total: 100, used: 50, free: 50, usage: 50 },
    };

    render(() => <ThresholdsTable {...(baseProps() as any)} agents={[host]} />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-table-Machines')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-count-Machines')).toHaveTextContent('1');
    expect(screen.getByTestId('resource-name-h1')).toHaveTextContent('Host 1');
  });

  it('renders policy-redacted systems with their raw display name (operator-local UI does not redact)', async () => {
    // Resource-policy redaction is a transmission-boundary policy (docs/PRIVACY.md):
    // it gates what leaves the instance toward non-local model providers, not the
    // operator's own browser. The Threshold table must show the same name the
    // operator sees on /infrastructure.
    setPathname('/alerts/thresholds/systems');
    const host = {
      id: 'h2',
      hostname: 'secret-host',
      displayName: 'Secret Host',
      status: 'online',
      lastSeen: 123,
      memory: { total: 100, used: 50, free: 50, usage: 50 },
      policy: {
        sensitivity: 'restricted',
        routing: { scope: 'local-only', redact: ['hostname'] },
      },
      aiSafeSummary: 'redacted by policy',
    } as Agent & { policy: unknown; aiSafeSummary: string };

    render(() => <ThresholdsTable {...(baseProps() as any)} agents={[host]} />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-table-Machines')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-name-h2')).toHaveTextContent('Secret Host');
    expect(screen.getByTestId('resource-name-h2')).not.toHaveTextContent('redacted by policy');
  });

  it('renders TrueNAS appliances on the canonical systems tab with their disk surface', async () => {
    setPathname('/alerts/thresholds/systems');
    const truenasSystem = {
      id: 'truenas-resource',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'TrueNAS Main',
      status: 'online',
      lastSeen: 123,
      platformType: 'truenas',
      platformData: {
        agent: {
          agentId: 'truenas-main',
          disks: [{ mountpoint: '/mnt/tank', type: 'zfs', used: 50, total: 100 }],
        },
      },
    } as any;

    render(() => <ThresholdsTable {...(baseProps() as any)} agents={[truenasSystem]} />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-name-truenas-main')).toHaveTextContent('TrueNAS Main');
    });

    expect(screen.getByTestId('group-header-0')).toHaveTextContent('TrueNAS Main');
    expect(screen.getByTestId('resource-row-agent:truenas-main/disk:mnt-tank')).toBeInTheDocument();
  });

  it('renders infrastructure hosts and guests correctly', async () => {
    setPathname('/alerts/thresholds/infrastructure');
    const node = {
      id: 'node1',
      name: 'pve1',
      displayName: 'PVE Node 1',
      status: 'online',
    };
    const guest = {
      id: 'guest1',
      name: 'vm1',
      vmid: 100,
      status: 'running',
      node: 'pve1',
    };

    render(() => (
      <ThresholdsTable {...(baseProps() as any)} nodes={[node]} allGuests={() => [guest]} />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('section-Virtualization Hosts')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-name-node1')).toHaveTextContent('PVE');

    expect(screen.getByTestId('section-VMs & Containers')).toBeInTheDocument();
    expect(screen.getByTestId('resource-name-guest1')).toHaveTextContent('vm1');
  });

  it('renders policy-redacted guests with their raw display name in operator-local UI', async () => {
    setPathname('/alerts/thresholds/infrastructure');
    const guest = {
      id: 'guest2',
      name: 'secret-vm-2',
      vmid: 200,
      status: 'running',
      node: 'pve1',
      displayName: 'Secret VM 2',
      policy: {
        sensitivity: 'restricted',
        routing: { scope: 'local-only', redact: ['hostname'] },
      },
      aiSafeSummary: 'redacted by policy',
    } as any;

    render(() => <ThresholdsTable {...(baseProps() as any)} allGuests={() => [guest]} />);

    await waitFor(() => {
      expect(screen.getByTestId('section-VMs & Containers')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-name-guest2')).toHaveTextContent('Secret VM 2');
    expect(screen.getByTestId('resource-name-guest2')).not.toHaveTextContent('redacted by policy');
  });

  it('renders policy-redacted guest-group node headers with the raw display name', async () => {
    setPathname('/alerts/thresholds/infrastructure');
    const node = {
      id: 'node-governed',
      name: 'secret-node',
      displayName: 'Secret Node',
      status: 'online',
      policy: {
        sensitivity: 'restricted',
        routing: { scope: 'local-only', redact: ['hostname'] },
      },
      aiSafeSummary: 'redacted by policy',
    } as any;
    const guest = {
      id: 'guest-grouped',
      name: 'vm-grouped',
      vmid: 101,
      status: 'running',
      platformData: {
        node: 'secret-node',
        instance: 'secret-node',
      },
    } as any;

    render(() => (
      <ThresholdsTable {...(baseProps() as any)} nodes={[node]} allGuests={() => [guest]} />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('group-header-0')).toBeInTheDocument();
    });

    // Group headers run the same friendly-node normalizer as non-policied nodes
    // (see "PVE Node 1" -> "PVE" earlier), so "Secret Node" friendly-shortens to
    // "Secret". The point of this test is that the redacted aiSafeSummary does
    // NOT replace the node header in operator-local UI.
    expect(screen.getByTestId('group-header-0')).toHaveTextContent('Secret');
    expect(screen.getByTestId('group-header-0')).not.toHaveTextContent('redacted by policy');
  });

  it('renders policy-redacted storage with its raw display name in operator-local UI', async () => {
    setPathname('/alerts/thresholds/infrastructure');
    const storage = {
      id: 'storage1',
      name: 'secret-datastore',
      status: 'available',
      node: 'pve1',
      displayName: 'Secret Datastore',
      policy: {
        sensitivity: 'restricted',
        routing: { scope: 'local-only', redact: ['path'] },
      },
      aiSafeSummary: 'redacted by policy',
    } as any;

    render(() => <ThresholdsTable {...(baseProps() as any)} storage={[storage]} />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-name-storage1')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-name-storage1')).toHaveTextContent('Secret Datastore');
    expect(screen.getByTestId('resource-name-storage1')).not.toHaveTextContent(
      'redacted by policy',
    );
  });

  it('renders policy-redacted docker containers with their raw display name in operator-local UI', async () => {
    setPathname('/alerts/thresholds/containers');
    const dockerHost = {
      id: 'docker-host-1',
      type: 'docker-host',
      name: 'docker-parent',
      displayName: 'Docker Parent',
      platformId: 'docker-platform-parent',
      platformType: 'docker',
      sourceType: 'agent',
      status: 'online',
      lastSeen: 789,
    } as any;
    const container = {
      id: 'container-governed',
      parentId: 'docker-host-1',
      type: 'app-container',
      name: '/secret-nginx',
      displayName: 'Secret Nginx',
      platformId: 'docker-platform-parent',
      platformType: 'docker',
      sourceType: 'agent',
      status: 'running',
      lastSeen: 789,
      policy: {
        sensitivity: 'restricted',
        routing: { scope: 'local-only', redact: ['hostname'] },
      },
      aiSafeSummary: 'redacted by policy',
    } as any;

    render(() => (
      <ThresholdsTable
        {...(baseProps() as any)}
        containerRuntimes={[dockerHost]}
        dockerHosts={[dockerHost]}
        allResources={[container]}
      />
    ));

    await waitFor(() => {
      expect(
        screen.getByTestId('resource-name-docker:docker-host-1/container-governed'),
      ).toBeTruthy();
    });

    expect(
      screen.getByTestId('resource-name-docker:docker-host-1/container-governed'),
    ).toHaveTextContent('Secret Nginx');
    expect(
      screen.getByTestId('resource-name-docker:docker-host-1/container-governed'),
    ).not.toHaveTextContent('redacted by policy');
  });

  it('renders TrueNAS app containers under canonical container runtimes without Docker-only controls', async () => {
    setPathname('/alerts/thresholds/containers');
    const truenasRuntime = {
      id: 'truenas-resource',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'TrueNAS Main',
      platformId: 'truenas-main',
      platformType: 'truenas',
      sourceType: 'hybrid',
      status: 'online',
      lastSeen: 789,
    } as any;
    const container = {
      id: 'ix-nextcloud',
      parentId: 'truenas-resource',
      type: 'app-container',
      name: 'nextcloud',
      displayName: 'Nextcloud',
      platformId: 'truenas-main',
      platformType: 'truenas',
      sourceType: 'api',
      status: 'running',
      lastSeen: 789,
    } as any;

    render(() => (
      <ThresholdsTable
        {...(baseProps() as any)}
        containerRuntimes={[truenasRuntime]}
        allResources={[container]}
      />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('resource-table-Container Runtimes')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-name-truenas-resource')).toHaveTextContent('TrueNAS');
    expect(
      screen.getByTestId('resource-name-docker:truenas-resource/ix-nextcloud'),
    ).toHaveTextContent('Nextcloud');
    expect(screen.queryByText('Ignored container prefixes')).not.toBeInTheDocument();
    expect(screen.queryByText('Swarm service alerts')).not.toBeInTheDocument();
  });

  it('renders policy-redacted agent disk node labels with the raw display name in operator-local UI', async () => {
    setPathname('/alerts/thresholds/systems');
    const host = {
      id: 'agent-governed',
      type: 'agent',
      name: 'secret-host',
      displayName: 'Secret Host',
      status: 'online',
      lastSeen: 123,
      memory: { total: 100, used: 50, free: 50, usage: 50 },
      platformData: {
        agent: {
          disks: [{ mountpoint: '/var/lib', type: 'ext4' }],
        },
      },
      policy: {
        sensitivity: 'restricted',
        routing: { scope: 'local-only', redact: ['hostname'] },
      },
      aiSafeSummary: 'redacted by policy',
    } as any;

    render(() => <ThresholdsTable {...(baseProps() as any)} agents={[host]} />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-node-agent:agent-governed/disk:var-lib')).toBeTruthy();
    });

    expect(screen.getByTestId('resource-node-agent:agent-governed/disk:var-lib')).toHaveTextContent(
      'Secret Host',
    );
    expect(
      screen.getByTestId('resource-node-agent:agent-governed/disk:var-lib'),
    ).not.toHaveTextContent('redacted by policy');
  });

  it('renders vSphere alert targets from canonical VMware resources', async () => {
    setPathname('/alerts/thresholds/vmware');
    const resources = [
      {
        id: 'vmware:vc-1:host:host-101',
        type: 'agent',
        name: 'esxi-01.lab.local',
        displayName: 'ESXi 01',
        platformId: 'vc-1',
        platformType: 'vmware-vsphere',
        sourceType: 'api',
        sources: ['vmware'],
        status: 'online',
        lastSeen: 123,
        vmware: {
          connectionName: 'Lab vCenter',
          clusterName: 'Prod Compute',
          managedObjectId: 'host-101',
          entityType: 'host',
        },
      },
      {
        id: 'vmware:vc-1:vm:vm-201',
        type: 'vm',
        name: 'app-01',
        displayName: 'App 01',
        platformId: 'vc-1',
        platformType: 'vmware-vsphere',
        sourceType: 'api',
        sources: ['vmware'],
        status: 'online',
        lastSeen: 123,
        parentName: 'esxi-01.lab.local',
        vmware: {
          connectionName: 'Lab vCenter',
          runtimeHostName: 'esxi-01.lab.local',
          managedObjectId: 'vm-201',
          entityType: 'vm',
        },
      },
      {
        id: 'vmware:vc-1:datastore:datastore-301',
        type: 'storage',
        name: 'nvme-primary',
        displayName: 'nvme-primary',
        platformId: 'vc-1',
        platformType: 'vmware-vsphere',
        sourceType: 'api',
        sources: ['vmware'],
        status: 'online',
        lastSeen: 123,
        storage: { platform: 'vmware-vsphere', topology: 'datastore' },
        vmware: {
          connectionName: 'Lab vCenter',
          datacenterName: 'Lab Datacenter',
          managedObjectId: 'datastore-301',
          entityType: 'datastore',
        },
      },
      {
        id: 'vmware:vc-1:network:network-401',
        type: 'network',
        name: 'VM Network',
        displayName: 'VM Network',
        platformId: 'vc-1',
        platformType: 'vmware-vsphere',
        sourceType: 'api',
        sources: ['vmware'],
        status: 'online',
        lastSeen: 123,
        vmware: {
          connectionName: 'Lab vCenter',
          datacenterName: 'Lab Datacenter',
          managedObjectId: 'network-401',
          entityType: 'network',
        },
      },
    ] as any[];

    render(() => <ThresholdsTable {...(baseProps() as any)} allResources={resources} />);

    await waitFor(() => {
      expect(screen.getByTestId('section-Hosts')).toBeInTheDocument();
    });

    expect(screen.getByTestId('section-Virtual Machines')).toBeInTheDocument();
    expect(screen.getByTestId('section-Datastores')).toBeInTheDocument();
    expect(screen.getByTestId('section-Networks')).toBeInTheDocument();
    expect(screen.getByTestId('resource-name-vmware:vc-1:host:host-101')).toHaveTextContent(
      'ESXi 01',
    );
    expect(screen.getByTestId('resource-name-vmware:vc-1:vm:vm-201')).toHaveTextContent('App 01');
    expect(
      screen.getByTestId('resource-name-vmware:vc-1:datastore:datastore-301'),
    ).toHaveTextContent('nvme-primary');
    expect(screen.getByTestId('resource-name-vmware:vc-1:network:network-401')).toHaveTextContent(
      'VM Network',
    );
  });
});

describe('ThresholdsTable Metric Formatting', () => {
  it('formats metrics correctly', async () => {
    setPathname('/alerts/thresholds/systems');
    const host: Agent = {
      id: 'h1',
      hostname: 'host1',
      displayName: 'Host 1',
      status: 'online',
      lastSeen: 123,
      memory: { total: 100, used: 50, free: 50, usage: 50 },
    };

    const override = {
      id: 'h1',
      name: 'host1',
      type: 'agent' as const,
      thresholds: {
        cpu: 85,
      },
    };

    render(() => (
      <ThresholdsTable {...(baseProps() as any)} agents={[host]} overrides={() => [override]} />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('resource-cpu-h1')).toHaveTextContent('85%');
    });
  });
});

describe('ThresholdsTable V6 ID compatibility', () => {
  it('matches agent overrides keyed by actionable agent ID', async () => {
    setPathname('/alerts/thresholds/systems');
    const host = {
      id: 'resource:host:abc123',
      type: 'agent',
      name: 'host-v6',
      displayName: 'Host V6',
      platformId: 'host-platform-1',
      platformType: 'agent',
      sourceType: 'agent',
      status: 'online',
      lastSeen: 123,
      agent: { agentId: 'agent-host-123' },
      platformData: { agent: { agentId: 'agent-host-123' } },
    } as any;
    const override = {
      id: 'agent-host-123',
      name: 'Host V6',
      type: 'agent' as const,
      thresholds: { cpu: 88 },
    };

    render(() => (
      <ThresholdsTable {...(baseProps() as any)} agents={[host]} overrides={() => [override]} />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('resource-row-agent-host-123')).toBeInTheDocument();
    });

    expect(screen.queryByTestId('resource-row-resource:host:abc123')).not.toBeInTheDocument();
  });

  it('matches docker-host overrides keyed by actionable hostSourceId', async () => {
    setPathname('/alerts/thresholds/containers');
    const dockerHost = {
      id: 'resource:docker:xyz789',
      type: 'docker-host',
      name: 'docker-v6',
      displayName: 'Docker Host V6',
      platformId: 'docker-platform-1',
      platformType: 'docker',
      sourceType: 'agent',
      status: 'online',
      lastSeen: 456,
      platformData: { docker: { hostSourceId: 'docker-source-123' } },
    } as any;
    const override = {
      id: 'docker-source-123',
      name: 'Docker Host V6',
      type: 'dockerHost' as const,
      disableConnectivity: true,
      thresholds: {},
    };

    render(() => (
      <ThresholdsTable
        {...(baseProps() as any)}
        containerRuntimes={[dockerHost]}
        dockerHosts={[dockerHost]}
        overrides={() => [override]}
      />
    ));

    await waitFor(() => {
      expect(screen.getByTestId('resource-row-docker-source-123')).toBeInTheDocument();
    });

    expect(screen.queryByTestId('resource-row-resource:docker:xyz789')).not.toBeInTheDocument();
  });

  it('matches docker-container overrides keyed by actionable docker host ID', async () => {
    setPathname('/alerts/thresholds/containers');
    const dockerHost = {
      id: 'resource:docker:parent1',
      type: 'docker-host',
      name: 'docker-parent',
      displayName: 'Docker Parent',
      platformId: 'docker-platform-parent',
      platformType: 'docker',
      sourceType: 'agent',
      status: 'online',
      lastSeen: 789,
      platformData: { docker: { hostSourceId: 'docker-source-parent' } },
    } as any;
    const container = {
      id: 'container-hash-1',
      parentId: 'resource:docker:parent1',
      type: 'docker-container',
      name: '/nginx',
      displayName: 'nginx',
      platformId: 'docker-platform-parent',
      platformType: 'docker',
      sourceType: 'agent',
      status: 'running',
      lastSeen: 789,
    } as any;
    const override = {
      id: 'docker:docker-source-parent/container-hash-1',
      name: 'nginx',
      type: 'dockerContainer' as const,
      thresholds: { cpu: 92 },
    };

    render(() => (
      <ThresholdsTable
        {...(baseProps() as any)}
        containerRuntimes={[dockerHost]}
        dockerHosts={[dockerHost]}
        allResources={[container]}
        overrides={() => [override]}
      />
    ));

    await waitFor(() => {
      expect(
        screen.getByTestId('resource-row-docker:docker-source-parent/container-hash-1'),
      ).toBeInTheDocument();
    });

    expect(
      screen.queryByTestId('resource-row-docker:resource:docker:parent1/container-hash-1'),
    ).not.toBeInTheDocument();
  });

  it('removes docker host offline alerts using legacy compatibility IDs', async () => {
    setPathname('/alerts/thresholds/containers');
    const removeAlerts = vi.fn();
    const dockerHost = {
      id: 'docker-source-123',
      type: 'dockerHost',
      name: 'docker-v6',
      displayName: 'Docker Host V6',
      disableConnectivity: false,
      disabled: false,
      thresholds: {},
    } as any;

    render(() => (
      <ThresholdsTable
        {...(baseProps() as any)}
        containerRuntimes={[dockerHost]}
        dockerHosts={[dockerHost]}
        removeAlerts={removeAlerts}
      />
    ));

    await fireEvent.click(screen.getByTestId('toggle-connectivity-docker-source-123'));

    expect(removeAlerts).toHaveBeenCalledTimes(1);
    const predicate = removeAlerts.mock.calls[0][0] as (alert: Alert) => boolean;
    expect(
      predicate({
        id: 'docker:docker-source-123::connectivity',
        type: 'offline',
        level: 'critical',
        resourceId: 'docker:docker-source-123',
        resourceName: 'Docker Host V6',
        node: '',
        instance: '',
        message: 'offline',
        value: 0,
        threshold: 0,
        startTime: new Date().toISOString(),
        acknowledged: false,
      }),
    ).toBe(true);
  });

  it('removes PBS offline alerts using legacy compatibility IDs when disabled', async () => {
    setPathname('/alerts/thresholds/pbs');
    const removeAlerts = vi.fn();
    const pbs = {
      id: 'pbs-main',
      type: 'pbs',
      name: 'PBS Main',
      displayName: 'PBS Main',
      disableConnectivity: false,
      disabled: false,
      thresholds: {},
    } as any;

    render(() => (
      <ThresholdsTable {...(baseProps() as any)} pbsInstances={[pbs]} removeAlerts={removeAlerts} />
    ));

    await fireEvent.click(screen.getByTestId('toggle-disabled-pbs-main'));

    expect(removeAlerts).toHaveBeenCalledTimes(1);
    const predicate = removeAlerts.mock.calls[0][0] as (alert: Alert) => boolean;
    expect(
      predicate({
        id: 'pbs:pbs-main::connectivity',
        type: 'offline',
        level: 'critical',
        resourceId: 'pbs-main',
        resourceName: 'PBS Main',
        node: '',
        instance: '',
        message: 'offline',
        value: 0,
        threshold: 0,
        startTime: new Date().toISOString(),
        acknowledged: false,
      }),
    ).toBe(true);
  });
});
