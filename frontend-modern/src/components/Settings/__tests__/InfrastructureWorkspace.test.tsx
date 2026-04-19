import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { UnifiedAgentRow } from '../infrastructureOperationsModel';
import { InfrastructureWorkspace } from '../InfrastructureWorkspace';

let mockPathname = '/settings/infrastructure';
let mockSearch = '';
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
    useLocation: () => ({ pathname: mockPathname, search: mockSearch }),
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
    setupHandoff: () => null,
  }),
}));

vi.mock('../InfrastructureInstallerSection', () => ({
  InfrastructureInstallerSection: () => <div data-testid="install-section">install</div>,
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

vi.mock('@/api/connections', async () => {
  const actual = await vi.importActual<typeof import('@/api/connections')>('@/api/connections');
  return {
    ...actual,
    ConnectionsAPI: {
      list: vi.fn(),
      probe: vi.fn().mockResolvedValue({ candidates: [], probedMs: 0 }),
    },
  };
});

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

const setShowNodeModalSpy = vi.fn();
const setEditingNodeSpy = vi.fn();
const setCurrentNodeTypeSpy = vi.fn();
const setModalResetKeySpy = vi.fn();

const baseProps = () =>
  ({
    pveNodes: () => [{ id: 'pve-1', name: 'zeus', host: '10.0.0.1', type: 'pve', status: 'connected' }],
    pbsNodes: () => [],
    pmgNodes: () => [],
    agentStateResources: () => [],
    trueNASSettings: {
      connections: () => [{ id: 'tn-1', name: 'Tower NAS', host: '10.0.0.20', enabled: true }],
      openCreateDialog: vi.fn(),
      closeDialog: vi.fn(),
      closeDeleteDialog: vi.fn(),
      openEditDialog: vi.fn(),
    },
    vmwareSettings: {
      connections: () => [{ id: 'vm-1', name: 'lab-vcenter', host: '10.0.0.30', enabled: true }],
      openCreateDialog: vi.fn(),
      closeDialog: vi.fn(),
      closeDeleteDialog: vi.fn(),
      openEditDialog: vi.fn(),
    },
    selectedAgent: () => 'pve',
    onSelectAgent: vi.fn(),
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
    setShowNodeModalSpy.mockReset();
    setEditingNodeSpy.mockReset();
    setCurrentNodeTypeSpy.mockReset();
    setModalResetKeySpy.mockReset();
    mockPathname = '/settings/infrastructure';
    mockSearch = '';
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

  it('renders the inventory table at the base infrastructure route', () => {
    renderWorkspace();

    expect(screen.getByRole('heading', { name: 'Monitored systems' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add connection' })).toBeInTheDocument();
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.queryByTestId('proxmox-section')).toBeNull();
    expect(screen.queryByTestId('agent-profiles')).toBeNull();
  });

  it('shows only monitored agents in the inventory table', () => {
    renderWorkspace();

    expect(screen.getByText('tower')).toBeInTheDocument();
    expect(screen.queryByText('zeus')).toBeNull();
    expect(screen.queryByText('Tower NAS')).toBeNull();
    expect(screen.queryByText('lab-vcenter')).toBeNull();
  });

  it('opens the unified add drawer with the probe step when Add infrastructure is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));

    expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/pve01\.lan/)).toBeInTheDocument();
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('routes to the agent credential slot when the user picks agent manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /Agent \(install on host\)/i }));

    expect(screen.getByTestId('install-section')).toBeInTheDocument();
  });

  it('routes to the TrueNAS credential slot when TrueNAS is picked manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /TrueNAS SCALE/i }));

    expect(screen.getByTestId('truenas-section')).toBeInTheDocument();
  });

  it('routes to the VMware credential slot when VMware is picked manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /VMware vCenter/i }));

    expect(screen.getByTestId('vmware-section')).toBeInTheDocument();
  });

  it('routes to the Proxmox credential slot when Proxmox VE is picked manually', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Proxmox VE/i }));

    expect(screen.getByTestId('proxmox-section')).toBeInTheDocument();
  });

  it('can return to the probe step from a credential slot via Back to probe', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /Agent \(install on host\)/i }));
    fireEvent.click(screen.getByRole('button', { name: /Back to probe/i }));

    expect(screen.getByRole('button', { name: /Probe address/i })).toBeInTheDocument();
    expect(screen.queryByTestId('install-section')).toBeNull();
  });

  it('toggles agent profiles inside the agent credential slot', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /Agent \(install on host\)/i }));
    fireEvent.click(screen.getByRole('button', { name: 'Manage agent profiles' }));

    expect(screen.getByTestId('agent-profiles')).toBeInTheDocument();
  });

  it('does not mount node-modal legacy spies when a credential slot is rendered', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add connection/i }));
    fireEvent.click(screen.getByRole('button', { name: /Enter credentials manually/i }));
    fireEvent.click(screen.getByRole('button', { name: /^Proxmox VE/i }));

    expect(setShowNodeModalSpy).not.toHaveBeenCalled();
    expect(setEditingNodeSpy).not.toHaveBeenCalled();
    expect(setCurrentNodeTypeSpy).not.toHaveBeenCalled();
    expect(setModalResetKeySpy).not.toHaveBeenCalled();
  });

  it('opens reporting details from the inventory in a drawer', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: 'View details' }));

    expect(setExpandedRowKeySpy).toHaveBeenCalledWith('agent:tower');
    expect(screen.getByTestId('active-details')).toBeInTheDocument();
  });

  it('redirects legacy install deep link and pre-selects the agent credential slot', async () => {
    mockPathname = '/settings/infrastructure/install';
    renderWorkspace();

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', { replace: true });
    await waitFor(() => expect(screen.getByTestId('install-section')).toBeInTheDocument());
  });

  it('redirects legacy truenas deep link and pre-selects the TrueNAS credential slot', async () => {
    mockPathname = '/settings/infrastructure/platforms/truenas';
    renderWorkspace();

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', { replace: true });
    await waitFor(() => expect(screen.getByTestId('truenas-section')).toBeInTheDocument());
  });

  it('hides Add infrastructure and the add drawer in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    renderWorkspace();

    expect(screen.queryByRole('button', { name: /Add connection/i })).toBeNull();
    expect(screen.queryByTestId('install-section')).toBeNull();
    expect(screen.queryByRole('button', { name: /Probe address/i })).toBeNull();
  });

  it('collapses read-only sessions back to the inventory and redirects', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    mockPathname = '/settings/infrastructure/install';
    renderWorkspace();

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', { replace: true });
    expect(screen.queryByTestId('install-section')).toBeNull();
  });
});
