import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI } from '@/api/charts';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { Resource } from '@/types/resource';
import { DockerContainersTable } from '../DockerContainersTable';
import { CONTAINER_CPU_CAPACITY_DESCRIPTION } from '../dockerCpuPresentation';

vi.mock('@/stores/license', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/stores/license')>()),
  loadRuntimeCapabilities: vi.fn(async () => undefined),
  maxHistoryDays: () => 7,
  isRangeLocked: () => false,
}));

const containerResource = (id: string, cpu: number | undefined, running = true): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'docker',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  type: 'app-container',
  status: running ? 'running' : 'stopped',
  lastSeen: 1_700_000_000_000,
  cpu: cpu === undefined ? undefined : { current: cpu },
  memory: { current: 23.9 },
  metricsTarget: { resourceType: 'app-container', resourceId: id },
  docker: {
    hostname: 'lab-host',
    containerId: id,
    containerState: running ? 'running' : 'exited',
    runtime: 'podman',
  },
});

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  );
});

afterEach(() => {
  cleanup();
  window.history.pushState({}, '', '/');
  window.localStorage.clear();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('Docker container CPU presentation through the real row and drawer', () => {
  it('separates zero, fractional, unavailable and stopped readings without changing sort values', async () => {
    const { container } = render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <DockerContainersTable
              resources={[
                containerResource('zero', 0),
                containerResource('tiny', 0.026),
                containerResource('small', 0.17),
                containerResource('rounded', 0.49),
                containerResource('unavailable', undefined),
                containerResource('stopped', 3, false),
              ]}
              emptyIcon={<span />}
              emptyTitle="No containers"
              emptyDescription="No containers"
              showToolbar={false}
            />
          )}
        />
      </Router>
    ));
    const heads = screen.getAllByRole('columnheader');
    const cpuIndex = heads.findIndex((head) => head.textContent?.trim() === 'CPU');
    expect(cpuIndex).toBeGreaterThan(0);
    const cell = (id: string) =>
      container.querySelector(`[data-docker-container-row="${id}"]`)?.children[cpuIndex];
    for (const [id, label] of [
      ['zero', '0%'],
      ['tiny', '<0.1%'],
      ['small', '0.2%'],
      ['rounded', '0.5%'],
      ['unavailable', '—'],
      ['stopped', '—'],
    ]) {
      expect(cell(id)?.textContent?.trim()).toBe(label);
    }
    expect(screen.getByTestId('container-table-cpu-scale')).toHaveTextContent(
      CONTAINER_CPU_CAPACITY_DESCRIPTION,
    );
    fireEvent.click(heads[cpuIndex]);
    const order = [...container.querySelectorAll('[data-docker-container-row]')].map((row) =>
      row.getAttribute('data-docker-container-row'),
    );
    // The stopped container still has its raw reading. Presentation never
    // edits that datum, so the existing numeric sort continues to see it.
    expect(order.slice(0, 5)).toEqual(['stopped', 'rounded', 'small', 'tiny', 'zero']);
    expect(order[5]).toBe('unavailable');

    const time = 1_700_000_000_000;
    const fetch = vi.spyOn(ChartsAPI, 'getMetricsHistory').mockResolvedValue({
      resourceType: 'app-container',
      resourceId: 'tiny',
      range: '24h',
      start: time,
      end: time + 120_000,
      source: 'store',
      metrics: {
        cpu: [0, 0.026, 9.4].map((value, index) => ({
          timestamp: time + index * 60_000,
          value,
          min: value,
          max: value,
        })),
      },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Expand details for tiny' }));
    expect(screen.getByTestId('container-drawer-cpu-scale')).toHaveTextContent(
      CONTAINER_CPU_CAPACITY_DESCRIPTION,
    );
    fireEvent.click(screen.getByRole('tab', { name: 'History' }));
    const slider = await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    await waitFor(() => expect(slider).toHaveAttribute('max', '2'));
    fireEvent.input(slider, { target: { value: '1' } });
    expect(slider).toHaveAttribute('aria-valuetext', expect.stringContaining('CPU <0.1%'));
    fireEvent.input(slider, { target: { value: '0' } });
    expect(slider).toHaveAttribute('aria-valuetext', expect.stringContaining('CPU 0.0%'));
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch).toHaveBeenCalledWith(
      expect.objectContaining({ resourceType: 'app-container', resourceId: 'tiny' }),
    );
  });
});
