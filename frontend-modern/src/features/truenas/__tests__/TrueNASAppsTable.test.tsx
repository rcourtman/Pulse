import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { TrueNASAppsTable } from '@/features/truenas/TrueNASAppsTable';
import type { Resource } from '@/types/resource';

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    getMetricThresholds: () => null,
  }),
}));

beforeAll(() => {
  if (typeof globalThis.ResizeObserver === 'undefined') {
    globalThis.ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    } as typeof ResizeObserver;
  }
});

const makeApp = (id: string, state: string, overrides: Partial<Resource> = {}): Resource =>
  ({
    id,
    type: 'app-container',
    name: id,
    displayName: id,
    parentName: 'truenas-main',
    status: state === 'RUNNING' ? 'online' : state === 'STOPPED' ? 'offline' : 'degraded',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    cpu: { current: 12 },
    memory: { used: 512 * 1024 * 1024, total: 2 * 1024 * 1024 * 1024 },
    truenas: {
      hostname: 'truenas-main',
      app: {
        id,
        name: id,
        state,
        humanVersion: '29.0.7',
        containerCount: 2,
      },
    },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASAppsTable', () => {
  it('labels stale running apps as Stale data instead of amber Running and preserves Crashed state', () => {
    const staleRunning = makeApp('nextcloud', 'RUNNING', {
      sourceStatus: { truenas: { status: 'stale' } },
    });
    const staleCrashed = makeApp('immich', 'CRASHED', {
      sourceStatus: { truenas: { status: 'error', error: 'WebSocket read timed out' } },
    });
    const recoveredRunning = makeApp('jellyfin', 'RUNNING', {
      sourceStatus: { truenas: { status: 'online' } },
      platformData: { sourceStatus: { truenas: { status: 'stale' } } },
    });

    const apps = [staleRunning, staleCrashed, recoveredRunning];
    const { container } = render(() => (
      <TrueNASAppsTable
        apps={apps}
        scope={apps}
        emptyIcon={<span />}
        emptyTitle="No apps"
        emptyDescription="No apps"
        showToolbar={false}
      />
    ));

    const row = (id: string) =>
      within(container.querySelector(`[data-truenas-app-row="${id}"]`) as HTMLElement);

    const nextcloudState = row('nextcloud')
      .getByText('Stale data')
      .closest('[data-truenas-app-state]');
    expect(nextcloudState).toHaveAttribute('data-truenas-app-state', 'warning');
    expect(nextcloudState).toHaveAttribute(
      'title',
      'Last reported Running · TrueNAS has not updated this recently',
    );
    expect(nextcloudState).not.toHaveTextContent('Running');

    const immichState = row('immich')
      .getByText('Crashed · Stale data')
      .closest('[data-truenas-app-state]');
    expect(immichState).toHaveAttribute('data-truenas-app-state', 'danger');
    expect(immichState).toHaveAttribute(
      'title',
      'Last reported Crashed · TrueNAS collection error: WebSocket read timed out',
    );

    const jellyfinState = container.querySelector(
      '[data-truenas-app-row="jellyfin"] [data-truenas-app-state]',
    );
    expect(jellyfinState).toHaveAttribute('data-truenas-app-state', 'success');
    expect(jellyfinState).toHaveClass('sr-only');
    expect(screen.queryByText('Stale data')).toBeInTheDocument();
  });
});
