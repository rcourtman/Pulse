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

  it('renders Recovery without query rewrite redirects', async () => {
    mockLocationSearch = '?view=artifacts&backupType=remote&group=guest&search=vm-101&source=pbs';
    render(() => <RecoveryRoute />);

    await waitFor(() => {
      expect(screen.getByTestId('recovery-component')).toBeInTheDocument();
    });
    expect(navigateSpy).not.toHaveBeenCalled();
  });
});
