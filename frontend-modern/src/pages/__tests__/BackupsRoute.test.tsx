import { cleanup, render, waitFor, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import BackupsRoute from '@/pages/BackupsRoute';

let mockLocationSearch = '';
let mockLocationPath = '/backups';
const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockLocationPath, search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/components/Backups/Backups', () => ({
  default: () => <div data-testid="backups-component">Backups Component</div>,
}));

describe('BackupsRoute', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    mockLocationPath = '/backups';
    mockLocationSearch = '';
  });

  afterEach(() => cleanup());

  it('redirects legacy query params to the canonical v6 Backups URL and does not render the page during redirect', async () => {
    mockLocationSearch = '?view=artifacts&backupType=remote&group=guest&search=vm-101&source=pbs&type=vm';
    render(() => <BackupsRoute />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledTimes(1);
    });

    const [target, opts] = navigateSpy.mock.calls[0] as [string, { replace?: boolean }];
    expect(target).toContain('/backups?');
    expect(target).toContain('view=events');
    expect(target).toContain('mode=remote');
    expect(target).toContain('scope=workload');
    expect(target).toContain('provider=proxmox-pbs');
    expect(target).toContain('q=vm-101');
    expect(target).not.toContain('type=');
    expect(opts?.replace).toBe(true);

    expect(screen.queryByTestId('backups-component')).not.toBeInTheDocument();
  });

  it('renders Backups when query params are already canonical', async () => {
    mockLocationSearch = '?view=events&mode=remote&scope=workload&q=vm-101';
    render(() => <BackupsRoute />);

    await waitFor(() => {
      expect(screen.getByTestId('backups-component')).toBeInTheDocument();
    });
    expect(navigateSpy).not.toHaveBeenCalled();
  });
});
