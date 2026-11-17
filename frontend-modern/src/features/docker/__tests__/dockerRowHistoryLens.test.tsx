import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import type { WorkloadsMetricHoverMode } from '@/components/Workloads/workloadsFilterModel';
import { createDockerRowHistoryLens, type DockerRowHistoryLens } from '../dockerRowHistoryLens';
import { DockerHostsTable } from '../DockerHostsTable';
import { DockerContainersTable } from '../DockerContainersTable';

const getMetricsHistoryMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/charts', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/charts')>();
  return {
    ...original,
    ChartsAPI: { ...original.ChartsAPI, getMetricsHistory: getMetricsHistoryMock },
  };
});

vi.mock('@/api/monitoring', () => ({ MonitoringAPI: {} }));
vi.mock('@/stores/systemSettings', () => ({
  areSystemSettingsLoaded: () => true,
  shouldHideDockerUpdateActions: () => false,
}));
vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: {} }),
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 80, critical: 90 }),
  }),
}));

const host: Resource = {
  id: 'agent:docker-01',
  name: 'docker-01',
  displayName: 'docker-01',
  platformId: 'homelab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'agent',
  lastSeen: Date.now(),
  cpu: { current: 42 },
  memory: { total: 8_000, used: 3_200, free: 4_800, current: 40 },
  disk: { total: 20_000, used: 12_500, free: 7_500, current: 62.5 },
  temperature: 61,
};

const container: Resource = {
  id: 'docker-01/web',
  name: 'web',
  displayName: 'web',
  platformId: 'homelab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'running',
  type: 'app-container',
  lastSeen: Date.now(),
  cpu: { current: 12 },
  memory: { total: 1_000, used: 250, free: 750, current: 25 },
  metricsTarget: { resourceType: 'app-container', resourceId: 'docker-01:web' },
  docker: { hostname: 'docker-01', containerState: 'running', image: 'nginx:latest' },
};

const points = [
  { timestamp: 1, value: 10 },
  { timestamp: 2, value: 20 },
];

/** A lens whose every row already has history, for the table tests. */
const fakeLens = (mode: () => WorkloadsMetricHoverMode): DockerRowHistoryLens => ({
  hoverMode: mode,
  setHoverMode: () => undefined,
  range: () => '1h',
  setRange: () => undefined,
  previewRow: vi.fn(),
  clearPreview: vi.fn(),
  seriesFor: (_row, metric) => [{ id: metric, label: metric, color: '#000', points }],
});

beforeEach(() => {
  // jsdom has no ResizeObserver; the resting bars measure their width with one.
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  localStorage.clear();
  getMetricsHistoryMock.mockReset();
  getMetricsHistoryMock.mockImplementation(async (request: { resourceId: string }) => ({
    resourceType: 'agent',
    resourceId: request.resourceId,
    range: '1h',
    start: 0,
    end: 2,
    metrics: { cpu: points, memory: points, disk: points, temperature: points },
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('createDockerRowHistoryLens', () => {
  it('reads nothing until a row is previewed, then reads that row first', async () => {
    await createRoot(async (dispose) => {
      const lens = createDockerRowHistoryLens({
        hosts: () => [host],
        containers: () => [container],
      });
      await Promise.resolve();
      expect(getMetricsHistoryMock).not.toHaveBeenCalled();

      lens.previewRow(container);
      await vi.waitFor(() => expect(getMetricsHistoryMock).toHaveBeenCalled());
      expect(getMetricsHistoryMock.mock.calls[0][0]).toMatchObject({
        resourceType: 'app-container',
        resourceId: 'docker-01:web',
        range: '1h',
      });
      await vi.waitFor(() => expect(lens.seriesFor(container, 'cpu')[0]?.points).toHaveLength(2));
      dispose();
    });
  });

  it('reads a host through its agent history target', async () => {
    await createRoot(async (dispose) => {
      const lens = createDockerRowHistoryLens({ hosts: () => [host], containers: () => [] });
      lens.previewRow(host);
      await vi.waitFor(() => expect(getMetricsHistoryMock).toHaveBeenCalled());
      expect(getMetricsHistoryMock.mock.calls[0][0]).toMatchObject({
        resourceType: 'agent',
        resourceId: 'docker-01',
      });
      await vi.waitFor(() =>
        expect(lens.seriesFor(host, 'temperature')[0]?.points).toHaveLength(2),
      );
      dispose();
    });
  });

  it('reads nothing in Details hover', async () => {
    localStorage.setItem('workloadsMetricHoverMode', 'details');
    await createRoot(async (dispose) => {
      const lens = createDockerRowHistoryLens({
        hosts: () => [host],
        containers: () => [container],
      });
      expect(lens.hoverMode()).toBe('details');
      lens.previewRow(container);
      await Promise.resolve();
      expect(getMetricsHistoryMock).not.toHaveBeenCalled();
      dispose();
    });
  });
});

describe('Docker row hover lens', () => {
  it('previews a host row as CPU, memory, disk and temperature sparklines', () => {
    const [mode, setMode] = createSignal<WorkloadsMetricHoverMode>('history');
    const lens = fakeLens(mode);
    const { container: root } = render(() => (
      <DockerHostsTable
        resources={[host]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
        historyLens={lens}
      />
    ));
    const row = root.querySelector('[data-docker-host-row]') as HTMLElement;

    fireEvent.pointerEnter(row, { pointerType: 'mouse' });
    expect(row).toHaveAttribute('data-history-lens-active', 'true');
    expect(lens.previewRow).toHaveBeenCalledWith(host);
    for (const metric of ['CPU', 'memory', 'disk', 'temperature']) {
      expect(
        screen.getByRole('img', { name: new RegExp(`docker-01 ${metric} history`) }),
      ).toBeInTheDocument();
    }

    fireEvent.pointerLeave(row, { pointerType: 'mouse' });
    expect(row).not.toHaveAttribute('data-history-lens-active');
    expect(screen.queryByRole('img', { name: /history/ })).not.toBeInTheDocument();

    setMode('details');
    fireEvent.pointerEnter(row, { pointerType: 'mouse' });
    expect(row).not.toHaveAttribute('data-history-lens-active');
  });

  it('keeps a stale host on its last values instead of previewing history', () => {
    const lens = fakeLens(() => 'history');
    const { container: root } = render(() => (
      <DockerHostsTable
        resources={[
          {
            ...host,
            health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '1h' }] },
          },
        ]}
        emptyIcon={<span />}
        emptyTitle="No Docker hosts"
        emptyDescription="No hosts"
        showToolbar={false}
        historyLens={lens}
      />
    ));
    const row = root.querySelector('[data-docker-host-row]') as HTMLElement;
    fireEvent.pointerEnter(row, { pointerType: 'mouse' });
    expect(row).not.toHaveAttribute('data-history-lens-active');
  });

  it('previews a running container row as CPU and memory sparklines', () => {
    const lens = fakeLens(() => 'history');
    const { container: root } = render(() => (
      <Router>
        <Route
          path="*"
          component={() => (
            <DockerContainersTable
              resources={[container]}
              hosts={[host]}
              emptyIcon={<span />}
              emptyTitle="No containers"
              emptyDescription="No containers"
              historyLens={lens}
            />
          )}
        />
      </Router>
    ));
    const row = root.querySelector('[data-docker-container-row]') as HTMLElement;
    fireEvent.pointerEnter(row, { pointerType: 'mouse' });
    expect(row).toHaveAttribute('data-history-lens-active', 'true');
    expect(screen.getByRole('img', { name: /web CPU history/ })).toBeInTheDocument();
    expect(screen.getByRole('img', { name: /web memory history/ })).toBeInTheDocument();
  });
});
