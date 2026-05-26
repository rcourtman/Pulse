import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Connection } from '@/api/connections';
import type {
  InfrastructureSystemMemberRow,
  InfrastructureSystemRow,
} from '../connectionsTableModel';
import { InfrastructureWorkspace } from '../InfrastructureWorkspace';

const routeState = vi.hoisted(() => ({
  pathname: '/settings/infrastructure',
  search: '',
}));
const connectionState = vi.hoisted(() => ({
  connections: [] as Connection[],
  rows: null as InfrastructureSystemRow[] | null,
}));
const emptyFleetRow = vi.hoisted(
  () =>
    ({
      fleetSignals: [],
      fleetHighlights: [],
    }) as Pick<InfrastructureSystemRow, 'fleetSignals' | 'fleetHighlights'>,
);
const emptyFleetMember = vi.hoisted(
  () =>
    ({
      fleetSignals: [],
      fleetHighlights: [],
    }) as Pick<InfrastructureSystemMemberRow, 'fleetSignals' | 'fleetHighlights'>,
);
const navigateSpy = vi.hoisted(() => vi.fn());
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));

const originalResizeObserver = globalThis.ResizeObserver;
let latestResizeObserverCallback: ResizeObserverCallback | null = null;

const installResizeObserverMock = () => {
  latestResizeObserverCallback = null;
  class MockResizeObserver {
    constructor(callback: ResizeObserverCallback) {
      latestResizeObserverCallback = callback;
    }

    observe() {}
    unobserve() {}
    disconnect() {}
  }

  Object.defineProperty(globalThis, 'ResizeObserver', {
    configurable: true,
    writable: true,
    value: MockResizeObserver,
  });
};

const emitResizeObserverWidth = (width: number) => {
  latestResizeObserverCallback?.(
    [{ contentRect: { width } } as ResizeObserverEntry],
    {} as ResizeObserver,
  );
};

const restoreResizeObserver = () => {
  if (originalResizeObserver) {
    Object.defineProperty(globalThis, 'ResizeObserver', {
      configurable: true,
      writable: true,
      value: originalResizeObserver,
    });
    return;
  }
  delete (globalThis as any).ResizeObserver;
};

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: routeState.pathname, search: routeState.search }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
}));

vi.mock('../useInfrastructureOperationsState', () => ({
  InfrastructureOperationsStateProvider: (props: { children: unknown }) => <>{props.children}</>,
  useInfrastructureOperationsContext: () => ({
    getUninstallCommand: () => 'curl -fsSL http://pulse/install.sh | bash -s -- --uninstall',
    getWindowsUninstallCommand: () =>
      '$env:PULSE_URL="http://pulse"; $env:PULSE_UNINSTALL="true"; iwr /install.ps1 | iex',
  }),
}));

vi.mock('../useConnectionsLedger', () => ({
  useConnectionsLedger: () => ({
    connections: () => connectionState.connections,
    rows: () =>
      connectionState.rows ??
      connectionState.connections.map((connection) => ({
        id: connection.id,
        ownerType: connection.type,
        name: connection.name || connection.id,
        subtitle: connection.type === 'agent' ? 'via Pulse Agent' : 'via platform API',
        source: connection.type === 'agent' ? 'agent' : 'api',
        host: connection.address,
        coverageLabels: ['VMs'],
        statusLabel: connection.state === 'paused' ? 'Paused' : 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '1m ago',
        lastErrorMessage: connection.lastError?.message,
        ...emptyFleetRow,
        enabled: connection.enabled,
        canEdit: ['pve', 'pbs', 'pmg', 'vmware', 'truenas'].includes(connection.type),
        canPause: connection.capabilities.supportsPause,
        canRemove: connection.type !== 'docker' && connection.type !== 'kubernetes',
        isAgent: connection.type === 'agent',
        isCluster: false,
        attachedConnections: [],
        members: [],
        connection,
      })),
    findById: (id: string) =>
      connectionState.connections.find((connection) => connection.id === id),
    reload: vi.fn(),
    loading: () => false,
    error: () => null,
  }),
}));

vi.mock('../InfrastructureInstallerSection', () => ({
  InfrastructureInstallerSection: (props: { focus?: string }) => (
    <div data-testid="install-section" data-focus={props.focus}>
      install
    </div>
  ),
}));

vi.mock('../ConnectionEditor/CredentialSlots/NodeCredentialSlot', () => ({
  NodeCredentialSlot: (props: { nodeType: string }) => (
    <div data-testid="proxmox-section" data-node-type={props.nodeType}>
      proxmox
    </div>
  ),
}));

vi.mock('../ConnectionEditor/CredentialSlots/TrueNASCredentialSlot', () => ({
  TrueNASCredentialSlot: () => <div data-testid="truenas-section">truenas</div>,
}));

vi.mock('../ConnectionEditor/CredentialSlots/VMwareCredentialSlot', () => ({
  VMwareCredentialSlot: () => <div data-testid="vmware-section">vmware</div>,
}));

vi.mock('../AgentProfilesPanel', () => ({
  AgentProfilesPanel: () => <div data-testid="agent-profiles">profiles</div>,
}));

const connectionFixture = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'pve:zeus',
  type: 'pve',
  name: 'zeus',
  address: 'https://10.0.0.1:8006',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['vms', 'containers'],
  scope: { vms: true, containers: true },
  lastSeen: new Date().toISOString(),
  lastError: null,
  source: 'manual',
  capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
  ...overrides,
});

const baseProps = () =>
  ({
    selectedAgent: () => 'pve',
    onSelectAgent: vi.fn(),
    initialLoadComplete: () => true,
    discoveryEnabled: () => false,
    discoveryMode: () => 'auto',
    discoverySubnetDraft: () => '',
    discoverySubnetError: () => undefined,
    discoveryScanStatus: () => ({ scanning: false }),
    discoveredNodes: () => [],
    savingDiscoverySettings: () => false,
    envOverrides: () => ({}),
    agentStateResources: () => [],
    pbsInstances: () => [],
    pmgInstances: () => [],
    pveNodes: () => [],
    pbsNodes: () => [],
    pmgNodes: () => [],
    trueNASSettings: {
      closeDialog: vi.fn(),
      connections: () => [],
    } as any,
    vmwareSettings: {
      closeDialog: vi.fn(),
      connections: () => [],
    } as any,
    temperatureMonitoringEnabled: () => false,
    triggerDiscoveryScan: vi.fn(),
    loadDiscoveredNodes: vi.fn(),
    handleDiscoveryEnabledChange: vi.fn(),
    handleDiscoveryModeChange: vi.fn(),
    setDiscoveryMode: vi.fn(),
    setDiscoverySubnetDraft: vi.fn(),
    setDiscoverySubnetError: vi.fn(),
    setLastCustomSubnet: vi.fn(),
    commitDiscoverySubnet: vi.fn(),
    parseSubnetList: vi.fn(() => []),
    normalizeSubnetList: vi.fn((value: string) => value),
    isValidCIDR: vi.fn(() => true),
    currentDraftSubnetValue: vi.fn(() => ''),
    discoverySubnetInputRef: vi.fn(),
    testNodeConnection: vi.fn(),
    requestDeleteNode: vi.fn(),
    refreshClusterNodes: vi.fn(),
    setShowNodeModal: vi.fn(),
    editingNode: () => null,
    setEditingNode: vi.fn(),
    setCurrentNodeType: vi.fn(),
    modalResetKey: () => 0,
    setModalResetKey: vi.fn(),
    isNodeModalVisible: () => false,
    securityStatus: () => null,
    resolveTemperatureMonitoringEnabled: () => false,
    temperatureMonitoringLocked: () => false,
    savingTemperatureSetting: () => false,
    handleTemperatureMonitoringChange: vi.fn(),
    disableDockerUpdateActions: () => false,
    disableDockerUpdateActionsLocked: () => false,
    savingDockerUpdateActions: () => false,
    handleDisableDockerUpdateActionsChange: vi.fn(),
    handleNodeTemperatureMonitoringChange: vi.fn(),
    saveNode: vi.fn(),
    showDeleteNodeModal: () => false,
    cancelDeleteNode: vi.fn(),
    deleteNode: vi.fn(),
    deleteNodeLoading: () => false,
    nodePendingDeleteLabel: () => '',
    nodePendingDeleteHost: () => '',
    nodePendingDeleteType: () => '',
    nodePendingDeleteTypeLabel: () => '',
  }) as any;

describe('InfrastructureWorkspace', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    routeState.pathname = '/settings/infrastructure';
    routeState.search = '';
    connectionState.connections = [connectionFixture()];
    connectionState.rows = null;
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      writable: true,
      value: 1024,
    });
  });

  afterEach(() => {
    cleanup();
    restoreResizeObserver();
  });

  const renderWorkspace = (propOverrides: Record<string, unknown> = {}) =>
    render(() => (<InfrastructureWorkspace {...{ ...baseProps(), ...propOverrides }} />) as any);

  it('renders the source manager landing without empty platform sections', async () => {
    renderWorkspace();

    // Card title shifted from 'Infrastructure systems' (which duplicated
    // the page header) to 'Connected systems' which describes the card's
    // contents distinctly. Card-level description was dropped because it
    // duplicated the page-level subtitle.
    await waitFor(() => expect(screen.getByText('Connected systems')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /^Run discovery$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Discovery settings$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Add infrastructure$/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Detect address$/i })).toBeNull();
    // Row-level 'Install agent' surfaces per system that has API coverage
    // but no Pulse Agent yet. The fixture has one such system, so at least
    // one of these buttons should exist.
    expect(screen.getAllByRole('button', { name: /^Install agent$/i }).length).toBeGreaterThan(0);
    const readiness = screen.getByRole('region', {
      name: /Infrastructure setup summary/i,
    });
    expect(within(readiness).getByText('Setup status')).toBeInTheDocument();
    expect(within(readiness).getByText('Systems')).toBeInTheDocument();
    expect(within(readiness).getByText('Live')).toBeInTheDocument();
    expect(within(readiness).getByText('Needs attention')).toBeInTheDocument();
    expect(within(readiness).getByText('Needs agent')).toBeInTheDocument();
    expect(within(readiness).getByText('Discovery')).toBeInTheDocument();
    expect(within(readiness).queryByText('Infrastructure coverage')).toBeNull();
    expect(within(readiness).queryByText('Fleet governance')).toBeNull();
    expect(within(readiness).getAllByText('1 system').length).toBeGreaterThan(0);
    expect(within(readiness).getAllByText('0 systems').length).toBeGreaterThan(0);
    expect(within(readiness).getByText('Discovery off')).toBeInTheDocument();
    // Global 'Install agents' recommendation button is hidden when
    // row-level 'Install agent' chips already surface the apiOnly state
    // per-system.
    expect(
      within(readiness).queryByRole('button', { name: /Install agents/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE').closest('tr')?.className).toContain('grouped-table-row');
    expect(screen.queryByText('VMware vCenter')).toBeNull();
    expect(screen.queryByText('TrueNAS SCALE')).toBeNull();
    expect(screen.queryByText('Pulse Agent hosts')).toBeNull();
    expect(screen.getByRole('button', { name: /Add Proxmox VE/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Add TrueNAS SCALE/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Install Pulse Agent/i })).toBeNull();
    expect(screen.getByRole('button', { name: /Manage/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Monitored systems' })).not.toBeInTheDocument();
  });

  it('routes first-run actions from the source manager guidance', async () => {
    renderWorkspace();

    await waitFor(() => expect(screen.getByText('Connected systems')).toBeInTheDocument());

    // Row-level 'Install agent' replaced the global 'Install agents'
    // recommendation button; same routing target.
    fireEvent.click(screen.getAllByRole('button', { name: /^Install agent$/i })[0]);
    expect(navigateSpy).toHaveBeenLastCalledWith('/settings/infrastructure?add=linux-host', {
      scroll: false,
    });

    fireEvent.click(screen.getByRole('button', { name: /^Add infrastructure$/i }));
    expect(navigateSpy).toHaveBeenLastCalledWith('/settings/infrastructure?add=pick', {
      scroll: false,
    });

    expect(screen.queryByRole('button', { name: /^Monitor endpoint$/i })).toBeNull();
  });

  it('keeps source groups in the catalog order instead of count order', async () => {
    const pveConnection = connectionFixture({
      id: 'pve:zeus',
      type: 'pve',
      name: 'zeus',
      address: 'https://10.0.0.1:8006',
    });
    const towerAgent = connectionFixture({
      id: 'agent:tower',
      type: 'agent',
      name: 'Tower',
      address: 'Tower',
      source: 'agent',
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const miniAgent = connectionFixture({
      id: 'agent:mini',
      type: 'agent',
      name: 'Mini',
      address: 'Mini',
      source: 'agent',
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });

    connectionState.connections = [pveConnection, towerAgent, miniAgent];
    connectionState.rows = [
      {
        id: pveConnection.id,
        ownerType: 'pve',
        name: 'zeus',
        subtitle: 'via platform API',
        source: 'api',
        host: pveConnection.address,
        coverageLabels: ['VMs'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '1m ago',
        ...emptyFleetRow,
        enabled: true,
        canEdit: true,
        canPause: true,
        canRemove: true,
        isAgent: false,
        isCluster: false,
        attachedConnections: [],
        members: [],
        connection: pveConnection,
      },
      {
        id: towerAgent.id,
        ownerType: 'agent',
        name: 'Tower',
        subtitle: 'via Pulse Agent',
        source: 'agent',
        host: 'Tower',
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '1m ago',
        ...emptyFleetRow,
        enabled: true,
        canEdit: false,
        canPause: false,
        canRemove: true,
        isAgent: true,
        isCluster: false,
        attachedConnections: [],
        members: [],
        connection: towerAgent,
      },
      {
        id: miniAgent.id,
        ownerType: 'agent',
        name: 'Mini',
        subtitle: 'via Pulse Agent',
        source: 'agent',
        host: 'Mini',
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '1m ago',
        ...emptyFleetRow,
        enabled: true,
        canEdit: false,
        canPause: false,
        canRemove: true,
        isAgent: true,
        isCluster: false,
        attachedConnections: [],
        members: [],
        connection: miniAgent,
      },
    ];

    renderWorkspace();

    await waitFor(() => expect(screen.getByText('Proxmox VE')).toBeInTheDocument());
    const pveGroup = screen.getByText('Proxmox VE');
    const hostGroup = screen.getByText('Pulse Agent hosts');
    expect(pveGroup.compareDocumentPosition(hostGroup) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it('switches the source manager layout from measured container width during live resize', async () => {
    installResizeObserverMock();
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      writable: true,
      value: 1180,
    });

    renderWorkspace();

    await waitFor(() =>
      expect(screen.getByRole('columnheader', { name: 'System' })).toBeInTheDocument(),
    );

    emitResizeObserverWidth(640);

    await waitFor(() =>
      expect(screen.queryByRole('columnheader', { name: 'System' })).not.toBeInTheDocument(),
    );
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
    expect(screen.getByText('zeus')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('keeps onboarding copy visible in the empty infrastructure state', async () => {
    connectionState.connections = [];
    connectionState.rows = [];

    renderWorkspace();

    await waitFor(() =>
      expect(screen.getByText('Start monitoring infrastructure')).toBeInTheDocument(),
    );
    expect(
      screen.getByText('Choose an infrastructure source to start monitoring your environment.'),
    ).toBeInTheDocument();
    expect(screen.getByText(/Supported source types include TrueNAS SCALE/i)).toBeInTheDocument();
    expect(
      screen.getByText(/VMware vCenter is available as a preview platform/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/Pulse Agent hosts/i)).toBeInTheDocument();
  });

  it('routes discovery actions from the manager and shows discovered candidates in the matching platform group', async () => {
    const triggerDiscoveryScan = vi.fn();
    renderWorkspace({
      discoveredNodes: () => [
        {
          ip: '10.0.0.55',
          port: 8006,
          type: 'pve',
          version: '8.2.2',
          hostname: 'discovered-pve.lab',
        },
      ],
      discoveryScanStatus: () => ({ scanning: false, lastResultAt: Date.now() }),
      triggerDiscoveryScan,
    });

    await waitFor(() => expect(screen.getByText('discovered-pve.lab')).toBeInTheDocument());
    expect(screen.getByText('Discovered')).toBeInTheDocument();
    const readiness = screen.getByRole('region', {
      name: /Infrastructure setup summary/i,
    });
    expect(within(readiness).getByText('1 to review')).toBeInTheDocument();
    expect(within(readiness).getByText(/1 candidate discovered and waiting/i)).toBeInTheDocument();
    fireEvent.click(within(readiness).getByRole('button', { name: /Review candidate/i }));
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=pve', {
      scroll: false,
    });

    fireEvent.click(screen.getByRole('button', { name: /Run discovery/i }));
    expect(triggerDiscoveryScan).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /Discovery settings/i }));
    expect(screen.getByRole('dialog', { name: /Discovery settings/i })).toBeInTheDocument();
    expect(
      screen.getByText(/Configure the saved network scope and background scan behavior/i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^Review$/i }));
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=pve', {
      scroll: false,
    });
  });

  it('hides discovered candidates already represented by the unified connections ledger member aliases', async () => {
    connectionState.rows = [
      {
        id: 'pve:homelab',
        ownerType: 'pve',
        name: 'homelab',
        subtitle: 'Cluster · 1 node',
        source: 'api',
        host: undefined,
        coverageLabels: ['VMs', 'Containers', 'Storage', 'Backups'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '3s ago',
        ...emptyFleetRow,
        enabled: true,
        canEdit: true,
        canPause: true,
        canRemove: true,
        isAgent: false,
        isCluster: true,
        attachedConnections: [],
        members: [
          {
            id: 'node-pi',
            name: 'pi',
            subtitle: 'Primary node',
            source: 'agent',
            host: 'https://pi:8006',
            hostAliases: ['pi', '192.168.0.2'],
            coverageLabels: ['Host telemetry'],
            statusLabel: 'Active',
            statusClassName: 'bg-green-100 text-green-800',
            lastActivityText: '3s ago',
            ...emptyFleetMember,
            primary: true,
          },
        ],
        connection: connectionFixture({
          id: 'pve:homelab',
          name: 'homelab',
          address: 'https://pi:8006',
        }),
      },
    ];

    renderWorkspace({
      discoveredNodes: () => [
        {
          ip: '192.168.0.2',
          port: 8006,
          type: 'pve',
          version: '8.2.2',
        },
      ],
      discoveryScanStatus: () => ({ scanning: false, lastResultAt: Date.now() }),
    });

    await waitFor(() => expect(screen.getByText('homelab')).toBeInTheDocument());
    expect(screen.queryByText('192.168.0.2')).not.toBeInTheDocument();
    expect(screen.queryByText('Discovered')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Review$/i })).toBeNull();
  });

  it('opens a type-specific add route from the matching platform section', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add Proxmox VE/i }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=pve', {
      scroll: false,
    });
  });

  it('opens the platform picker from the Add infrastructure action', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /^Add infrastructure$/i }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=pick', {
      scroll: false,
    });
  });

  it('renders the route-backed picker dialog when the pick query is present', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    const dialog = screen.getByRole('dialog');
    expect(screen.getAllByText('Add infrastructure').length).toBeGreaterThan(0);
    expect(
      screen.getByText('Choose the system, device, host, or service you want Pulse to monitor.'),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByPlaceholderText('Search sources, devices, services...'),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole('button', { name: /Detect API platform/i }),
    ).toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: /Monitor network endpoint/i })).toBeNull();
    expect(within(dialog).getByText('Choose how Pulse should connect')).toBeInTheDocument();
    expect(within(dialog).getByText('Or pick a specific source')).toBeInTheDocument();
    expect(within(dialog).queryByText('Agent telemetry')).toBeNull();
    expect(within(dialog).queryByText('API inventory')).toBeNull();
    expect(within(dialog).getByRole('button', { name: /TrueNAS SCALE/i })).toBeInTheDocument();
    expect(within(dialog).getByText('Unraid')).toBeInTheDocument();
  });

  it('routes the picker detect action into the detect dialog', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    // Primary path button now contains description text, so match by
    // accessible-name prefix.
    fireEvent.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: /Detect API platform/i }),
    );

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=detect', {
      scroll: false,
    });
  });

  it('routes the picker source cards into direct type-specific dialogs', async () => {
    routeState.search = '?add=pick';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    fireEvent.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: /TrueNAS SCALE/i }),
    );

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=truenas', {
      scroll: false,
    });
  });

  it('renders the detect dialog as a secondary utility flow', async () => {
    routeState.search = '?add=detect';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Detect API platform')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Probe a management API endpoint and let Pulse open the matching credential flow when it recognizes the platform.',
      ),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('dialog')).getByRole('button', { name: /Back to source types/i }),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('dialog')).getByRole('button', { name: /Probe API endpoint/i }),
    ).toBeInTheDocument();
  });

  it('renders the route-backed agent dialog when the agent query is present', async () => {
    routeState.search = '?add=agent';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getAllByText('Add Pulse Agent').length).toBeGreaterThan(0);
    expect(screen.getByTestId('install-section')).toBeInTheDocument();
    expect(screen.getByTestId('install-section')).toHaveAttribute('data-focus', 'agent');
  });

  it('tailors agent-backed catalog routes to the selected system', async () => {
    routeState.search = '?add=unraid';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getAllByText('Add Unraid').length).toBeGreaterThan(0);
    expect(screen.getByTestId('install-section')).toHaveAttribute('data-focus', 'unraid');
  });

  it('renders a direct type route before the add dialog mounts', async () => {
    routeState.search = '?add=truenas';
    renderWorkspace();

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByTestId('truenas-section')).toBeInTheDocument();
  });

  it('does not render the agentless availability target route from the infrastructure workspace', async () => {
    routeState.search = '?add=availability';
    renderWorkspace();

    await waitFor(() => expect(screen.getByText('Connected systems')).toBeInTheDocument());
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('opens the manage dialog directly from an existing source card', async () => {
    renderWorkspace({
      pveNodes: () => [{ name: 'zeus', host: 'https://10.0.0.1:8006' } as any],
    });

    await waitFor(() =>
      expect(screen.getByRole('button', { name: /^Manage$/i })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: /^Manage$/i }));

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Manage zeus')).toBeInTheDocument();
    expect(screen.getByTestId('proxmox-section')).toBeInTheDocument();
  });

  it('shows standalone agent identity in the landing row and the agent detail drawer', async () => {
    const towerAgent = connectionFixture({
      id: 'agent:tower',
      type: 'agent',
      name: 'Tower',
      address: 'Tower',
      surfaces: ['host'],
      scope: { host: true } as any,
      source: 'agent',
      agentVersion: '6.0.2',
      agentIdentity: {
        hostname: 'tower',
        platform: 'linux',
        hostProfile: 'unraid',
        osName: 'Unraid',
        osVersion: '7.1.0',
        kernelVersion: '6.12.0',
        architecture: 'x86_64',
        reportIp: '192.168.0.10',
        commandsEnabled: true,
      },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });

    connectionState.connections = [towerAgent];
    connectionState.rows = [
      {
        id: towerAgent.id,
        ownerType: 'agent',
        name: 'Tower',
        subtitle: 'via Pulse Agent',
        identitySubtitle: 'Unraid 7.1.0',
        source: 'agent',
        host: '192.168.0.10',
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '0s ago',
        ...emptyFleetRow,
        enabled: true,
        canEdit: false,
        canPause: false,
        canRemove: true,
        isAgent: true,
        isCluster: false,
        attachedConnections: [],
        members: [],
        connection: towerAgent,
      },
    ];

    renderWorkspace();

    await waitFor(() => expect(screen.getByText('Unraid')).toBeInTheDocument());
    expect(screen.queryByText('Pulse Agent hosts')).toBeNull();
    expect(screen.getByText('Tower')).toBeInTheDocument();
    expect(screen.getByText('Unraid 7.1.0')).toBeInTheDocument();
    expect(screen.getByText('192.168.0.10')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^Add Unraid$/i }));
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure?add=unraid', {
      scroll: false,
    });
    expect(towerAgent.agentIdentity?.hostProfile).toBe('unraid');
    expect(towerAgent.agentIdentity?.platform).toBe('linux');

    fireEvent.click(screen.getByRole('button', { name: /^Manage$/i }));

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Pulse Agent version')).toBeInTheDocument();
    expect(screen.getByText('6.0.2')).toBeInTheDocument();
    expect(screen.getByText('Operating system')).toBeInTheDocument();
    expect(screen.getAllByText('Unraid 7.1.0').length).toBeGreaterThan(0);
    expect(screen.getByText('Reported hostname')).toBeInTheDocument();
    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.getByText('Reported IP')).toBeInTheDocument();
    expect(screen.getAllByText('192.168.0.10').length).toBeGreaterThan(0);
    expect(screen.getByText('Kernel')).toBeInTheDocument();
    expect(screen.getByText('6.12.0')).toBeInTheDocument();
    expect(screen.getByText('Architecture')).toBeInTheDocument();
    expect(screen.getByText('x86_64')).toBeInTheDocument();
    expect(screen.getByText('Remote commands')).toBeInTheDocument();
    expect(screen.getAllByText('Enabled').length).toBeGreaterThan(0);
  });

  it('shows attached Pulse Agent augmentation details on a grouped platform source', async () => {
    const primaryConnection = connectionFixture();
    const attachedAgent = connectionFixture({
      id: 'agent:zeus',
      type: 'agent',
      name: 'zeus-agent',
      address: 'zeus.lab',
      surfaces: ['host'],
      scope: { host: true } as any,
      lastSeen: new Date().toISOString(),
      agentVersion: '6.0.0',
      expectedAgentVersion: '6.0.2',
      agentUpdateAvailable: true,
      capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
    });

    connectionState.connections = [primaryConnection, attachedAgent];
    connectionState.rows = [
      {
        id: primaryConnection.id,
        ownerType: 'pve',
        name: primaryConnection.name || primaryConnection.id,
        subtitle: 'via platform API and Pulse Agent',
        source: 'both',
        host: primaryConnection.address,
        coverageLabels: ['VMs', 'Host telemetry'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 1,
        lastActivityText: '1m ago',
        ...emptyFleetRow,
        enabled: true,
        canEdit: true,
        canPause: true,
        canRemove: true,
        isAgent: false,
        isCluster: false,
        attachedConnections: [attachedAgent],
        members: [],
        connection: primaryConnection,
      },
    ];

    renderWorkspace({
      pveNodes: () => [{ name: 'zeus', host: 'https://10.0.0.1:8006' } as any],
    });

    await waitFor(() =>
      expect(screen.getByRole('button', { name: /^Manage$/i })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: /^Manage$/i }));

    expect(screen.getByText('Agent update')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    expect(screen.getByText('Pulse Agent augmentation')).toBeInTheDocument();
    expect(screen.getByText('zeus-agent')).toBeInTheDocument();
    expect(screen.getByText('Update available')).toBeInTheDocument();
    expect(screen.getByText('6.0.0 -> 6.0.2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Copy uninstall command/i })).toBeInTheDocument();
  });

  it('renders Proxmox cluster rows under the cluster moniker instead of a sibling standalone host row', async () => {
    const primaryConnection = connectionFixture();
    const dellyAgent = connectionFixture({
      id: 'agent:agent-delly',
      type: 'agent',
      name: 'delly',
      address: 'delly',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const minipcAgent = connectionFixture({
      id: 'agent:agent-minipc',
      type: 'agent',
      name: 'minipc',
      address: 'minipc',
      surfaces: ['host'],
      scope: { host: true },
      source: 'agent',
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    });
    const passiveAgentConfigSignals: InfrastructureSystemRow['fleetSignals'] = [
      {
        key: 'config-drift',
        label: 'Config pending',
        detail: 'Pulse has not received a comparable applied agent configuration fingerprint yet',
        tone: 'warning',
      },
      {
        key: 'rollout',
        label: 'Rollout pending',
        detail: 'waiting for the agent to report an applied configuration fingerprint',
        tone: 'warning',
      },
    ];
    connectionState.connections = [primaryConnection, dellyAgent, minipcAgent];
    connectionState.rows = [
      {
        id: primaryConnection.id,
        ownerType: 'pve',
        name: 'homelab',
        subtitle: 'Cluster · 2 nodes',
        source: 'api',
        host: undefined,
        coverageLabels: ['VMs', 'Containers', 'Storage', 'Backups'],
        statusLabel: 'Active',
        statusClassName: 'bg-green-100 text-green-800',
        agentUpdateCount: 0,
        lastActivityText: '1m ago',
        ...emptyFleetRow,
        fleetSignals: passiveAgentConfigSignals,
        fleetHighlights: [],
        enabled: true,
        canEdit: true,
        canPause: true,
        canRemove: true,
        isAgent: false,
        isCluster: true,
        attachedConnections: [dellyAgent, minipcAgent],
        members: [
          {
            id: 'node-delly',
            name: 'delly',
            subtitle: 'Primary node',
            source: 'agent',
            host: 'https://delly:8006',
            coverageLabels: ['Host telemetry'],
            statusLabel: 'Active',
            statusClassName: 'bg-green-100 text-green-800',
            lastActivityText: '1m ago',
            ...emptyFleetMember,
            fleetSignals: passiveAgentConfigSignals,
            fleetHighlights: [],
            primary: true,
            agentConnection: dellyAgent,
          },
          {
            id: 'node-minipc',
            name: 'minipc',
            subtitle: 'Cluster member',
            source: 'agent',
            host: 'https://minipc:8006',
            coverageLabels: ['Host telemetry'],
            statusLabel: 'Active',
            statusClassName: 'bg-green-100 text-green-800',
            lastActivityText: '1m ago',
            ...emptyFleetMember,
            fleetSignals: passiveAgentConfigSignals,
            fleetHighlights: [],
            primary: false,
            agentConnection: minipcAgent,
          },
        ],
        connection: primaryConnection,
      },
    ];

    renderWorkspace();

    await waitFor(() => expect(screen.getByText('homelab')).toBeInTheDocument());
    const readiness = screen.getByRole('region', {
      name: /Infrastructure setup summary/i,
    });
    expect(within(readiness).getByText('Needs attention').nextElementSibling).toHaveTextContent(
      '0 systems',
    );
    expect(screen.getByText('Cluster · 2 nodes')).toBeInTheDocument();
    expect(screen.queryByText('Fleet OK')).toBeNull();
    expect(screen.getByText('Primary node')).toBeInTheDocument();
    expect(screen.getAllByText('Agent').length).toBeGreaterThan(0);
    expect(screen.getByText('delly')).toBeInTheDocument();
    expect(screen.getByText('minipc')).toBeInTheDocument();
    expect(screen.queryByText('Remote control disabled')).toBeNull();
    expect(screen.queryByText('Config pending')).toBeNull();
    expect(screen.queryByText('Rollout pending')).toBeNull();
    expect(screen.getAllByText('Host telemetry').length).toBeGreaterThan(0);
    expect(screen.queryByText('Pulse Agent hosts')).toBeNull();
    expect(screen.getAllByText('delly')).toHaveLength(1);
    expect(screen.getAllByText('minipc')).toHaveLength(1);
  });

  it('hides the add flow and source-manager actions in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    routeState.search = '?add=agent';
    renderWorkspace();

    expect(screen.queryByRole('button', { name: /Detect address/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /Add Proxmox VE/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /^Manage$/i })).toBeNull();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.getByText('Connected systems')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Monitored systems' })).not.toBeInTheDocument();
  });
});
