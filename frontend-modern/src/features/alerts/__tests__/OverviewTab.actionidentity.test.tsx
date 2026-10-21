import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import { OverviewTab } from '../OverviewTab';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));
vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getDeliveryDiagnoses: vi.fn().mockResolvedValue([]),
    snooze: vi.fn(),
    unsnooze: vi.fn(),
  },
}));
vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getHealth: vi.fn().mockResolvedValue({ queue: { status: 'healthy' } }),
  },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));
const alert = (id: string): Alert => ({
  id,
  type: 'connection',
  level: 'critical',
  resourceId: `host-${id}`,
  resourceName: 'Database',
  node: '',
  instance: 'synthetic',
  message: `Synthetic observation ${id}`,
  value: 0,
  threshold: 0,
  startTime: '2026-10-21T11:55:00Z',
  acknowledged: false,
});
const settle = async () => {
  for (let i = 0; i < 16; i++) await Promise.resolve();
};
function mount(related = false) {
  const target = alert('target'),
    neighbour = alert('neighbour');
  if (related) {
    target.correlation = {
      key: 'same-group',
      kind: 'shared-system',
      role: 'supporting',
      reason: 'Synthetic membership',
    };
    neighbour.correlation = { ...target.correlation, role: 'primary' };
  }
  const [rows, setRows] = createSignal<Record<string, Alert>>({ target, neighbour });
  render(() => (
    <OverviewTab
      activeAlerts={rows()}
      updateAlert={(id, changes) =>
        setRows((current) => ({ ...current, [id]: { ...current[id], ...changes } }))
      }
      overrides={[]}
      showAcknowledged={() => true}
      setShowAcknowledged={vi.fn()}
      showQuickTip={() => false}
      dismissQuickTip={vi.fn()}
      alertsDisabled={() => false}
    />
  ));
  if (related) fireEvent.click(screen.getByRole('button', { name: /^\+1 linked signal/ }));
  return { rows, setRows, card: document.getElementById('alert-target')! };
}
describe('overview alert action identity through resampling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-21T12:00:00Z'));
    vi.clearAllMocks();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });
  it.each([false, true])(
    'keeps the %s-related card, open dialog and rejected snooze retry through resampling',
    async (related) => {
      const { card, rows, setRows } = mount(related);
      fireEvent.click(within(card).getByRole('button', { name: 'Snooze' }));
      const dialog = screen.getByRole('dialog', { name: 'Snooze this alert' });
      let fail!: (error: Error) => void;
      vi.mocked(AlertsAPI.snooze).mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            fail = reject;
          }),
      );
      const preset = within(dialog).getByRole('button', { name: 'For 1 hour' });
      fireEvent.click(preset);
      expect(preset).toBeDisabled();
      expect(screen.getByRole('dialog')).toBe(dialog);
      // A normal full observation replaces objects without changing identity.
      setRows((current) =>
        Object.fromEntries(
          Object.entries(current).map(([key, row]) => [
            key,
            { ...row, message: `Refreshed ${key}` },
          ]),
        ),
      );
      expect(document.getElementById('alert-target')).toBe(card);
      expect(screen.getByRole('dialog')).toBe(dialog);
      expect(within(card).getByText('Refreshed target')).toBeInTheDocument();
      fireEvent.click(preset);
      expect(AlertsAPI.snooze).toHaveBeenCalledTimes(1);
      fail(new Error('Synthetic unconfirmed action'));
      await settle();
      expect(rows().target.operationalRecord).toBeUndefined();
      expect(rows().neighbour.operationalRecord).toBeUndefined();
      expect(screen.getByRole('dialog')).toBe(dialog);
      expect(preset).toBeEnabled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).toHaveBeenCalledWith(
        'Snooze could not be confirmed. Refresh alerts before trying again.',
      );
      vi.mocked(AlertsAPI.snooze).mockResolvedValueOnce({
        success: true,
        snoozedUntil: '2026-10-21T13:00:00Z',
      });
      fireEvent.click(preset);
      await settle();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(document.getElementById('alert-target')).toBe(card);
      expect(within(card).getByRole('button', { name: 'Resume' })).toBeInTheDocument();
      expect(AlertsAPI.snooze).toHaveBeenCalledTimes(2);
    },
  );
  it('retires an open primary dialog when a different alert becomes the primary', () => {
    const { rows, setRows } = mount(true);
    const oldPrimary = document.getElementById('alert-neighbour')!;
    fireEvent.click(within(oldPrimary).getByRole('button', { name: 'Snooze' }));
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    setRows({
      target: { ...rows().target, correlation: { ...rows().target.correlation!, role: 'primary' } },
      neighbour: {
        ...rows().neighbour,
        correlation: { ...rows().neighbour.correlation!, role: 'supporting' },
      },
    });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(oldPrimary).not.toBeInTheDocument();
    expect(AlertsAPI.snooze).not.toHaveBeenCalled();
    expect(
      within(document.getElementById('alert-target')!).getByRole('button', {
        name: 'Snooze',
      }),
    ).toBeInTheDocument();
  });
  it.each([false, true])(
    'removes a vanished %s-related alert without retaining a ghost dialog',
    (related) => {
      const { card, rows, setRows } = mount(related);
      fireEvent.click(within(card).getByRole('button', { name: 'Snooze' }));
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      setRows({ neighbour: rows().neighbour });
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(card).not.toBeInTheDocument();
      expect(document.getElementById('alert-neighbour')).toBeInTheDocument();
      expect(AlertsAPI.snooze).not.toHaveBeenCalled();
    },
  );
});
