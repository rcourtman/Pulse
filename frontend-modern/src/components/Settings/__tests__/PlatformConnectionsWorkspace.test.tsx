import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PlatformConnectionsWorkspace } from '../PlatformConnectionsWorkspace';

let mockPathname = '/settings/infrastructure/platforms/proxmox';
const navigateSpy = vi.hoisted(() => vi.fn());
const trueNASStateSpy = vi.hoisted(() => vi.fn());
const vmwareStateSpy = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockPathname }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('../ProxmoxSettingsPanel', () => ({
  ProxmoxSettingsPanel: () => <div data-testid="proxmox-settings">proxmox</div>,
}));

vi.mock('../TrueNASSettingsPanel', () => ({
  TrueNASSettingsPanel: (props: { state: unknown }) => {
    trueNASStateSpy(props.state);
    return <div data-testid="truenas-settings">truenas</div>;
  },
}));

vi.mock('../VMwareSettingsPanel', () => ({
  VMwareSettingsPanel: (props: { state: unknown }) => {
    vmwareStateSpy(props.state);
    return <div data-testid="vmware-settings">vmware</div>;
  },
}));

describe('PlatformConnectionsWorkspace', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    trueNASStateSpy.mockReset();
    vmwareStateSpy.mockReset();
    mockPathname = '/settings/infrastructure/platforms/proxmox';
  });

  afterEach(() => {
    cleanup();
  });

  const renderWorkspace = () =>
    render(
      () =>
        (
          <PlatformConnectionsWorkspace
            {...({
              pveNodes: () => [],
              pbsNodes: () => [],
              pmgNodes: () => [],
              trueNASSettings: { connections: () => [], featureDisabled: () => false },
              vmwareSettings: { connections: () => [], featureDisabled: () => false },
            } as any)}
          />
        ) as any,
    );

  it('renders the Proxmox workspace by default', () => {
    renderWorkspace();

    expect(screen.getByRole('tab', { name: 'Proxmox' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'TrueNAS' })).toHaveAttribute('aria-selected', 'false');
    expect(screen.getByRole('tab', { name: 'VMware' })).toHaveAttribute('aria-selected', 'false');
    expect(screen.getByTestId('proxmox-settings')).toBeInTheDocument();
  });

  it('navigates to the canonical TrueNAS route from the shared subtabs', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('tab', { name: 'TrueNAS' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/truenas');
  });

  it('navigates to the canonical VMware route from the shared subtabs', () => {
    renderWorkspace();

    fireEvent.click(screen.getByRole('tab', { name: 'VMware' }));

    expect(navigateSpy).toHaveBeenCalledWith('/settings/infrastructure/platforms/vmware');
  });

  it('treats legacy TrueNAS routes as the TrueNAS workspace', () => {
    mockPathname = '/settings/infrastructure/truenas';
    renderWorkspace();

    expect(screen.getByRole('tab', { name: 'TrueNAS' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('truenas-settings')).toBeInTheDocument();
    expect(trueNASStateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        connections: expect.any(Function),
        featureDisabled: expect.any(Function),
      }),
    );
  });

  it('treats the canonical VMware route as the VMware workspace', () => {
    mockPathname = '/settings/infrastructure/platforms/vmware';
    renderWorkspace();

    expect(screen.getByRole('tab', { name: 'VMware' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('vmware-settings')).toBeInTheDocument();
    expect(vmwareStateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        connections: expect.any(Function),
        featureDisabled: expect.any(Function),
      }),
    );
  });
});
