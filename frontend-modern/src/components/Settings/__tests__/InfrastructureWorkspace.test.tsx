import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from '@/utils/licensePresentation';
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
    mockPathname = '/settings/infrastructure/operations';
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

  it('renders the canonical subtablist', () => {
    renderWorkspace();

    const tablist = screen.getByRole('tablist', { name: 'Infrastructure workspace' });
    expect(tablist).toBeInTheDocument();
    expect(screen.getByText('Infrastructure operations')).toBeInTheDocument();
    expect(
      screen.getByText(
        'New to Pulse? Start with Install on a host to add your first monitored system. If you prefer a direct integration instead, use Direct Proxmox.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(SELF_HOSTED_PRO_BILLING_PRESENTATION.infrastructureWorkspaceReferral),
    ).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Install on a host' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
    expect(screen.getByRole('tab', { name: 'Direct Proxmox' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
    expect(screen.getByRole('tab', { name: 'Reporting & control' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
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

  it('returns to the base settings route when switching away from direct proxmox', () => {
    mockPathname = '/settings/infrastructure/proxmox';
    renderWorkspace();

    fireEvent.click(screen.getByRole('tab', { name: 'Reporting & control' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/operations');
  });
});
