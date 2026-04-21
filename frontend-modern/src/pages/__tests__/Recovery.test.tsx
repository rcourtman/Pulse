import { cleanup, render, waitFor, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import appSource from '@/App.tsx?raw';
import recoveryPageSource from '@/pages/Recovery.tsx?raw';
import routePreloadSource from '@/routing/routePreload.ts?raw';

import RecoveryPage from '@/pages/Recovery';

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

describe('Recovery page route shell', () => {
  beforeEach(() => {
    navigateSpy.mockReset();
    mockLocationPath = '/recovery';
    mockLocationSearch = '';
  });

  afterEach(() => cleanup());

  it('keeps App and route preloading on the recovery page shell', () => {
    expect(appSource).toContain("const RecoveryPage = lazy(() => import('./pages/Recovery'));");
    expect(appSource).toContain('<Route path={RECOVERY_ROUTE_PATH} component={RecoveryPage} />');
    expect(routePreloadSource).toContain("import('@/pages/Recovery').then(() => undefined)");
    expect(recoveryPageSource).toContain("import RecoverySurface from '@/components/Recovery/Recovery';");
    expect(recoveryPageSource).toContain('<RecoverySurface />');
    expect(recoveryPageSource).not.toContain('useLocation(');
    expect(recoveryPageSource).not.toContain('useNavigate(');
  });

  it('renders Recovery without query rewrite redirects', async () => {
    mockLocationSearch = '?view=artifacts&backupType=remote&group=guest&search=vm-101&source=pbs';
    render(() => <RecoveryPage />);

    await waitFor(() => {
      expect(screen.getByTestId('recovery-component')).toBeInTheDocument();
    });
    expect(navigateSpy).not.toHaveBeenCalled();
  });

  it('preserves focused rollup routes without redirecting away from the selected history view', async () => {
    mockLocationSearch = '?view=events&rollupId=res%3Avm-123';
    render(() => <RecoveryPage />);

    await waitFor(() => {
      expect(screen.getByTestId('recovery-component')).toBeInTheDocument();
    });
    expect(navigateSpy).not.toHaveBeenCalled();
  });
});
