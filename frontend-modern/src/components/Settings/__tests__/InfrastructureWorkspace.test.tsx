import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { InfrastructureWorkspace } from '../InfrastructureWorkspace';

let mockPathname = '/settings/infrastructure';
const navigateSpy = vi.hoisted(() => vi.fn());
const presentationPolicyIsReadOnlyMock = vi.hoisted(() => vi.fn(() => false));

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

vi.mock('../InfrastructureInstallPanel', () => ({
  InfrastructureInstallPanel: () => <div data-testid="install-panel">install</div>,
}));

vi.mock('../InfrastructureReportingPanel', () => ({
  InfrastructureReportingPanel: () => <div data-testid="reporting-panel">operations</div>,
}));

vi.mock('../PlatformConnectionsWorkspace', () => ({
  PlatformConnectionsWorkspace: () => <div data-testid="platform-connections">platforms</div>,
}));

const onSelectAgentSpy = vi.fn();

const baseProps = () =>
  ({
    pveNodes: () => [],
    pbsNodes: () => [],
    pmgNodes: () => [],
    agentStateResources: () => [],
    trueNASSettings: { connections: () => [] },
    vmwareSettings: { connections: () => [] },
    platformConnectionsSummary: () => ({
      pveCount: 0,
      pbsCount: 0,
      pmgCount: 0,
      truenasCount: 0,
      truenasAvailable: true,
      vmwareCount: 0,
      vmwareAvailable: true,
    }),
    selectedAgent: () => 'pve',
    onSelectAgent: onSelectAgentSpy,
  }) as any;

describe('InfrastructureWorkspace', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    presentationPolicyIsReadOnlyMock.mockReset();
    presentationPolicyIsReadOnlyMock.mockReturnValue(false);
    onSelectAgentSpy.mockReset();
    mockPathname = '/settings/infrastructure';
  });

  afterEach(() => {
    cleanup();
  });

  const renderWorkspace = (propOverrides: Record<string, unknown> = {}) =>
    render(() => (<InfrastructureWorkspace {...{ ...baseProps(), ...propOverrides }} />) as any);

  it('renders the unified connections table at the base infrastructure route', () => {
    renderWorkspace();

    expect(screen.getByText('Connections')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Add a system/i })).toBeInTheDocument();
    expect(screen.queryByTestId('install-panel')).toBeNull();
    expect(screen.queryByTestId('platform-connections')).toBeNull();
  });

  it('merges every connection source into a single alpha-sorted table', () => {
    renderWorkspace({
      pveNodes: () => [
        { id: 'n1', name: 'zeus', host: '10.0.0.1', type: 'pve', status: 'connected' },
      ],
      agentStateResources: () => [
        { id: 'a1', name: 'tower', displayName: 'tower', status: 'online', lastSeen: Date.now() },
      ],
      trueNASSettings: {
        connections: () => [
          { id: 't1', name: 'nas.home', host: '10.0.0.2', enabled: true, insecureSkipVerify: false, useHttps: true },
        ],
      },
    });

    const rowNames = screen.getAllByText(/zeus|tower|nas\.home/).map((el) => el.textContent);
    expect(rowNames).toContain('nas.home');
    expect(rowNames).toContain('tower');
    expect(rowNames).toContain('zeus');
  });

  it('opens the add-system picker when the add button is clicked', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add a system/i }));

    expect(screen.getByText('Linux or Docker host (agent)')).toBeInTheDocument();
    expect(screen.getByText('Proxmox VE')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS SCALE')).toBeInTheDocument();
  });

  it('routes the agent-host choice to the dedicated install workspace', () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole('button', { name: /Add a system/i }));

    fireEvent.click(screen.getByText('Linux or Docker host (agent)'));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/install');
  });

  it('routes TrueNAS and VMware choices to their platform panels', () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole('button', { name: /Add a system/i }));
    fireEvent.click(screen.getByText('TrueNAS SCALE'));
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/truenas');

    fireEvent.click(screen.getByRole('button', { name: /Add a system/i }));
    fireEvent.click(screen.getByText('VMware vSphere or ESXi'));
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/vmware');
  });

  it('preselects the Proxmox kind and lands on the platforms route for PVE, PBS, and PMG', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: /Add a system/i }));
    fireEvent.click(screen.getByText('Proxmox Backup Server'));

    expect(onSelectAgentSpy).toHaveBeenCalledWith('pbs');
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/proxmox');
  });

  it('renders the install workspace when the URL is /install', () => {
    mockPathname = '/settings/infrastructure/install';
    renderWorkspace();
    expect(screen.getByTestId('install-panel')).toBeInTheDocument();
  });

  it('renders the platforms workspace when the URL is under /platforms', () => {
    mockPathname = '/settings/infrastructure/platforms/truenas';
    renderWorkspace();
    expect(screen.getByTestId('platform-connections')).toBeInTheDocument();
  });

  it('keeps /operations reachable as a legacy detail route', () => {
    mockPathname = '/settings/infrastructure/operations';
    renderWorkspace();
    expect(screen.getByTestId('reporting-panel')).toBeInTheDocument();
  });

  it('renders a back-to-inventory header on the install subview', () => {
    mockPathname = '/settings/infrastructure/install';
    renderWorkspace();

    const backButton = screen.getByRole('button', { name: /Connections and Inventory/i });
    expect(backButton).toBeInTheDocument();
    expect(screen.getByText('Install on a host')).toBeInTheDocument();

    fireEvent.click(backButton);
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure');
  });

  it('renders a back-to-inventory header on the platforms subview', () => {
    mockPathname = '/settings/infrastructure/platforms/truenas';
    renderWorkspace();

    const backButton = screen.getByRole('button', { name: /Connections and Inventory/i });
    expect(backButton).toBeInTheDocument();
    expect(screen.getByText('Platform connections')).toBeInTheDocument();

    fireEvent.click(backButton);
    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure');
  });

  it('does not render the back-to-inventory header on the inventory landing', () => {
    mockPathname = '/settings/infrastructure';
    renderWorkspace();

    expect(screen.queryByRole('button', { name: /^Connections and Inventory$/ })).toBeNull();
  });

  it('hides the add-system action and redirects install routes in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    mockPathname = '/settings/infrastructure/install';
    renderWorkspace();

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure', { replace: true });
  });

  it('still renders the connections table without an add button in read-only mode', () => {
    presentationPolicyIsReadOnlyMock.mockReturnValue(true);
    mockPathname = '/settings/infrastructure';
    renderWorkspace();

    expect(screen.getByText('Connections')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Add a system/i })).toBeNull();
  });
});
