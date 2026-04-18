import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { UnifiedAgentRow } from '../infrastructureOperationsModel';
import { InfrastructureWorkspace } from '../InfrastructureWorkspace';

let mockPathname = '/settings/infrastructure';
const navigateSpy = vi.hoisted(() => vi.fn());
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));
const setExpandedRowKeySpy = vi.hoisted(() => vi.fn());
const setSelectedIgnoredRowKeySpy = vi.hoisted(() => vi.fn());

let mockActiveRows: UnifiedAgentRow[] = [];
let mockIgnoredRows: UnifiedAgentRow[] = [];
const [selectedActiveRowKey, setSelectedActiveRowKey] = createSignal<string | null>(null);
const [selectedIgnoredRowKey, setSelectedIgnoredRowKey] = createSignal<string | null>(null);

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockPathname }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => presentationPolicyIsReadOnlyMock(),
}));

vi.mock('../useInfrastructureOperationsState', () => ({
  InfrastructureOperationsStateProvider: (props: { children: unknown }) => <>{props.children}</>,
  useInfrastructureOperationsContext: () => ({
    activeRows: () => mockActiveRows,
    monitoringStoppedRows: () => mockIgnoredRows,
    selectedActiveRow: () =>
      mockActiveRows.find((row) => row.rowKey === selectedActiveRowKey()) ?? null,
    selectedIgnoredRow: () =>
      mockIgnoredRows.find((row) => row.rowKey === selectedIgnoredRowKey()) ?? null,
    setExpandedRowKey: (value: string | null) => {
      setExpandedRowKeySpy(value);
      setSelectedActiveRowKey(value);
    },
    setSelectedIgnoredRowKey: (value: string | null) => {
      setSelectedIgnoredRowKeySpy(value);
      setSelectedIgnoredRowKey(value);
    },
  }),
}));

vi.mock('../InfrastructureInstallerSection', () => ({
  InfrastructureInstallerSection: () => <div data-testid="install-section">install</div>,
}));

vi.mock('../PlatformConnectionsWorkspace', () => ({
  PlatformConnectionsWorkspace: () => <div data-testid="platform-section">platforms</div>,
}));

vi.mock('../InfrastructureActiveRowDetails', () => ({
  InfrastructureActiveRowDetails: () => <div data-testid="active-details">active details</div>,
}));

vi.mock('../InfrastructureIgnoredRowDetails', () => ({
  InfrastructureIgnoredRowDetails: () => <div data-testid="ignored-details">ignored details</div>,
}));

vi.mock('../InfrastructureStopMonitoringDialog', () => ({
  InfrastructureStopMonitoringDialog: () => <div data-testid="stop-monitoring-dialog" />,
}));

vi.mock('../AgentProfilesPanel', () => ({
  AgentProfilesPanel: () => <div data-testid="agent-profiles">profiles</div>,
}));

const reportingRow = (overrides: Partial<UnifiedAgentRow> = {}): UnifiedAgentRow =>
  ({
    rowKey: 'agent:tower',
    id: 'tower',
    name: 'tower',
    hostname: 'tower.local',
    capabilities: ['agent'],
    status: 'active',
    healthStatus: 'online',
    lastSeen: Date.now(),
    upgradePlatform: 'linux',
    scope: { label: 'Default', category: 'default' },
    installFlags: [],
    searchText: 'tower',
    surfaces: [
      {
        key: 'agent',
        kind: 'agent',
        label: 'Host telemetry',
        detail: 'Host telemetry',
        action: 'stop-monitoring',
        controlId: 'tower',
      },
    ],
    ...overrides,
  }) as UnifiedAgentRow;

const trueNASOpenCreateDialogSpy = vi.fn();
const vmwareOpenCreateDialogSpy = vi.fn();
const setShowNodeModalSpy = vi.fn();
const setEditingNodeSpy = vi.fn();
const setCurrentNodeTypeSpy = vi.fn();
const setModalResetKeySpy = vi.fn();

const onSelectAgentSpy = vi.fn();

const baseProps = () =>
  ({
    pveNodes: () => [{ id: 'pve-1', name: 'zeus', host: '10.0.0.1', type: 'pve', status: 'connected' }],
    pbsNodes: () => [],
    pmgNodes: () => [],
    agentStateResources: () => [],
    trueNASSettings: {
      connections: () => [{ id: 'tn-1', name: 'Tower NAS', host: '10.0.0.20', enabled: true }],
      openCreateDialog: trueNASOpenCreateDialogSpy,
      closeDialog: vi.fn(),
      closeDeleteDialog: vi.fn(),
      openEditDialog: vi.fn(),
    },
    vmwareSettings: {
      connections: () => [{ id: 'vm-1', name: 'lab-vcenter', host: '10.0.0.30', enabled: true }],
      openCreateDialog: vmwareOpenCreateDialogSpy,
      closeDialog: vi.fn(),
      closeDeleteDialog: vi.fn(),
      openEditDialog: vi.fn(),
    },
    selectedAgent: () => 'pve',
    onSelectAgent: onSelectAgentSpy,
    setShowNodeModal: setShowNodeModalSpy,
    setEditingNode: setEditingNodeSpy,
    setCurrentNodeType: setCurrentNodeTypeSpy,
    setModalResetKey: setModalResetKeySpy,
  }) as any;

describe('InfrastructureWorkspace', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    setExpandedRowKeySpy.mockReset();
    setSelectedIgnoredRowKeySpy.mockReset();
    trueNASOpenCreateDialogSpy.mockReset();
    vmwareOpenCreateDialogSpy.mockReset();
    setShowNodeModalSpy.mockReset();
    setEditingNodeSpy.mockReset();
    setCurrentNodeTypeSpy.mockReset();
    setModalResetKeySpy.mockReset();
    onSelectAgentSpy.mockReset();
    mockPathname = '/settings/infrastructure';
    mockActiveRows = [reportingRow()];
    mockIgnoredRows = [];
    setSelectedActiveRowKey(null);
    setSelectedIgnoredRowKey(null);
  });

  afterEach(() => {
    cleanup();
  });

  const renderWorkspace = (propOverrides: Record<string, unknown> = {}) =>
    render(() => (<InfrastructureWorkspace {...{ ...baseProps(), ...propOverrides }} />) as any);

  it('renders only the top-level ledger at the base infrastructure route', () => {
    renderWorkspace();

    expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add infrastructure' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Connections' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Agent profiles' })).toBeNull();
    expect(screen.queryByTestId('platform-section')).toBeNull();
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.queryByTestId('agent-profiles')).toBeNull();
  });

  it('shows monitored systems only in the top ledger', () => {
    renderWorkspace();

    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.queryByText('zeus')).toBeNull();
    expect(screen.queryByText('Tower NAS')).toBeNull();
    expect(screen.queryByText('lab-vcenter')).toBeNull();
  });

  it('opens the add-system picker when the add button is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));

    expect(screen.getByText('Install on a host')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS SCALE')).toBeInTheDocument();
  });

  it('routes the agent-host choice to the install section deep link', () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByText('Install on a host'));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/install', {
      scroll: false,
    });
  });

  it('opens provider creation flows directly from the add-system picker', () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByText('TrueNAS SCALE'));

    expect(trueNASOpenCreateDialogSpy).toHaveBeenCalledTimes(1);
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/truenas', {
      scroll: false,
    });

    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByText('VMware vSphere or ESXi'));

    expect(vmwareOpenCreateDialogSpy).toHaveBeenCalledTimes(1);
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/vmware', {
      scroll: false,
    });
  });

  it('opens the proxmox node modal directly from the add-system picker', () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole('button', { name: /Add infrastructure/i }));
    fireEvent.click(screen.getByText('Proxmox VE'));

    expect(onSelectAgentSpy).toHaveBeenCalledWith('pve');
    expect(setCurrentNodeTypeSpy).toHaveBeenCalledWith('pve');
    expect(setEditingNodeSpy).toHaveBeenCalledWith(null);
    expect(setShowNodeModalSpy).toHaveBeenCalledWith(true);
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/proxmox/pve', {
      scroll: false,
    });
  });

  it('opens reporting details from the top ledger in a drawer', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: 'View details' }));

    expect(setExpandedRowKeySpy).toHaveBeenCalledWith('agent:tower');
    expect(screen.getByTestId('active-details')).toBeInTheDocument();
  });

  it('keeps the ledger visible and opens platform setup in a drawer on platform deep links', () => {
    mockPathname = '/settings/infrastructure/platforms/truenas';
    renderWorkspace();

    expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument();
    expect(screen.getByText('Platform connections')).toBeInTheDocument();
    expect(screen.getByTestId('platform-section')).toBeInTheDocument();
    expect(screen.queryByTestId('install-section')).toBeNull();
  });

  it('keeps the ledger visible and opens install tools in a drawer on install deep links', () => {
    mockPathname = '/settings/infrastructure/install';
    renderWorkspace();

    expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument();
    expect(screen.getByText('Install Pulse agent')).toBeInTheDocument();
    expect(screen.getByTestId('install-section')).toBeInTheDocument();
    expect(screen.queryByTestId('platform-section')).toBeNull();
  });

  it('opens agent profiles from the add-infrastructure chooser secondary action', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: 'Add infrastructure' }));
    fireEvent.click(screen.getByRole('button', { name: 'Manage agent profiles' }));

    expect(screen.getByTestId('agent-profiles')).toBeInTheDocument();
  });

  it('closes the platform drawer back to the ledger route', () => {
    mockPathname = '/settings/infrastructure/platforms/truenas';
    renderWorkspace();

    fireEvent.click(screen.getAllByRole('button', { name: 'Close' })[0]);

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', {
      scroll: false,
    });
  });

  it('collapses read-only sessions back to inventory and hides setup sections', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    mockPathname = '/settings/infrastructure/install';
    renderWorkspace();

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', { replace: true });
    expect(screen.queryByRole('button', { name: /Add infrastructure/i })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Connections' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Agent profiles' })).toBeNull();
    expect(screen.queryByTestId('platform-section')).toBeNull();
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.queryByTestId('agent-profiles')).toBeNull();
  });
});
