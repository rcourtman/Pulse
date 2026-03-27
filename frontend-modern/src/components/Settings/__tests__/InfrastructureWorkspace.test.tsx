import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from '../selfHostedBillingPresentation';
import { InfrastructureWorkspace } from '../InfrastructureWorkspace';

let mockPathname = '/settings';
const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockPathname }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('../InfrastructureInstallPanel', () => ({
  InfrastructureInstallPanel: () => <div data-testid="unified-agents">install</div>,
}));

vi.mock('../InfrastructureReportingPanel', () => ({
  InfrastructureReportingPanel: () => <div data-testid="agent-profiles">profiles</div>,
}));

vi.mock('../ProxmoxSettingsPanel', () => ({
  ProxmoxSettingsPanel: () => <div data-testid="proxmox-settings">direct</div>,
}));

describe('InfrastructureWorkspace', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    mockPathname = '/settings/infrastructure';
  });

  afterEach(() => {
    cleanup();
  });

  const renderWorkspace = () =>
    render(
      () =>
        (
          <InfrastructureWorkspace
            {...({
              pveNodes: () => [],
              pbsNodes: () => [],
              pmgNodes: () => [],
            } as any)}
          />
        ) as any,
    );

  it('defaults bare infrastructure routing to install on a host', () => {
    renderWorkspace();

    const tablist = screen.getByRole('tablist', { name: 'Infrastructure workspace' });
    expect(tablist).toBeInTheDocument();
    expect(screen.getByText('Connect your first system')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Start with Install on a host to connect the first machine you want Pulse to monitor. If you already know you want a direct integration instead, go straight to Direct Proxmox.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('1. Choose path')).toBeInTheDocument();
    expect(screen.getByText('2. Generate access')).toBeInTheDocument();
    expect(screen.getByText('3. Confirm reporting')).toBeInTheDocument();
    expect(
      screen.getByText(SELF_HOSTED_PRO_BILLING_PRESENTATION.infrastructureWorkspaceReferral),
    ).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Install on a host' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByRole('tab', { name: 'Direct Proxmox' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
    expect(screen.getByRole('tab', { name: 'Reporting & control' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
    expect(screen.getByTestId('unified-agents')).toBeInTheDocument();
  });

  it('uses the shared subtabs to switch to direct proxmox', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('tab', { name: 'Direct Proxmox' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/proxmox');
  });

  it('renders the direct workspace from the router pathname', () => {
    mockPathname = '/settings/infrastructure/proxmox';
    renderWorkspace();

    expect(screen.getByRole('tab', { name: 'Direct Proxmox' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByTestId('proxmox-settings')).toBeInTheDocument();
  });

  it('keeps the reporting route available for established operators', () => {
    mockPathname = '/settings/infrastructure/operations';
    renderWorkspace();

    expect(screen.getByRole('tab', { name: 'Reporting & control' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByTestId('agent-profiles')).toBeInTheDocument();
  });

  it('uses the guided workspace actions to open install and direct paths', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: 'Install on a host selected' }));
    fireEvent.click(screen.getByRole('button', { name: 'Open Direct Proxmox' }));

    expect(navigateSpy).toHaveBeenNthCalledWith(1, '/settings/infrastructure/install');
    expect(navigateSpy).toHaveBeenNthCalledWith(2, '/settings/infrastructure/proxmox');
  });

  it('returns to the base settings route when switching away from direct proxmox', () => {
    mockPathname = '/settings/infrastructure/proxmox';
    renderWorkspace();

    fireEvent.click(screen.getByRole('tab', { name: 'Install on a host' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
