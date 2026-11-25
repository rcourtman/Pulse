import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import type { StatusIndicatorVariant } from '@/utils/status';
import { canonicalizeRealtimeResource } from '@/utils/resourceStateAdapters';
import { buildBackupServerRows, ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Server details</div>,
}));

afterEach(cleanup);

const server = (health?: string, status: Resource['status'] = 'online'): Resource => ({
  id: 'pbs-connection',
  type: 'pbs',
  name: 'pbs-connection',
  displayName: 'pbs-connection',
  status,
  platformId: 'pbs-connection',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  lastSeen: 1_794_614_400_000,
  pbs: {
    instanceId: 'pbs-connection',
    connectionHealth: health,
    datastores: [
      { name: 'production', total: 1000, used: 940, available: 60, status: 'available' },
    ],
  },
});

const transport = (resource: Resource, canonical: boolean) => {
  if (!canonical) return resource;
  const { pbs, ...base } = resource;
  return canonicalizeRealtimeResource({ ...base, platformData: { sources: ['pbs'], pbs } });
};

const usedCell = (container: HTMLElement) => {
  const table = container.querySelector('table')!;
  const index = Array.from(table.querySelectorAll('thead th')).findIndex(
    (head) => head.textContent === 'Used',
  );
  return table.querySelector('tbody tr:not([data-backup-server-group])')!.querySelectorAll('td')[
    index
  ] as HTMLElement;
};

const connection = (container: HTMLElement) =>
  within(container.querySelector('[data-backup-server-group]') as HTMLElement).getByRole('img');

const toneClass: Record<StatusIndicatorVariant, string> = {
  success: 'bg-emerald-500',
  danger: 'bg-red-500',
  warning: 'bg-amber-500',
  muted: 'bg-slate-400',
  info: 'bg-blue-500',
};

const withReadings = (resource: Resource, pbs: 'online' | 'stale', agent = 'online') =>
  transport(
    {
      ...resource,
      sourceStatus: {
        pbs: {
          status: pbs,
          lastSeen: pbs === 'stale' ? '2026-11-24T10:00:00Z' : '2026-11-24T12:00:00Z',
        },
        agent: { status: agent, lastSeen: '2026-11-24T12:00:00Z' },
      },
    },
    true,
  );

const forecastAlert: Alert = {
  id: 'fixture-forecast',
  type: 'usage',
  level: 'warning',
  resourceId: 'pbs-connection-production',
  resourceName: 'production',
  node: '',
  instance: 'pbs-connection',
  value: 40,
  threshold: 90,
  message: 'Storage projected to fill in 4 days',
  startTime: '2026-11-24T10:00:00Z',
  acknowledged: false,
  metadata: { forecastDaysToFull: 4.2, storagePolicyAliases: ['pbs-connection/production'] },
};

describe.each([false, true])('PBS connection presentation (canonical=%s)', (canonical) => {
  it.each(['healthy', 'ok', 'online', 'running', 'connected', ' ONLINE '])(
    'admits the successful %s state without calling its datastore unreachable',
    (health) => {
      const [row] = buildBackupServerRows([transport(server(health), canonical)]);
      expect(row.online).toBe(true);
      expect(row.connectionVariant).toBe('success');
      expect(row.connectionLabel).toBe(
        health.trim().charAt(0).toUpperCase() + health.trim().slice(1),
      );
    },
  );

  it.each([
    ['error', 'danger'],
    ['unhealthy', 'danger'],
    ['offline', 'danger'],
    ['down', 'danger'],
    ['stopped', 'danger'],
    ['disconnected', 'danger'],
    ['failed', 'danger'],
    ['unreachable', 'danger'],
    ['timeout', 'danger'],
    ['warning', 'warning'],
    ['degraded', 'warning'],
    ['unknown', 'muted'],
    ['future-provider-state', 'muted'],
  ] as const)('keeps explicit %s distinct from a retained online status', (health, variant) => {
    const [row] = buildBackupServerRows([transport(server(health), canonical)]);
    expect(row.online).toBe(false);
    expect(row.connectionVariant).toBe(variant);
  });

  it.each([
    ['online', 'success'],
    ['running', 'success'],
    ['offline', 'danger'],
    ['stopped', 'danger'],
    ['degraded', 'warning'],
    ['warning', 'warning'],
    ['unknown', 'muted'],
  ] as const)('uses %s only when specific connection evidence is absent', (status, variant) => {
    const [row] = buildBackupServerRows([transport(server(undefined, status), canonical)]);
    expect(row.connectionVariant).toBe(variant);
    expect(row.online).toBe(variant === 'success');
    expect(row.connectionLabel).toBe(status.charAt(0).toUpperCase() + status.slice(1));
  });
});

describe.each([390, 1200])('PBS connection rendering at %ipx', (width) => {
  it('qualifies retained PBS readings and forecasts without closing the drawer', async () => {
    const resource = server('healthy', 'warning');
    resource.pbs!.datastores![0].used = 400;
    const [snapshot, setSnapshot] = createSignal(withReadings(resource, 'online', 'stale'));
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[snapshot()]}
        layoutWidth={() => width}
        datastoreAlerts={() => [forecastAlert]}
      />
    ));
    const label = () => connection(container);
    expect(label()).toHaveAccessibleName('Healthy');
    const toggle = screen.getByRole('button', { name: /Expand details.*production/ });
    fireEvent.click(toggle);
    toggle.focus();
    const selected = screen.getByRole('region', { name: 'Selected PBS datastore' });
    const cell = usedCell(container);
    const forecast = () => container.querySelector('[data-backup-server-forecast]')!;

    setSnapshot(withReadings(resource, 'stale'));
    await waitFor(() => expect(screen.getByText('PBS data stale')).toBeVisible());
    expect(label()).toHaveAccessibleName('PBS data stale · Last status: Healthy');
    expect(label()).toHaveClass(toneClass.warning);
    expect(cell).toHaveTextContent('40.0%');
    expect(cell).toHaveTextContent('Last reading');
    expect(cell.querySelector('.bg-emerald-500')).toBeNull();
    expect(forecast()).toHaveTextContent('Full in ~4d');
    expect(forecast().closest('td')).toHaveTextContent(
      width < 520 ? 'Last forecast' : 'Last reading & forecast',
    );
    const staleSummary = 'Stale. Capacity, forecasts and jobs are last reported values.';
    expect(selected).toHaveTextContent(staleSummary);
    expect(selected).toHaveTextContent('40.0% used');
    expect(selected).toHaveTextContent('Full in ~4 days');

    setSnapshot(withReadings(server('healthy'), 'online'));
    await waitFor(() => expect(screen.queryByText('PBS data stale')).toBeNull());
    expect(label()).toHaveAccessibleName('Healthy');
    expect(usedCell(container)).toBe(cell);
    expect(cell).toHaveTextContent('94.0%');
    expect(cell).not.toHaveTextContent('Last reading');
    expect(screen.getByRole('region', { name: 'Selected PBS datastore' })).toBe(selected);
    expect(selected).not.toHaveTextContent(staleSummary);
    expect(toggle).toHaveFocus();
  });

  it('keeps a recorded failure separate from silence without promoting unread capacity', () => {
    const resource = server('error', 'offline');
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[withReadings(resource, 'stale')]}
        layoutWidth={() => width}
        datastoreAlerts={() => [forecastAlert]}
      />
    ));
    expect(screen.getByText('PBS data stale')).toBeVisible();
    expect(screen.getByText('Error', { exact: true })).toBeVisible();
    expect(connection(container)).toHaveClass(toneClass.danger);
    expect(usedCell(container).textContent).toBe('Unavailable');
    expect(container.querySelector('[data-backup-server-forecast]')).toBeNull();
  });

  it('keeps a known failed verify job and a separate datastore read failure visible', () => {
    const resource = server('healthy', 'warning');
    resource.pbs!.datastores![0].status = 'unavailable';
    resource.pbs!.jobHealthEvidence = [
      {
        id: 'verify-production',
        family: 'verify',
        store: 'production',
        enabled: true,
        confidence: 'config-last-run',
        posture: 'critical',
      },
    ];
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[withReadings(resource, 'stale')]}
        layoutWidth={() => width}
      />
    ));
    expect(screen.getByText('PBS data stale')).toBeVisible();
    expect(screen.getByText('Verify failed', { exact: true })).toBeVisible();
    expect(connection(container)).toHaveClass(toneClass.danger);
    expect(usedCell(container).textContent).toBe('Unavailable');
  });

  it.each([
    ['online', 'success', '94.0%'],
    ['connected', 'success', '94.0%'],
    ['unknown', 'muted', 'Unknown'],
    ['future-provider-state', 'muted', 'Unknown'],
    ['degraded', 'warning', 'Unknown'],
    ['error', 'danger', 'Unavailable'],
  ] as const)(
    'shows %s consistently in the header, capacity and selected datastore',
    (health, variant, capacity) => {
      const { container } = render(() => (
        <ProxmoxBackupServersTable
          servers={[transport(server(health), true)]}
          layoutWidth={() => width}
        />
      ));
      expect(connection(container)).toHaveClass(toneClass[variant]);
      expect(usedCell(container)).toHaveTextContent(capacity);
      if (variant !== 'success') {
        expect(usedCell(container)).not.toHaveTextContent('%');
        expect(container.textContent).not.toContain('940 B');
      }
      if (variant !== 'danger') expect(container.textContent).not.toContain('Unreachable');
      fireEvent.click(screen.getByRole('button', { name: /Expand details.*production/ }));
      const selected = screen.getByRole('region', { name: 'Selected PBS datastore' });
      expect(within(selected).getByRole('img')).toHaveClass(toneClass[variant]);
      expect(selected).toHaveTextContent('on pbs-connection');
      expect(selected).toHaveTextContent(capacity);
      if (variant !== 'success') {
        expect(selected).toHaveTextContent('This does not mean the datastore is empty.');
      }
    },
  );

  it('updates the same open datastore through unknown, recovery, warning and failure', async () => {
    const [servers, setServers] = createSignal([transport(server('unknown'), true)]);
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={servers()} layoutWidth={() => width} />
    ));
    const toggle = screen.getByRole('button', { name: /Expand details.*production/ });
    fireEvent.click(toggle);
    toggle.focus();
    const selected = screen.getByRole('region', { name: 'Selected PBS datastore' });
    const cell = usedCell(container);
    for (const [health, variant, capacity] of [
      ['unknown', 'muted', 'Unknown'],
      ['connected', 'success', '94.0%'],
      ['degraded', 'warning', 'Unknown'],
      ['error', 'danger', 'Unavailable'],
      ['healthy', 'success', '94.0%'],
    ] as const) {
      setServers([transport(server(health), true)]);
      await waitFor(() => expect(connection(container)).toHaveClass(toneClass[variant]));
      expect(usedCell(container)).toBe(cell);
      expect(cell).toHaveTextContent(capacity);
      expect(screen.getByRole('region', { name: 'Selected PBS datastore' })).toBe(selected);
      expect(within(selected).getByRole('img')).toHaveClass(toneClass[variant]);
      expect(selected).toHaveTextContent(capacity);
      expect(toggle).toHaveFocus();
    }
  });

  it('does not turn explicit datastore failure into capacity when a connection alias succeeds', () => {
    const resource = server('connected');
    resource.pbs!.datastores![0].status = 'unavailable';
    const { container } = render(() => (
      <ProxmoxBackupServersTable servers={[resource]} layoutWidth={() => width} />
    ));
    expect(connection(container)).toHaveClass(toneClass.success);
    expect(usedCell(container)).toHaveTextContent('Unavailable');
    expect(usedCell(container)).not.toHaveTextContent('94.0%');
  });
});
