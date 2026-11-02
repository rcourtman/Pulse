import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createStore, produce } from 'solid-js/store';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div>Details</div>,
}));

afterEach(cleanup);

const server = (status = 'available'): Resource => ({
  id: 'pbs-main',
  type: 'pbs',
  name: 'main',
  displayName: 'main',
  status: 'online',
  platformId: 'pbs-main',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  lastSeen: 1_790_000_000_000,
  pbs: {
    instanceId: 'pbs-main',
    connectionHealth: 'healthy',
    datastores: [{ name: 'tank', total: 1000, used: 820, available: 180, status }],
  },
});

const forecastAlert = {
  id: 'pbs-main-tank-usage',
  type: 'usage',
  level: 'warning',
  resourceId: 'pbs-main-tank',
  resourceName: 'tank',
  instance: 'pbs-main',
  message: 'Storage projected to fill in 4 days',
  startTime: '2026-11-02T08:00:00Z',
  acknowledged: false,
  metadata: {
    forecastDaysToFull: 4.2,
    forecastDailyChangePct: 4.1,
    storagePolicyAliases: ['pbs-main/tank'],
  },
} as unknown as Alert;

const lookup = (alerts: Alert[]) => (ids: string[]) =>
  alerts.filter((alert) => ids.includes(alert.resourceId));

describe('PBS datastore fill forecast', () => {
  it('puts the forecast in the Used cell and the drawer', async () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[server()]}
        layoutWidth={() => 1400}
        datastoreAlerts={lookup([forecastAlert])}
      />
    ));
    const pill = container.querySelector('[data-backup-server-forecast]')!;
    expect(pill).toHaveAttribute('data-backup-server-forecast', 'warning');
    expect(pill).toHaveTextContent('Full in ~4d');
    expect(pill).toHaveAttribute('title', expect.stringContaining('growing 4.1% a day'));
    // The forecast takes the bytes' place, so the row stays one line.
    expect(pill.closest('td')).not.toHaveTextContent('820 B /');

    await fireEvent.click(
      screen.getByRole('button', { name: 'Expand details for main · datastore tank' }),
    );
    const drawer = screen.getByRole('region', { name: 'Selected PBS datastore' });
    expect(drawer).toHaveTextContent('Fill forecast');
    expect(drawer).toHaveTextContent('Full in ~4 days, growing 4.1% a day');
  });

  it('follows the live alert store as a forecast appears, changes and clears', async () => {
    const [alerts, setAlerts] = createStore<Record<string, Alert>>({});
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[server()]}
        layoutWidth={() => 1400}
        datastoreAlerts={(ids) => Object.values(alerts).filter((a) => ids.includes(a.resourceId))}
      />
    ));
    const pill = () => container.querySelector('[data-backup-server-forecast]');
    expect(pill()).toBeNull();

    setAlerts(forecastAlert.id, structuredClone(forecastAlert));
    await waitFor(() => expect(pill()).toHaveTextContent('Full in ~4d'));

    setAlerts(
      produce((draft) => {
        draft[forecastAlert.id].level = 'critical';
        draft[forecastAlert.id].metadata!.forecastDaysToFull = 0.4;
      }),
    );
    await waitFor(() => expect(pill()).toHaveTextContent('Full <1d'));
    expect(pill()).toHaveAttribute('data-backup-server-forecast', 'critical');

    setAlerts(forecastAlert.id, undefined as unknown as Alert);
    await waitFor(() => expect(pill()).toBeNull());
    expect(container).toHaveTextContent('820 B /');
  });

  it('names the forecast in State on phones and keeps the percentage whole', () => {
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[server()]}
        layoutWidth={() => 375}
        datastoreAlerts={lookup([forecastAlert])}
      />
    ));
    const forecasts = container.querySelectorAll('[data-backup-server-forecast]');
    expect(forecasts).toHaveLength(1);
    expect(forecasts[0]).toHaveTextContent('Full in ~4d');
    expect(forecasts[0].closest('td')).not.toHaveTextContent('82');
    expect(container).toHaveTextContent('82.0%');
  });

  it('lets only a critical job problem keep State on phones over a warning forecast', () => {
    const critical = server();
    critical.pbs!.jobHealthEvidence = [
      {
        id: 'sync-tank',
        family: 'sync',
        store: 'tank',
        enabled: true,
        confidence: 'config-last-run',
        posture: 'critical',
        postureReason: 'last-run-failed',
      },
    ];
    const { container } = render(() => (
      <ProxmoxBackupServersTable
        servers={[critical]}
        layoutWidth={() => 375}
        datastoreAlerts={lookup([forecastAlert])}
      />
    ));
    expect(container.querySelector('[data-backup-server-forecast]')).toBeNull();
    expect(container.querySelector('[data-backup-server-jobs="critical"]')).not.toBeNull();
  });

  it('shows no forecast without one, or for a store it cannot read', () => {
    const { container, unmount } = render(() => (
      <ProxmoxBackupServersTable
        servers={[server()]}
        layoutWidth={() => 1400}
        datastoreAlerts={lookup([])}
      />
    ));
    expect(container.querySelector('[data-backup-server-forecast]')).toBeNull();
    expect(container).toHaveTextContent('820 B /');
    unmount();

    const unreadable = render(() => (
      <ProxmoxBackupServersTable
        servers={[server('unavailable')]}
        layoutWidth={() => 1400}
        datastoreAlerts={lookup([forecastAlert])}
      />
    ));
    expect(unreadable.container.querySelector('[data-backup-server-forecast]')).toBeNull();
    expect(unreadable.container).toHaveTextContent('Unavailable');
  });
});
