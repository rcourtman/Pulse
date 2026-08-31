import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import HomePageSurface from '../HomePageSurface';

const resourcesMock = vi.hoisted(() => vi.fn());

vi.mock('@/hooks/useResources', () => ({
  useResources: () => resourcesMock(),
}));

const resource = (overrides: Partial<Resource>): Resource => ({
  id: 'host-1',
  type: 'agent',
  name: 'Host One',
  displayName: 'Host One',
  platformId: 'pve-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  health: { verdict: 'ok', reasons: [] },
  ...overrides,
});

const renderHome = (items: Resource[]) => {
  resourcesMock.mockReturnValue({
    resources: () => items,
    loading: () => false,
    error: () => undefined,
    refetch: vi.fn(async () => items),
  });
  window.history.replaceState({}, '', '/home');
  return render(() => (
    <Router>
      <Route path="/home" component={HomePageSurface} />
    </Router>
  ));
};

describe('HomePageSurface', () => {
  afterEach(() => {
    cleanup();
    resourcesMock.mockReset();
    window.history.replaceState({}, '', '/');
  });

  it('presents attention before platform groups with descriptive resource links', () => {
    renderHome([
      resource({
        id: 'critical-host',
        name: 'Critical Host',
        displayName: 'Critical Host',
        health: { verdict: 'critical', reasons: [{ code: 'offline' }] },
      }),
      resource({ id: 'healthy-host', name: 'Healthy Host', displayName: 'Healthy Host' }),
    ]);

    expect(screen.getByRole('heading', { level: 1, name: 'Home' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Needs attention' })).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Critical Host: Critical. Offline' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Healthy Host: Healthy. Healthy' }),
    ).toBeInTheDocument();
  });

  it('offers infrastructure onboarding rather than an unexplained blank page', () => {
    renderHome([]);
    expect(
      screen.getByRole('heading', { name: 'Connect your first monitored system' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Add infrastructure' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
  });
});
