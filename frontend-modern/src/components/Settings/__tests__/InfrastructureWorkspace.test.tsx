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

vi.mock('../PlatformConnectionsWorkspace', () => ({
  PlatformConnectionsWorkspace: () => <div data-testid="platform-connections">platforms</div>,
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
        'Start with Install on a host to connect the first machine you want Pulse to monitor. If you already know you want an API-backed platform such as Proxmox or TrueNAS instead, go straight to Platform connections.',
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
    expect(screen.getByRole('tab', { name: 'Platform connections' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
    expect(screen.getByRole('tab', { name: 'Reporting & control' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
    expect(screen.getByTestId('unified-agents')).toBeInTheDocument();
  });

  it('uses the shared subtabs to switch to platform connections', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('tab', { name: 'Platform connections' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms');
  });

  it('renders the platform workspace from the router pathname', () => {
    mockPathname = '/settings/infrastructure/platforms';
    renderWorkspace();

    expect(screen.getByRole('tab', { name: 'Platform connections' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    expect(screen.getByTestId('platform-connections')).toBeInTheDocument();
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

  it('uses the guided workspace actions to open install and platform paths', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('button', { name: 'Install on a host selected' }));
    fireEvent.click(screen.getByRole('button', { name: 'Open Platform connections' }));

    expect(navigateSpy).toHaveBeenNthCalledWith(1, '/settings/infrastructure/install');
    expect(navigateSpy).toHaveBeenNthCalledWith(2, '/settings/infrastructure/platforms');
  });

  it('returns to the base settings route when switching away from platform connections', () => {
    mockPathname = '/settings/infrastructure/platforms';
    renderWorkspace();

    fireEvent.click(screen.getByRole('tab', { name: 'Install on a host' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/install');
  });
});
