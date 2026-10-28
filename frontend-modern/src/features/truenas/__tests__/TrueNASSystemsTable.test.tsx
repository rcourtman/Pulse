import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: (props: { detailsLabel?: string }) => (
    <div data-testid="stacked-memory-bar" data-details-label={props.detailsLabel} />
  ),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

const activeAlertsRef = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: activeAlertsRef.current }),
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 80, critical: 85 }),
  }),
}));

import { TrueNASSystemsTable } from '@/features/truenas/TrueNASSystemsTable';
import type { Resource } from '@/types/resource';
import { temperatureStore } from '@/utils/temperature';

const makeSystem = (overrides: Partial<Resource> & Pick<Resource, 'id'>): Resource =>
  ({
    type: 'agent',
    name: overrides.id,
    displayName: overrides.id,
    status: 'online',
    platformId: 'truenas-1',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    sourceType: 'agent',
    sources: ['agent', 'truenas'],
    lastSeen: 1_700_000_000_000,
    cpu: { current: 12 },
    memory: { current: 40 },
    disk: { current: 55 },
    agent: { osVersion: 'TrueNAS-SCALE-24.10.2' },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
  temperatureStore.setUnit('celsius');
  activeAlertsRef.current = {};
});

describe('TrueNASSystemsTable', () => {
  it('names each inspectable memory record after its own system rather than its connection', () => {
    const systems = [
      makeSystem({ id: 'nas-1', name: 'core-east' }),
      makeSystem({ id: 'nas-2', name: 'core-west' }),
    ];
    const { container } = render(() => (
      <TrueNASSystemsTable
        systems={systems}
        scope={systems}
        emptyIcon={<span />}
        emptyTitle="No systems"
        emptyDescription="No systems"
        showToolbar={false}
      />
    ));
    for (const [id, name] of [
      ['nas-1', 'core-east'],
      ['nas-2', 'core-west'],
    ]) {
      expect(
        container.querySelector(
          `[data-truenas-system-row="${id}"] [data-testid="stacked-memory-bar"]`,
        ),
      ).toHaveAttribute('data-details-label', `Memory composition for ${name}`);
    }
  });

  it('updates both the reading and compact temperature heading with the display preference', async () => {
    temperatureStore.setUnit('celsius');
    const system = makeSystem({ id: 'nas-temperature', temperature: 40 });
    const { container } = render(() => (
      <TrueNASSystemsTable
        systems={[system]}
        scope={[system]}
        emptyIcon={<span />}
        emptyTitle="No systems"
        emptyDescription="No systems"
        showToolbar={false}
      />
    ));
    const temperatureHeader = () =>
      [...container.querySelectorAll('thead th')].find((header) =>
        header.textContent?.includes('Temp'),
      );

    expect(screen.getByText('40.0°C')).toBeInTheDocument();
    expect(temperatureHeader()).toHaveTextContent('°C');
    temperatureStore.setUnit('fahrenheit');
    expect(await screen.findByText('104.0°F')).toBeInTheDocument();
    expect(temperatureHeader()).toHaveTextContent('°F');
    expect(temperatureHeader()).not.toHaveTextContent('°C');
    expect(system.temperature).toBe(40);
    temperatureStore.setUnit('celsius');
    expect(await screen.findByText('40.0°C')).toBeInTheDocument();
    expect(temperatureHeader()).toHaveTextContent('°C');
  });

  it('keeps capacity and temperature in the phone column set', () => {
    const { container } = render(() => (
      <TrueNASSystemsTable
        systems={[makeSystem({ id: 'nas-1' })]}
        scope={[makeSystem({ id: 'nas-1' })]}
        emptyIcon={<span />}
        emptyTitle="No systems"
        emptyDescription="No systems"
        showToolbar={false}
      />
    ));

    const headers = [...container.querySelectorAll('thead th')];
    expect(headers.find((header) => header.textContent?.includes('System'))).toHaveClass(
      'platform-table-mobile-w-30',
    );
    expect(headers.find((header) => header.textContent?.includes('Capacity'))).toHaveClass(
      'platform-table-mobile-w-20',
    );
    // A three-digit Fahrenheit reading and its unit must fit without cutting
    // the value; the name keeps its existing share of the phone table.
    expect(headers.find((header) => header.textContent?.includes('°C'))).toHaveClass(
      'platform-table-mobile-w-20',
    );
  });

  it('treats an impaired TrueNAS source as degraded for the row indicator and health filter', async () => {
    const healthy = makeSystem({ id: 'truenas-healthy', name: 'truenas-healthy' });
    const impaired = makeSystem({
      id: 'truenas-impaired',
      name: 'truenas-impaired',
      platformData: {
        sourceStatus: {
          truenas: { status: 'error' },
        },
      },
    });

    const { container } = render(() => (
      <TrueNASSystemsTable
        systems={[healthy, impaired]}
        scope={[healthy, impaired]}
        emptyIcon={<span />}
        emptyTitle="No systems"
        emptyDescription="No systems"
      />
    ));

    const impairedRow = container.querySelector('[data-truenas-system-row="truenas-impaired"]');
    expect(impairedRow).not.toBeNull();
    expect(impairedRow?.querySelector('[title="degraded"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-system-row="truenas-healthy"]')).not.toBeNull();

    await fireEvent.click(
      within(screen.getByRole('group', { name: 'Status' })).getByRole('button', {
        name: /^Degraded, \d+$/,
      }),
    );

    expect(container.querySelector('[data-truenas-system-row="truenas-impaired"]')).not.toBeNull();
    expect(container.querySelector('[data-truenas-system-row="truenas-healthy"]')).toBeNull();
    expect(screen.getByText('1 of 2 systems')).toBeInTheDocument();
  });
  it("tints a system row for its pools' alerts, which carry the system hostname", () => {
    // TrueNAS pool, dataset, disk and app alerts are keyed on the child
    // resource and name the system in "node". The system's drawer states the
    // same pool problem through its health issue, so the row matches by
    // hostname as well as id; no producer keys a TrueNAS system's alerts on
    // its "agent:" alias.
    activeAlertsRef.current = {
      pool: {
        id: 'pool',
        type: 'truenas-pool-health',
        level: 'warning',
        resourceId: 'storage-archive',
        resourceName: 'archive',
        node: 'truenas-main',
        instance: 'TrueNAS',
        message: 'Pool archive is DEGRADED',
        value: 0,
        threshold: 0,
        startTime: '2026-10-06T10:00:00Z',
        acknowledged: false,
      },
    };

    const { container } = render(() => (
      <TrueNASSystemsTable
        systems={[makeSystem({ id: 'truenas-main' }), makeSystem({ id: 'truenas-backup' })]}
        scope={[makeSystem({ id: 'truenas-main' }), makeSystem({ id: 'truenas-backup' })]}
        emptyIcon={<span />}
        emptyTitle="No systems"
        emptyDescription="No systems"
        showToolbar={false}
      />
    ));

    const row = (id: string) => container.querySelector(`[data-truenas-system-row="${id}"]`);
    expect(row('truenas-main')).toHaveClass('bg-yellow-50');
    expect(row('truenas-backup')).not.toHaveClass('bg-yellow-50');
  });
});
