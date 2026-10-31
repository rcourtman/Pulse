import { renderHook } from '@solidjs/testing-library';
import { createMemo } from 'solid-js';
import { createStore, produce, reconcile } from 'solid-js/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import {
  createAlertAcknowledgementRequests,
  type AlertAcknowledgementRequests,
} from '@/stores/alertAcknowledgementRequests';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import { logger } from '@/utils/logger';
import {
  getAlertOverviewBulkAcknowledgedNotification,
  getAlertOverviewBulkAcknowledgeFailureNotification,
  getAlertOverviewBulkAcknowledgeGenericFailureNotification,
} from '@/utils/alertOverviewPresentation';

import { useAlertAcknowledgementState } from '../useAlertAcknowledgementState';

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    acknowledge: vi.fn(),
    bulkAcknowledge: vi.fn(),
    unacknowledge: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

function makeAlert(id: string, acknowledged = false): Alert {
  return {
    id,
    type: 'cpu',
    level: 'warning',
    resourceId: `vm-${id}`,
    resourceName: `VM ${id}`,
    node: 'node-1',
    message: `CPU high on ${id}`,
    startTime: '2026-03-22T11:00:00Z',
    acknowledged,
  } as Alert;
}

function acknowledgedBy(id: string, ackUser: string): Alert {
  return { ...makeAlert(id, true), ackTime: '2026-03-22T11:30:00Z', ackUser };
}

type BulkAcknowledgeResult = Awaited<ReturnType<typeof AlertsAPI.bulkAcknowledge>>;

// A request the test answers later, so it can act while the request is out.
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((settle, fail) => {
    resolve = settle;
    reject = fail;
  });
  return { promise, resolve, reject };
}

// The keyed alert store the way the websocket store keeps it: updateAlert
// merges a local change into the stored alert, and every server payload
// replaces the stored alert in place through reconcile. This mocks the
// store's contract; the request owner each test passes is the real module the
// store creates. The store lives outside any view, as the app's does.
function createAlertStore(initial: Alert[]) {
  const [alertsById, setAlertsById] = createStore<Record<string, Alert>>(
    Object.fromEntries(initial.map((alert) => [alert.id, alert])),
  );
  const updateAlert = vi.fn((alertIdentifier: string, updates: Partial<Alert>) => {
    const existing = alertsById[alertIdentifier];
    if (existing) {
      setAlertsById(alertIdentifier, { ...existing, ...updates });
    }
  });
  const serverSends = (alert: Alert) => setAlertsById(alert.id, reconcile(alert));
  // A resolved alert leaves the store, as the websocket store removes it.
  const serverResolves = (alertIdentifier: string) =>
    setAlertsById(
      produce((draft) => {
        delete draft[alertIdentifier];
      }),
    );
  return { alertsById, updateAlert, serverSends, serverResolves };
}

type AlertStoreHarness = ReturnType<typeof createAlertStore>;

// One overview view over the store and the app's request owner. A second view
// over the same pair after the first is cleaned up is a remount. The page
// reads the alerts through a memo over the keyed store, as
// useAlertOverviewState does.
function renderOverview(
  store: AlertStoreHarness,
  acknowledgementRequests: AlertAcknowledgementRequests,
) {
  return renderHook(() => {
    const alerts = createMemo(() => Object.values(store.alertsById));
    return {
      ...useAlertAcknowledgementState({
        alerts,
        updateAlert: store.updateAlert,
        acknowledgementRequests,
        allowRestore: true,
      }),
      alerts,
      updateAlert: store.updateAlert,
      serverSends: store.serverSends,
      serverResolves: store.serverResolves,
    };
  });
}

// Each test gets its own store and its own request owner.
function renderWithAlertStore(initial: Alert[]) {
  const store = createAlertStore(initial);
  const acknowledgementRequests = createAlertAcknowledgementRequests();
  return { ...renderOverview(store, acknowledgementRequests), store, acknowledgementRequests };
}

describe('useAlertAcknowledgementState', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-22T12:00:00Z'));
    vi.mocked(AlertsAPI.acknowledge).mockReset();
    vi.mocked(AlertsAPI.unacknowledge).mockReset();
    vi.mocked(AlertsAPI.bulkAcknowledge).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.error).mockReset();
    vi.mocked(AlertsAPI.acknowledge).mockResolvedValue(undefined as never);
    vi.mocked(AlertsAPI.unacknowledge).mockResolvedValue(undefined as never);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const unacknowledgedIds = (result: ReturnType<typeof renderWithAlertStore>['result']) =>
    result.unacknowledgedAlerts().map((alert) => alert.id);

  it('shows an unacknowledge from another session after a local acknowledge', async () => {
    const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);

    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(AlertsAPI.acknowledge).toHaveBeenCalledWith('alert-1');
    expect(notificationStore.success).toHaveBeenCalledWith('Alert acknowledged');
    expect(unacknowledgedIds(result)).toEqual(['alert-2']);

    result.serverSends(acknowledgedBy('alert-1', 'admin'));
    expect(unacknowledgedIds(result)).toEqual(['alert-2']);
    expect(result.alerts()[0]).toMatchObject({ acknowledged: true, ackUser: 'admin' });

    // Unacknowledged from another session: the payload omits the ack fields.
    result.serverSends(makeAlert('alert-1'));
    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);
    expect(result.alerts()[0].acknowledged).toBe(false);
    expect(result.alerts()[0]).not.toHaveProperty('ackUser');

    // The card acknowledges again rather than restoring a stale local state.
    vi.advanceTimersByTime(1500);
    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(AlertsAPI.acknowledge).toHaveBeenCalledTimes(2);
    expect(AlertsAPI.unacknowledge).not.toHaveBeenCalled();
    expect(unacknowledgedIds(result)).toEqual(['alert-2']);
  });

  it('shows an acknowledge from another session after a local restore', async () => {
    const { result } = renderWithAlertStore([acknowledgedBy('alert-1', 'admin')]);
    expect(unacknowledgedIds(result)).toEqual([]);

    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(AlertsAPI.unacknowledge).toHaveBeenCalledWith('alert-1');
    expect(notificationStore.success).toHaveBeenCalledWith('Alert restored');
    expect(unacknowledgedIds(result)).toEqual(['alert-1']);

    result.serverSends(makeAlert('alert-1'));
    expect(unacknowledgedIds(result)).toEqual(['alert-1']);

    result.serverSends(acknowledgedBy('alert-1', 'operator'));
    expect(unacknowledgedIds(result)).toEqual([]);
    expect(result.alerts()[0]).toMatchObject({ acknowledged: true, ackUser: 'operator' });
  });

  it('acknowledges, restores and bulk-acknowledges through the shared alert store', async () => {
    const { result } = renderWithAlertStore([
      makeAlert('alert-1'),
      makeAlert('alert-2', true),
      makeAlert('alert-3'),
    ]);
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
      results: [
        { alertIdentifier: 'alert-2', success: true },
        { alertIdentifier: 'alert-3', success: false },
      ],
    } as never);

    expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-3']);

    await result.handleAlertAcknowledgement(result.alerts()[0]);
    expect(result.updateAlert).toHaveBeenCalledWith(
      'alert-1',
      expect.objectContaining({ acknowledged: true }),
    );
    expect(unacknowledgedIds(result)).toEqual(['alert-3']);

    vi.advanceTimersByTime(1500);
    expect(result.processingAlerts().has('alert-1')).toBe(false);

    await result.handleAlertAcknowledgement(result.alerts()[1]);
    expect(AlertsAPI.unacknowledge).toHaveBeenCalledWith('alert-2');
    expect(result.updateAlert).toHaveBeenCalledWith(
      'alert-2',
      expect.objectContaining({ acknowledged: false }),
    );
    expect(notificationStore.success).toHaveBeenCalledWith('Alert restored');
    expect(unacknowledgedIds(result)).toEqual(['alert-2', 'alert-3']);

    await result.handleBulkAcknowledge();
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-2', 'alert-3']);
    expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 1 alert.');
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
    expect(unacknowledgedIds(result)).toEqual(['alert-3']);
  });

  describe('group acknowledgement', () => {
    it('reports the alerts a group acknowledgement could not acknowledge', async () => {
      const { result } = renderWithAlertStore([
        makeAlert('alert-1'),
        makeAlert('alert-2'),
        makeAlert('alert-3'),
      ]);
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const settled = result.handleGroupAcknowledge(result.alerts());
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-1', 'alert-2', 'alert-3']);
      expect([...result.processingAlerts()]).toEqual(['alert-1', 'alert-2', 'alert-3']);
      expect(notificationStore.success).not.toHaveBeenCalled();

      response.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: false },
          { alertIdentifier: 'alert-3', success: false },
        ],
      });
      await settled;

      // The partial success keeps its count, and the failures are said beside it.
      expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 1 alert.');
      expect(notificationStore.error).toHaveBeenCalledWith(
        getAlertOverviewBulkAcknowledgeFailureNotification(2),
      );
      expect(result.updateAlert).toHaveBeenCalledTimes(1);
      expect(unacknowledgedIds(result)).toEqual(['alert-2', 'alert-3']);

      vi.advanceTimersByTime(1500);
      expect(result.processingAlerts().size).toBe(0);
    });

    it('says so when a group acknowledgement acknowledges none of its alerts', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const settled = result.handleGroupAcknowledge(result.alerts());
      response.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: false },
          { alertIdentifier: 'alert-2', success: false },
        ],
      });
      await settled;

      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).toHaveBeenCalledTimes(1);
      expect(notificationStore.error).toHaveBeenCalledWith(
        getAlertOverviewBulkAcknowledgeFailureNotification(2),
      );
      expect(result.updateAlert).not.toHaveBeenCalled();
      expect(unacknowledgedIds(result)).toEqual(['alert-1', 'alert-2']);
    });

    it('sends a failed alert again on retry while its card still shows processing', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockResolvedValueOnce({
          results: [
            { alertIdentifier: 'alert-1', success: false },
            { alertIdentifier: 'alert-2', success: true },
          ],
        })
        .mockResolvedValueOnce({ results: [{ alertIdentifier: 'alert-1', success: true }] });

      await result.handleGroupAcknowledge(result.alerts());
      expect(result.processingAlerts().has('alert-1')).toBe(true);

      await result.handleGroupAcknowledge(result.alerts());
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenNthCalledWith(2, ['alert-1']);
      expect(unacknowledgedIds(result)).toEqual([]);
    });

    it('does not send a group again while its acknowledgement is in flight', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const first = result.handleGroupAcknowledge(result.alerts());
      const second = result.handleGroupAcknowledge(result.alerts());
      // A card in the group waits for the same request.
      await result.handleAlertAcknowledgement(result.alerts()[0]);
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledTimes(1);
      expect(AlertsAPI.acknowledge).not.toHaveBeenCalled();

      response.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: true },
        ],
      });
      await Promise.all([first, second]);

      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        [getAlertOverviewBulkAcknowledgedNotification(2)],
      ]);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(unacknowledgedIds(result)).toEqual([]);
    });

    it('leaves an alert to its own request when its group is acknowledged meanwhile', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      const single = deferred<{ success: boolean }>();
      const bulk = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.acknowledge).mockReturnValueOnce(single.promise);
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(bulk.promise);

      const card = result.handleAlertAcknowledgement(result.alerts()[0]);
      const group = result.handleGroupAcknowledge(result.alerts());
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-2']);

      single.resolve({ success: true });
      await card;
      bulk.resolve({ results: [{ alertIdentifier: 'alert-2', success: true }] });
      await group;

      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        ['Alert acknowledged'],
        ['Acknowledged 1 alert.'],
      ]);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(unacknowledgedIds(result)).toEqual([]);
    });

    it('sends only the rest of a group while a global acknowledgement is out', async () => {
      const { result } = renderWithAlertStore([
        makeAlert('alert-1'),
        makeAlert('alert-2'),
        acknowledgedBy('alert-3', 'admin'),
      ]);
      const globalResponse = deferred<BulkAcknowledgeResult>();
      const groupResponse = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockReturnValueOnce(globalResponse.promise)
        .mockReturnValueOnce(groupResponse.promise);

      const globalAck = result.handleBulkAcknowledge();
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenLastCalledWith(['alert-1', 'alert-2']);

      // Another session unacknowledges alert-3 while the global request is out.
      result.serverSends(makeAlert('alert-3'));
      const groupAck = result.handleGroupAcknowledge([result.alerts()[1], result.alerts()[2]]);
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenLastCalledWith(['alert-3']);

      globalResponse.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: true },
        ],
      });
      groupResponse.resolve({ results: [{ alertIdentifier: 'alert-3', success: true }] });
      await Promise.all([globalAck, groupAck]);

      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        [getAlertOverviewBulkAcknowledgedNotification(2)],
        ['Acknowledged 1 alert.'],
      ]);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(unacknowledgedIds(result)).toEqual([]);
    });

    it('leaves a group its alerts when the global action runs meanwhile', async () => {
      const { result } = renderWithAlertStore([
        makeAlert('alert-1'),
        makeAlert('alert-2'),
        makeAlert('alert-3'),
      ]);
      const groupResponse = deferred<BulkAcknowledgeResult>();
      const globalResponse = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockReturnValueOnce(groupResponse.promise)
        .mockReturnValueOnce(globalResponse.promise);

      const groupAck = result.handleGroupAcknowledge([result.alerts()[0], result.alerts()[1]]);
      const globalAck = result.handleBulkAcknowledge();
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenNthCalledWith(1, ['alert-1', 'alert-2']);
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenNthCalledWith(2, ['alert-3']);

      globalResponse.resolve({ results: [{ alertIdentifier: 'alert-3', success: true }] });
      await globalAck;
      groupResponse.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: false },
        ],
      });
      await groupAck;

      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        ['Acknowledged 1 alert.'],
        ['Acknowledged 1 alert.'],
      ]);
      expect(vi.mocked(notificationStore.error).mock.calls).toEqual([
        ['Failed to acknowledge 1 alert.'],
      ]);
      expect(unacknowledgedIds(result)).toEqual(['alert-2']);
    });

    it('reports results for alerts resolved while the group request was out', async () => {
      const { result } = renderWithAlertStore([
        makeAlert('alert-1'),
        makeAlert('alert-2'),
        makeAlert('alert-3'),
      ]);
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const settled = result.handleGroupAcknowledge(result.alerts());
      result.serverResolves('alert-1');
      result.serverResolves('alert-3');
      response.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: true },
          { alertIdentifier: 'alert-3', success: false },
        ],
      });
      await settled;

      // The store ignores a write for an alert it no longer holds, so a
      // resolved alert does not come back as an acknowledged card.
      expect(result.alerts().map((alert) => alert.id)).toEqual(['alert-2']);
      expect(result.alerts()[0].acknowledged).toBe(true);
      expect(notificationStore.success).toHaveBeenCalledWith(
        getAlertOverviewBulkAcknowledgedNotification(2),
      );
      expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
    });

    it('reports a rejected group request once and releases its cards', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const settled = result.handleGroupAcknowledge(result.alerts());
      response.reject(new Error('Service Unavailable'));
      await settled;

      expect(vi.mocked(notificationStore.error).mock.calls).toEqual([
        [getAlertOverviewBulkAcknowledgeGenericFailureNotification()],
      ]);
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(result.updateAlert).not.toHaveBeenCalled();
      expect(result.processingAlerts().size).toBe(2);
      vi.advanceTimersByTime(1500);
      expect(result.processingAlerts().size).toBe(0);
    });

    it('still reports a group outcome that settles after the overview unmounts', async () => {
      const { result, cleanup } = renderWithAlertStore([
        makeAlert('alert-1'),
        makeAlert('alert-2'),
      ]);
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const settled = result.handleGroupAcknowledge(result.alerts());
      cleanup();
      response.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: true },
        ],
      });
      await settled;

      // The alert store and the notifications outlive the overview, so the
      // request's outcome still lands after it closes.
      expect(result.updateAlert).toHaveBeenCalledTimes(2);
      expect(notificationStore.success).toHaveBeenCalledWith(
        getAlertOverviewBulkAcknowledgedNotification(2),
      );
      expect(() => vi.advanceTimersByTime(1500)).not.toThrow();
    });
  });

  describe('request ownership', () => {
    it('shows the cards a global request carries as processing until it settles', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const globalAck = result.handleBulkAcknowledge();
      expect(result.bulkAckProcessing()).toBe(true);
      expect([...result.processingAlerts()]).toEqual(['alert-1', 'alert-2']);
      await result.handleAlertAcknowledgement(result.alerts()[0]);
      expect(AlertsAPI.acknowledge).not.toHaveBeenCalled();

      response.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: false },
        ],
      });
      await globalAck;

      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([['Acknowledged 1 alert.']]);
      expect(vi.mocked(notificationStore.error).mock.calls).toEqual([
        ['Failed to acknowledge 1 alert.'],
      ]);
      expect(result.bulkAckProcessing()).toBe(false);
      // A settled global request holds nothing, so the failed alert is ready at once.
      expect(result.processingAlerts().size).toBe(0);
      expect(unacknowledgedIds(result)).toEqual(['alert-2']);
    });

    it('retries a failed global request at once', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1')]);
      const retry = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockRejectedValueOnce(new Error('Service Unavailable'))
        .mockReturnValueOnce(retry.promise);

      await result.handleBulkAcknowledge();
      expect(vi.mocked(notificationStore.error).mock.calls).toEqual([
        [getAlertOverviewBulkAcknowledgeGenericFailureNotification()],
      ]);
      expect(result.processingAlerts().size).toBe(0);

      const retried = result.handleBulkAcknowledge();
      expect(vi.mocked(AlertsAPI.bulkAcknowledge).mock.calls).toEqual([
        [['alert-1']],
        [['alert-1']],
      ]);
      retry.resolve({ results: [{ alertIdentifier: 'alert-1', success: true }] });
      await retried;
      expect(unacknowledgedIds(result)).toEqual([]);
    });

    it('keeps a retried alert busy when the earlier hold ends', async () => {
      const { result } = renderWithAlertStore([makeAlert('alert-1')]);
      const retry = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockResolvedValueOnce({ results: [{ alertIdentifier: 'alert-1', success: false }] })
        .mockReturnValueOnce(retry.promise);

      await result.handleGroupAcknowledge(result.alerts());
      expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');

      vi.advanceTimersByTime(500);
      const retried = result.handleGroupAcknowledge(result.alerts());
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledTimes(2);

      // The first request's hold ends here while the retry is still out.
      vi.advanceTimersByTime(1000);
      expect(result.processingAlerts().has('alert-1')).toBe(true);
      await result.handleAlertAcknowledgement(result.alerts()[0]);
      expect(AlertsAPI.acknowledge).not.toHaveBeenCalled();

      retry.resolve({ results: [{ alertIdentifier: 'alert-1', success: true }] });
      await retried;
      expect(unacknowledgedIds(result)).toEqual([]);
      expect(result.processingAlerts().has('alert-1')).toBe(true);
      vi.advanceTimersByTime(1500);
      expect(result.processingAlerts().has('alert-1')).toBe(false);
    });

    it('leaves a restore its alert when its broadcast arrives before its response', async () => {
      const { result } = renderWithAlertStore([
        acknowledgedBy('alert-1', 'admin'),
        makeAlert('alert-2'),
      ]);
      const restore = deferred<{ success: boolean }>();
      const bulk = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.unacknowledge).mockReturnValueOnce(restore.promise);
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(bulk.promise);

      const restoring = result.handleAlertAcknowledgement(result.alerts()[0]);
      // The restore's own broadcast reaches the store before its HTTP response.
      result.serverSends(makeAlert('alert-1'));
      const globalAck = result.handleBulkAcknowledge();
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-2']);

      restore.resolve({ success: true });
      await restoring;
      bulk.resolve({ results: [{ alertIdentifier: 'alert-2', success: true }] });
      await globalAck;

      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        ['Alert restored'],
        ['Acknowledged 1 alert.'],
      ]);
      expect(unacknowledgedIds(result)).toEqual(['alert-1']);
    });

    it('keeps a pending group with the app when the overview remounts', async () => {
      const store = createAlertStore([
        makeAlert('alert-1'),
        makeAlert('alert-2'),
        makeAlert('alert-3'),
      ]);
      const acknowledgementRequests = createAlertAcknowledgementRequests();
      const groupResponse = deferred<BulkAcknowledgeResult>();
      const globalResponse = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockReturnValueOnce(groupResponse.promise)
        .mockReturnValueOnce(globalResponse.promise);

      const first = renderOverview(store, acknowledgementRequests);
      const groupAck = first.result.handleGroupAcknowledge([
        first.result.alerts()[0],
        first.result.alerts()[1],
      ]);
      first.cleanup();

      const second = renderOverview(store, acknowledgementRequests);
      expect([...second.result.processingAlerts()]).toEqual(['alert-1', 'alert-2']);
      await second.result.handleAlertAcknowledgement(second.result.alerts()[0]);
      await second.result.handleGroupAcknowledge([
        second.result.alerts()[0],
        second.result.alerts()[1],
      ]);
      const globalAck = second.result.handleBulkAcknowledge();
      expect(AlertsAPI.acknowledge).not.toHaveBeenCalled();
      expect(vi.mocked(AlertsAPI.bulkAcknowledge).mock.calls).toEqual([
        [['alert-1', 'alert-2']],
        [['alert-3']],
      ]);

      // The first view's request answers after the second view mounted.
      groupResponse.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: false },
        ],
      });
      await groupAck;
      globalResponse.resolve({ results: [{ alertIdentifier: 'alert-3', success: true }] });
      await globalAck;

      expect(store.updateAlert).toHaveBeenCalledWith(
        'alert-1',
        expect.objectContaining({ acknowledged: true }),
      );
      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        ['Acknowledged 1 alert.'],
        ['Acknowledged 1 alert.'],
      ]);
      expect(vi.mocked(notificationStore.error).mock.calls).toEqual([
        ['Failed to acknowledge 1 alert.'],
      ]);
      expect(unacknowledgedIds(second.result)).toEqual(['alert-2']);
      expect(second.result.processingAlerts().has('alert-2')).toBe(true);
      vi.advanceTimersByTime(1500);
      expect(second.result.processingAlerts().size).toBe(0);
    });

    it('shows a global request from an earlier view as in progress', async () => {
      const store = createAlertStore([makeAlert('alert-1'), makeAlert('alert-2')]);
      const acknowledgementRequests = createAlertAcknowledgementRequests();
      const response = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge).mockReturnValueOnce(response.promise);

      const first = renderOverview(store, acknowledgementRequests);
      const globalAck = first.result.handleBulkAcknowledge();
      first.cleanup();

      const second = renderOverview(store, acknowledgementRequests);
      expect(second.result.bulkAckProcessing()).toBe(true);
      expect([...second.result.processingAlerts()]).toEqual(['alert-1', 'alert-2']);
      await second.result.handleBulkAcknowledge();
      await second.result.handleAlertAcknowledgement(second.result.alerts()[0]);
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledTimes(1);
      expect(AlertsAPI.acknowledge).not.toHaveBeenCalled();

      response.resolve({
        results: [
          { alertIdentifier: 'alert-1', success: true },
          { alertIdentifier: 'alert-2', success: true },
        ],
      });
      await globalAck;

      expect(second.result.bulkAckProcessing()).toBe(false);
      expect(unacknowledgedIds(second.result)).toEqual([]);
      expect(notificationStore.success).toHaveBeenCalledWith(
        getAlertOverviewBulkAcknowledgedNotification(2),
      );
    });

    it('keeps a request to the app that owns it', async () => {
      const appRequests = createAlertAcknowledgementRequests();
      const appResponse = deferred<BulkAcknowledgeResult>();
      const otherResponse = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockReturnValueOnce(appResponse.promise)
        .mockReturnValueOnce(otherResponse.promise);

      const app = renderOverview(createAlertStore([makeAlert('alert-1')]), appRequests);
      const other = renderOverview(
        createAlertStore([makeAlert('alert-1')]),
        createAlertAcknowledgementRequests(),
      );
      const appAck = app.result.handleGroupAcknowledge(app.result.alerts());
      // Another app (or test) with its own owner is not held by this request.
      const otherAck = other.result.handleGroupAcknowledge(other.result.alerts());
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledTimes(2);

      appRequests.reset();
      expect(app.result.processingAlerts().size).toBe(0);
      expect(other.result.processingAlerts().has('alert-1')).toBe(true);

      appResponse.resolve({ results: [{ alertIdentifier: 'alert-1', success: true }] });
      otherResponse.resolve({ results: [{ alertIdentifier: 'alert-1', success: true }] });
      await Promise.all([appAck, otherAck]);
      expect(unacknowledgedIds(app.result)).toEqual(['alert-1']);
      expect(unacknowledgedIds(other.result)).toEqual([]);
      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([['Acknowledged 1 alert.']]);
    });

    it('drops responses from an ended alert scope without touching the next one', async () => {
      const ids = ['alert-1', 'alert-2', 'alert-3', 'alert-4'];
      const { result, store, acknowledgementRequests } = renderWithAlertStore(
        ids.map((id) => makeAlert(id)),
      );
      vi.mocked(logger.error).mockClear();
      const oldCard = deferred<{ success: boolean }>();
      const oldGroup = deferred<BulkAcknowledgeResult>();
      const oldGlobal = deferred<BulkAcknowledgeResult>();
      const newGlobal = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.acknowledge).mockReturnValueOnce(oldCard.promise);
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockReturnValueOnce(oldGroup.promise)
        .mockReturnValueOnce(oldGlobal.promise)
        .mockReturnValueOnce(newGlobal.promise);

      const cardAck = result.handleAlertAcknowledgement(result.alerts()[0]);
      const groupAck = result.handleGroupAcknowledge([result.alerts()[1], result.alerts()[2]]);
      const globalAck = result.handleBulkAcknowledge();
      expect(vi.mocked(AlertsAPI.bulkAcknowledge).mock.calls).toEqual([
        [['alert-2', 'alert-3']],
        [['alert-4']],
      ]);

      // An organization switch: the scope ends and the next one holds alerts
      // with the same identifiers.
      acknowledgementRequests.reset();
      ids.forEach((id) => store.serverSends({ ...makeAlert(id), message: `next scope ${id}` }));
      expect(result.processingAlerts().size).toBe(0);
      expect(result.bulkAckProcessing()).toBe(false);
      const nextAck = result.handleBulkAcknowledge();
      expect(vi.mocked(AlertsAPI.bulkAcknowledge).mock.calls.at(-1)).toEqual([ids]);

      // The old scope answers: a success, a partial failure and a rejection.
      oldCard.resolve({ success: true });
      oldGroup.resolve({
        results: [
          { alertIdentifier: 'alert-2', success: true },
          { alertIdentifier: 'alert-3', success: false },
        ],
      });
      oldGlobal.reject(new Error('Service Unavailable'));
      await Promise.all([cardAck, groupAck, globalAck]);

      expect(store.updateAlert).not.toHaveBeenCalled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      // The rejection is still logged; it reaches no one's screen.
      expect(logger.error).toHaveBeenCalledWith('Bulk acknowledge failed', expect.any(Error));
      expect(unacknowledgedIds(result)).toEqual(ids);
      // Nothing the next scope's request carries was released.
      expect([...result.processingAlerts()]).toEqual(ids);
      expect(result.bulkAckProcessing()).toBe(true);

      newGlobal.resolve({
        results: ids.map((alertIdentifier) => ({ alertIdentifier, success: true })),
      });
      await nextAck;
      expect(store.updateAlert).toHaveBeenCalledTimes(4);
      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        [getAlertOverviewBulkAcknowledgedNotification(4)],
      ]);
      expect(unacknowledgedIds(result)).toEqual([]);
      expect(result.alerts().map((alert) => alert.message)).toEqual(
        ids.map((id) => `next scope ${id}`),
      );
    });
  });
});
