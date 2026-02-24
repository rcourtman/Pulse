import { cleanup, render, waitFor, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import RecoveryRoute from '@/pages/RecoveryRoute';

let mockLocationSearch = '';
let mockLocationPath = '/recovery';
const navigateSpy = vi.hoisted(() => vi.fn());

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: mockLocationPath, search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/components/Recovery/Recovery', () => ({
  default: () => <div data-testid="recovery-component">Recovery Component</div>,
}));

describe('RecoveryRoute', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    mockLocationPath = '/recovery';
    mockLocationSearch = '';
  });

  afterEach(() => cleanup());

  it('redirects legacy query params to the canonical v6 Recovery URL and does not render the page during redirect', async () => {
    mockLocationSearch =
      '?view=artifacts&backupType=remote&group=guest&search=vm-101&source=pbs&type=vm';
    render(() => <RecoveryRoute />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledTimes(1);
    });

    const [target, opts] = navigateSpy.mock.calls[0] as [string, { replace?: boolean }];
    expect(target).toContain('/recovery?');
    expect(target).toContain('view=events');
    expect(target).toContain('mode=remote');
    expect(target).toContain('scope=workload');
    expect(target).toContain('provider=proxmox-pbs');
    expect(target).toContain('q=vm-101');
    expect(target).not.toContain('type=');
    expect(opts?.replace).toBe(true);

    expect(screen.queryByTestId('recovery-component')).not.toBeInTheDocument();
  });

  it('renders Recovery when query params are already canonical', async () => {
    mockLocationSearch = '?view=events&mode=remote&scope=workload&q=vm-101';
    render(() => <RecoveryRoute />);

    await waitFor(() => {
      expect(screen.getByTestId('recovery-component')).toBeInTheDocument();
    });
    expect(navigateSpy).not.toHaveBeenCalled();
  });
});
