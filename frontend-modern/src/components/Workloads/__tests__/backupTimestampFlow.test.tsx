import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Show, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useWorkloads, __resetWorkloadsCacheForTests } from '@/hooks/useWorkloads';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { Resource } from '@/types/resource';
import { GuestDrawer } from '../GuestDrawer';
import { BackupIndicator, BackupStatusCell } from '../GuestRowCells';

const api = vi.hoisted(() => ({ json: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: api.json,
  getOrgID: () => 'default',
}));
vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    getBackupThresholds: () => ({ freshHours: 24, staleHours: 72 }),
    getMetricThresholds: () => null,
  }),
}));
vi.mock('@/api/charts', async () => {
  const actual = await vi.importActual<typeof import('@/api/charts')>('@/api/charts');
  return {
    ...actual,
    ChartsAPI: {
      ...actual.ChartsAPI,
      getMetricsHistory: vi.fn().mockResolvedValue({ metrics: {} }),
    },
  };
});
vi.mock('@/api/resourceOperatorState', () => ({
  getResourceOperatorState: vi.fn().mockResolvedValue(null),
}));
vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 90,
}));

const now = Date.parse('2026-10-04T12:00:00Z');
const valid = '2026-10-04T13:00:00.123456789+02:00';
const iso = '2026-10-04T11:00:00.123Z';
const malformed = ['2026-09-31T11:00:00Z', '2026-10-04T11:00:00', '2026-10-04'];

const rawResource = (lastBackup: string, type: 'vm' | 'system-container', running: boolean) => ({
  id: 'east-100',
  name: 'backup-time-guest',
  type,
  status: 'running',
  lastSeen: new Date(now).toISOString(),
  sources: ['proxmox'],
  proxmox: {
    vmid: 100,
    instance: 'east',
    node: 'pve-1',
    nodeName: 'pve-1',
    lastBackup,
    backupInProgress: running,
  },
});

function mount(transport: 'api' | 'snapshot', type: 'vm' | 'system-container') {
  let raw = rawResource(malformed[0], type, false);
  api.json.mockImplementation(() => Promise.resolve({ data: [raw], meta: { totalPages: 1 } }));
  const [snapshot, setSnapshot] = createSignal<Resource[]>([raw as unknown as Resource]);
  let source!: ReturnType<typeof useWorkloads>;
  render(() => {
    source = useWorkloads(
      () => true,
      transport === 'snapshot' ? { resourceSnapshot: snapshot } : {},
    );
    const guest = () => source.workloads()[0];
    return (
      <Show when={guest()}>
        <BackupStatusCell
          lastBackup={guest().lastBackup}
          backupRunning={guest().backupInProgress}
        />
        <BackupIndicator
          lastBackup={guest().lastBackup}
          backupRunning={guest().backupInProgress}
          isTemplate={false}
        />
        <GuestDrawer guest={guest()} onClose={() => {}} />
      </Show>
    );
  });
  return {
    source: () => source,
    update: async (value: string, running = false) => {
      raw = rawResource(value, type, running);
      if (transport === 'snapshot') setSnapshot([raw as unknown as Resource]);
      else await source.refetch();
    },
  };
}

const backupRow = () =>
  within(screen.getByTestId('guest-drawer-overview'))
    .getByText('Last completed backup')
    .closest('tr')!;

beforeEach(() => {
  vi.useFakeTimers({ now, toFake: ['Date'] });
  __resetWorkloadsCacheForTests();
  resetCreateNonSuspendingQueryCacheForTest();
  api.json.mockReset();
});
afterEach(() => {
  cleanup();
  __resetWorkloadsCacheForTests();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
});

describe.each(['api', 'snapshot'] as const)(
  'backup time through %s resource producer',
  (transport) => {
    it.each(['vm', 'system-container'] as const)(
      '[regression] keeps malformed %s evidence unknown through live recovery and recurrence',
      async (type) => {
        const view = mount(transport, type);
        const badge = await screen.findByRole('button', { name: /^Backup status:/ });
        const details = screen.getByTestId('guest-technical-details') as HTMLDetailsElement;
        details.open = true;
        badge.focus();
        fireEvent.click(badge);
        const dialog = screen.getByRole('dialog', { name: /^Backup status details/ });
        const close = within(dialog).getByRole('button', { name: 'Close backup details' });
        close.focus();
        for (const value of malformed) {
          await view.update(value);
          await waitFor(() => expect(badge).toHaveTextContent('Unknown'));
          expect(view.source().workloads()[0].lastBackup).toBeNaN();
          expect(badge.className).not.toContain('green');
          expect(
            screen.getByRole('button', { name: /^Backup time unavailable:/ }),
          ).toBeInTheDocument();
          expect(dialog.querySelector('time')).toBeNull();
          expect(backupRow()).toHaveTextContent('invalid timestamp');
          expect(backupRow().querySelector('time')).toBeNull();
          expect(screen.getByTestId('guest-technical-details')).toBe(details);
          expect(close).toHaveFocus();
        }
        await view.update(valid);
        await waitFor(() => expect(badge).toHaveAccessibleName(/fresh/));
        expect(view.source().workloads()[0].lastBackup).toBe(Date.parse(iso));
        expect(dialog.querySelector('time')).toHaveAttribute('datetime', iso);
        expect(backupRow().querySelector('time')).toHaveAttribute('datetime', iso);
        expect(dialog.querySelector('time')).toHaveTextContent('2026-10-04 11:00:00 UTC');
        expect(badge.className).toContain('green');
        await view.update(malformed[0], true);
        await waitFor(() => expect(badge).toHaveTextContent('Running'));
        expect(badge).toHaveAccessibleName(/invalid timestamp/);
        expect(dialog.querySelector('time')).toBeNull();
        expect(backupRow().querySelector('time')).toBeNull();
        expect(screen.getByTestId('guest-drawer-overview')).toHaveTextContent(
          'Running · not completed yet',
        );
        expect(screen.getByRole('dialog', { name: /^Backup status details/ })).toBe(dialog);
        expect(close).toHaveFocus();
        fireEvent.click(close);
        expect(badge).toHaveFocus();
        expect(view.source().workloads()[0]).toMatchObject({
          id: 'east-100',
          name: 'backup-time-guest',
          instance: 'east',
          vmid: 100,
          node: 'pve-1',
        });
        if (transport === 'snapshot') expect(api.json).not.toHaveBeenCalled();
        else expect(api.json).toHaveBeenCalledTimes(6);
      },
    );

    it('[control] preserves explicit absence, valid offsets and future uncertainty', async () => {
      const view = mount(transport, 'vm');
      const badge = await screen.findByRole('button', { name: /^Backup status:/ });
      for (const value of ['', '0001-01-01T00:00:00Z', '0001-01-01T00:00:00.000000000Z']) {
        await view.update(value);
        await waitFor(() => expect(badge).toHaveTextContent('None'));
        expect(view.source().workloads()[0].lastBackup).toBe(0);
      }
      await view.update('2026-10-04T04:00:00-07:00');
      await waitFor(() => expect(badge).toHaveAccessibleName(/fresh/));
      await view.update('2026-10-04T12:00:01Z');
      await waitFor(() => expect(badge).toHaveAccessibleName(/timestamp is in the future/));
      expect(view.source().workloads()[0].lastBackup).toBe(now + 1000);
    });
  },
);
