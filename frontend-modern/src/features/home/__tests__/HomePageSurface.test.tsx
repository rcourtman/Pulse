import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
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
    expect(screen.getByRole('link', { name: 'Healthy Host: Healthy. Healthy' })).toHaveTextContent(
      'Healthy',
    );
  });

  it('warns when a refresh fails while retaining the last loaded resource state', async () => {
    const [error, setError] = createSignal<unknown>();
    const refetch = vi.fn(async () => {
      const failure = new Error('refresh failed');
      setError(failure);
      throw failure;
    });
    resourcesMock.mockReturnValue({
      resources: () => [resource({})],
      loading: () => false,
      error,
      refetch,
    });
    window.history.replaceState({}, '', '/home');
    render(() => (
      <Router>
        <Route path="/home" component={HomePageSurface} />
      </Router>
    ));

    await fireEvent.click(screen.getByRole('button', { name: 'Refresh fleet health' }));

    await waitFor(() => expect(refetch).toHaveBeenCalledOnce());
    expect(screen.getByRole('alert')).toHaveTextContent('Fleet health could not be refreshed');
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Showing the last loaded data. Resource statuses may be out of date.',
    );
    expect(screen.getByRole('link', { name: 'Host One: Healthy. Healthy' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
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

  it('identifies the resource region controlled by each group disclosure', async () => {
    renderHome(
      Array.from({ length: 61 }, (_, index) =>
        resource({
          id: `host-${index}`,
          name: `Host ${index}`,
          displayName: `Host ${index}`,
        }),
      ),
    );

    const disclosure = screen.getByRole('button', { name: 'Show all (1)' });
    expect(disclosure).toHaveAttribute('aria-expanded', 'false');
    expect(disclosure).toHaveAttribute('aria-controls', 'home-group-proxmox-resources');
    expect(document.getElementById('home-group-proxmox-resources')).toBeInTheDocument();

    disclosure.focus();
    await fireEvent.click(disclosure);

    expect(disclosure).toHaveAttribute('aria-expanded', 'true');
    expect(disclosure).toHaveFocus();
    expect(screen.getByRole('link', { name: 'Host 60: Healthy. Healthy' })).toBeInTheDocument();
  });
});
