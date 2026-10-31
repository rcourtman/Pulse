import { renderHook } from '@solidjs/testing-library';
import { createMemo, createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { useAlertAcknowledgementState } from '@/features/alerts/useAlertAcknowledgementState';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import { getAlertOverviewBulkAcknowledgedNotification } from '@/utils/alertOverviewPresentation';
import { logger } from '@/utils/logger';

// The real alert store owns the acknowledgement request record. These tests
// build it over the same inert WebSocket stand-in as websocket-unified.test.ts:
// no socket opens and no request leaves the test.
const apiFetchJSONMock = vi.fn();
vi.mock('@/utils/apiClient', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/apiClient')>();
  return {
    ...actual,
    apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
  };
});

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
    info: vi.fn(),
    warning: vi.fn(),
  },
}));

interface MockWebSocketInstance {
  url: string;
  readyState: number;
  send: ReturnType<typeof vi.fn>;
  close: ReturnType<typeof vi.fn>;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
}

let mockWsInstance: MockWebSocketInstance | null = null;

const MockWebSocket = vi.fn().mockImplementation(function (url: string): MockWebSocketInstance {
  const instance: MockWebSocketInstance = {
    url,
    readyState: 1,
    send: vi.fn(),
    close: vi.fn(),
    onopen: null,
    onmessage: null,
    onclose: null,
    onerror: null,
  };
  mockWsInstance = instance;
  setTimeout(() => {
    instance.onopen?.({} as Event);
  }, 0);
  return instance;
});

const installWebSocketMock = () => {
  vi.stubGlobal(
    'WebSocket',
    Object.assign(MockWebSocket, { CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3 }),
  );
};

const waitForOpenTick = async () => {
  vi.advanceTimersByTime(1);
  await Promise.resolve();
};

const emitInitialAlerts = (lastUpdate: number, activeAlerts: unknown[]) => {
  if (!mockWsInstance?.onmessage) {
    throw new Error('WebSocket onmessage handler is not initialized');
  }
  mockWsInstance.onmessage({
    data: JSON.stringify({
      type: 'initialState',
      data: { resources: [], lastUpdate, activeAlerts, recentlyResolved: [] },
    }),
  } as MessageEvent);
};

const createStoreHarness = async () => {
  const { createWebSocketStore } = await import('@/stores/websocket');
  let dispose = () => {};
  const store = createRoot((d) => {
    dispose = d;
    return createWebSocketStore('ws://localhost/ws');
  });
  return { store, dispose };
};

const alertPayload = (id: string, message: string) => ({
  id,
  type: 'cpu',
  level: 'warning',
  resourceId: `vm-${id}`,
  resourceName: `VM ${id}`,
  node: 'node-1',
  message,
  startTime: '2026-10-30T10:00:00Z',
  acknowledged: false,
});

type BulkAcknowledgeResult = Awaited<ReturnType<typeof AlertsAPI.bulkAcknowledge>>;

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((settle, fail) => {
    resolve = settle;
    reject = fail;
  });
  return { promise, resolve, reject };
}

describe('alert store acknowledgement request lifetime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetModules();
    mockWsInstance = null;
    MockWebSocket.mockClear();
    apiFetchJSONMock.mockReset();
    vi.mocked(AlertsAPI.acknowledge).mockReset();
    vi.mocked(AlertsAPI.bulkAcknowledge).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.error).mockReset();
    installWebSocketMock();
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('keeps one request owner for the store and drops it on an organization switch', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      const requests = store.alertAcknowledgementRequests;
      expect(store.alertAcknowledgementRequests).toBe(requests);

      const settled = requests.claim('group', ['a'])!;
      requests.release(settled);
      const pending = requests.claim('bulk', ['b'])!;
      expect([...requests.processingAlerts()]).toEqual(['a', 'b']);

      store.switchUrl('ws://localhost/ws?org=another');
      expect(store.alertAcknowledgementRequests).toBe(requests);
      expect(requests.processingAlerts().size).toBe(0);
      expect(requests.bulkInFlight()).toBe(false);
      expect(requests.isCurrent(pending)).toBe(false);

      // The next organization's request is not released by the old one.
      const next = requests.claim('bulk', ['b'])!;
      requests.release(pending);
      expect(requests.isCurrent(next)).toBe(true);
    } finally {
      dispose();
    }
  });

  it.each([true, false])(
    'drops settled acknowledgement guards when switching alert scopes (ack=%s)',
    async (acknowledged) => {
      const { store, dispose } = await createStoreHarness();
      try {
        await waitForOpenTick();
        emitInitialAlerts(100, [
          { ...alertPayload('same-id', 'first scope'), acknowledged: !acknowledged },
        ]);
        store.updateAlert('same-id', { acknowledged });
        expect(store.activeAlerts['same-id']?.acknowledged).toBe(acknowledged);

        store.switchUrl('ws://localhost/ws?org=next');
        await waitForOpenTick();
        emitInitialAlerts(200, [
          { ...alertPayload('same-id', 'next scope'), acknowledged: !acknowledged },
        ]);
        // The first scope's confirmation guard must not suppress an unrelated
        // alert with the same identifier in this scope.
        expect(store.activeAlerts['same-id']).toMatchObject({
          message: 'next scope',
          acknowledged: !acknowledged,
        });
        vi.advanceTimersByTime(15001);
        await Promise.resolve();
        expect(notificationStore.error).not.toHaveBeenCalled();
      } finally {
        dispose();
      }
    },
  );

  it('retains request ownership and confirmation guards during same-scope reconnect', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      emitInitialAlerts(100, [alertPayload('same-id', 'current scope')]);
      store.updateAlert('same-id', { acknowledged: true });
      const requests = store.alertAcknowledgementRequests;
      const request = requests.claim('bulk', ['another-alert'])!;

      store.reconnect();
      await waitForOpenTick();
      emitInitialAlerts(200, [alertPayload('same-id', 'stale server view')]);
      expect(store.activeAlerts['same-id']?.acknowledged).toBe(true);
      expect(requests.isCurrent(request)).toBe(true);
      expect(requests.processingAlerts().has('another-alert')).toBe(true);

      emitInitialAlerts(300, [
        { ...alertPayload('same-id', 'server confirmed'), acknowledged: true },
      ]);
      emitInitialAlerts(400, [alertPayload('same-id', 'restored by another session')]);
      expect(store.activeAlerts['same-id']).toMatchObject({
        acknowledged: false,
        message: 'restored by another session',
      });
    } finally {
      dispose();
    }
  });

  it('drops the request owner on shutdown', async () => {
    const { store, dispose } = await createStoreHarness();
    try {
      await waitForOpenTick();
      const requests = store.alertAcknowledgementRequests;
      requests.release(requests.claim('alert', ['a'])!);
      requests.claim('group', ['b']);

      store.shutdown();
      expect(requests.processingAlerts().size).toBe(0);
      expect(requests.isOwned('b')).toBe(false);
    } finally {
      dispose();
    }
  });

  it('drops the request owner when the store root is disposed', async () => {
    const { store, dispose } = await createStoreHarness();
    await waitForOpenTick();
    const requests = store.alertAcknowledgementRequests;
    requests.claim('group', ['a']);

    dispose();
    expect(requests.isOwned('a')).toBe(false);
    expect(requests.processingAlerts().size).toBe(0);
  });

  it('keeps responses from before an organization switch out of the next one', async () => {
    const { store, dispose } = await createStoreHarness();
    const errorLog = vi.spyOn(logger, 'error').mockImplementation(() => {});
    try {
      await waitForOpenTick();
      const ids = ['cpu-1', 'cpu-2', 'cpu-3', 'cpu-4'];
      emitInitialAlerts(
        100,
        ids.map((id) => alertPayload(id, `first ${id}`)),
      );
      expect(Object.keys(store.activeAlerts).sort()).toEqual(ids);

      const updateAlert = vi.fn((id: string, updates: Partial<Alert>) =>
        store.updateAlert(id, updates),
      );
      const { result } = renderHook(() => {
        const alerts = createMemo(() => Object.values(store.activeAlerts));
        return {
          ...useAlertAcknowledgementState({
            alerts,
            updateAlert,
            acknowledgementRequests: store.alertAcknowledgementRequests,
            allowRestore: true,
          }),
          alerts,
        };
      });
      const byId = (id: string) => result.alerts().find((alert) => alert.id === id)!;

      const oldCard = deferred<{ success: boolean }>();
      const oldGroup = deferred<BulkAcknowledgeResult>();
      const oldGlobal = deferred<BulkAcknowledgeResult>();
      const nextGlobal = deferred<BulkAcknowledgeResult>();
      vi.mocked(AlertsAPI.acknowledge).mockReturnValueOnce(oldCard.promise);
      vi.mocked(AlertsAPI.bulkAcknowledge)
        .mockReturnValueOnce(oldGroup.promise)
        .mockReturnValueOnce(oldGlobal.promise)
        .mockReturnValueOnce(nextGlobal.promise);

      const cardAck = result.handleAlertAcknowledgement(byId('cpu-1'));
      const groupAck = result.handleGroupAcknowledge([byId('cpu-2'), byId('cpu-3')]);
      const globalAck = result.handleBulkAcknowledge();
      expect(vi.mocked(AlertsAPI.bulkAcknowledge).mock.calls).toEqual([
        [['cpu-2', 'cpu-3']],
        [['cpu-4']],
      ]);

      // The organization switch reuses the store; the next organization has
      // alerts with the same identifiers.
      store.switchUrl('ws://localhost/ws?org=next');
      await waitForOpenTick();
      emitInitialAlerts(
        200,
        ids.map((id) => alertPayload(id, `next ${id}`)),
      );
      expect(result.processingAlerts().size).toBe(0);
      const nextAck = result.handleBulkAcknowledge();
      expect([...vi.mocked(AlertsAPI.bulkAcknowledge).mock.calls.at(-1)![0]].sort()).toEqual(ids);

      // The old organization answers: a success, a partial failure, a rejection.
      oldCard.resolve({ success: true });
      oldGroup.resolve({
        results: [
          { alertIdentifier: 'cpu-2', success: true },
          { alertIdentifier: 'cpu-3', success: false },
        ],
      });
      oldGlobal.reject(new Error('Service Unavailable'));
      await Promise.all([cardAck, groupAck, globalAck]);

      expect(updateAlert).not.toHaveBeenCalled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(errorLog).toHaveBeenCalledWith('Bulk acknowledge failed', expect.any(Error));
      ids.forEach((id) => {
        expect(store.activeAlerts[id]).toMatchObject({
          acknowledged: false,
          message: `next ${id}`,
        });
      });
      expect([...result.processingAlerts()].sort()).toEqual(ids);

      nextGlobal.resolve({
        results: ids.map((alertIdentifier) => ({ alertIdentifier, success: true })),
      });
      await nextAck;
      expect(updateAlert).toHaveBeenCalledTimes(4);
      ids.forEach((id) => expect(store.activeAlerts[id]?.acknowledged).toBe(true));
      expect(vi.mocked(notificationStore.success).mock.calls).toEqual([
        [getAlertOverviewBulkAcknowledgedNotification(4)],
      ]);
    } finally {
      errorLog.mockRestore();
      dispose();
    }
  });
});
