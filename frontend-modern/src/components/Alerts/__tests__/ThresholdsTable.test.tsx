import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, fireEvent, screen, cleanup, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { ThresholdsTable, normalizeDockerIgnoredInput } from '../ThresholdsTable';
import type { PMGThresholdDefaults, SnapshotAlertConfig, BackupAlertConfig } from '@/types/alerts';
import type { Host } from '@/types/api';

const [getPathname, setPathname] = createSignal('/alerts/thresholds/containers');
const mockNavigate = vi.fn();

vi.mock('@solidjs/router', () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => ({
    get pathname() { return getPathname(); }
  }),
}));

vi.mock('../ResourceTable', () => ({
  ResourceTable: (props: {
    title?: string;
    resources?: any[];
    groupedResources?: Record<string, any[]>;
    formatMetricValue?: (metric: string, value: number | undefined) => string;
  }) => {
    const resources = props.resources || (props.groupedResources ? Object.values(props.groupedResources).flat() : []);
    const title = props.title ?? 'unnamed';
    return (
      <div data-testid={`resource-table-${title}`}>
        <div data-testid={`resource-count-${title}`}>{resources.length}</div>
        {resources.map((r: any) => (
          <div data-testid={`resource-row-${r.id}`}>
            <div data-testid={`resource-name-${r.id}`}>{r.name}</div>
            <div data-testid={`resource-cpu-${r.id}`}>
              {props.formatMetricValue && r.thresholds ? props.formatMetricValue('cpu', r.thresholds.cpu) : (r.thresholds?.cpu ?? '')}
            </div>
          </div>
        ))}
      </div>
    );
  },
  Resource: () => null,
  GroupHeaderMeta: () => null,
}));

vi.mock('../Thresholds/sections/CollapsibleSection', () => ({
  CollapsibleSection: (props: any) => (
    <div data-testid={`section-${props.title}`}>
      {props.children}
    </div>
  ),
}));

afterEach(() => {
  cleanup();
});

beforeEach(() => {
  setPathname('/alerts/thresholds/containers');
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
  hosts: [],
  storage: [],
  dockerHosts: [],
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
  hostDefaults: { cpu: 80, memory: 85, disk: 90 },
  setHostDefaults: vi.fn(),
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
  resetHostDefaults: vi.fn(),
  resetDockerDefaults: vi.fn(),
  resetDockerIgnoredPrefixes: vi.fn(),
  resetStorageDefault: vi.fn(),
  factoryGuestDefaults: {},
  factoryNodeDefaults: {},
  factoryHostDefaults: { cpu: 80, memory: 85, disk: 90 },
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
  timeThresholds: () => ({ guest: 5, node: 5, storage: 5, pbs: 5, host: 5 }),
  metricTimeThresholds: () => ({}),
  setMetricTimeThresholds: vi.fn(),
  activeAlerts: {},
  removeAlerts: vi.fn(),
  disableAllNodes: () => false,
  setDisableAllNodes: vi.fn(),
  disableAllGuests: () => false,
  setDisableAllGuests: vi.fn(),
  disableAllHosts: () => false,
  setDisableAllHosts: vi.fn(),
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
  disableAllNodesOffline: () => false,
  setDisableAllNodesOffline: vi.fn(),
  disableAllGuestsOffline: () => false,
  setDisableAllGuestsOffline: vi.fn(),
  disableAllHostsOffline: () => false,
  setDisableAllHostsOffline: vi.fn(),
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
  it('redirects from base path to proxmox', () => {
    setPathname('/alerts/thresholds');
    render(() => <ThresholdsTable {...(baseProps() as any)} />);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/proxmox', { replace: true });
  });

  it('redirects legacy docker path to containers', () => {
    setPathname('/alerts/thresholds/docker');
    render(() => <ThresholdsTable {...(baseProps() as any)} />);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/containers', { replace: true, scroll: false });
  });

  it('navigates to correct route when tabs are clicked', () => {
    render(() => <ThresholdsTable {...(baseProps() as any)} />);

    const hostsTab = screen.getAllByRole('button').find(el => el.textContent?.includes('Host Agents'));
    if (hostsTab) fireEvent.click(hostsTab);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/hosts');

    const mailTab = screen.getAllByRole('button').find(el => el.textContent?.includes('Mail Gateway'));
    if (mailTab) fireEvent.click(mailTab);
    expect(mockNavigate).toHaveBeenCalledWith('/alerts/thresholds/mail-gateway');
  });
});

describe('ThresholdsTable Resource Rendering', () => {
  it('renders host agents correctly', async () => {
    setPathname('/alerts/thresholds/hosts');
    const host: Host = {
      id: 'h1',
      hostname: 'host1',
      displayName: 'Host 1',
      status: 'online',
      lastSeen: 123,
      memory: { total: 100, used: 50, free: 50, usage: 50 },
    };

    render(() => <ThresholdsTable {...(baseProps() as any)} hosts={[host]} />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-table-Host Agents')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-count-Host Agents')).toHaveTextContent('1');
    expect(screen.getByTestId('resource-name-h1')).toHaveTextContent('Host 1');
  });

  it('renders proxmox nodes and guests correctly', async () => {
    setPathname('/alerts/thresholds/proxmox');
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

    render(() => <ThresholdsTable
      {...(baseProps() as any)}
      nodes={[node]}
      allGuests={() => [guest]}
    />);

    await waitFor(() => {
      expect(screen.getByTestId('section-Proxmox Nodes')).toBeInTheDocument();
    });

    expect(screen.getByTestId('resource-name-node1')).toHaveTextContent('PVE');

    expect(screen.getByTestId('section-VMs & Containers')).toBeInTheDocument();
    expect(screen.getByTestId('resource-name-guest1')).toHaveTextContent('vm1');
  });

});

describe('ThresholdsTable Metric Formatting', () => {
  it('formats metrics correctly', async () => {
    setPathname('/alerts/thresholds/hosts');
    const host: Host = {
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
      type: 'hostAgent' as const,
      thresholds: {
        cpu: 85
      }
    };

    render(() => <ThresholdsTable
      {...(baseProps() as any)}
      hosts={[host]}
      overrides={() => [override]}
    />);

    await waitFor(() => {
      expect(screen.getByTestId('resource-cpu-h1')).toHaveTextContent('85%');
    });
  });
});
