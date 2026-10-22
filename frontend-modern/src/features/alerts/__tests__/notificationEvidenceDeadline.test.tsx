import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { useNotificationDeliveryHealth } from '../useNotificationDeliveryHealth';
import { useNotificationDeliveryLog } from '../useNotificationDeliveryLog';
import { NOTIFICATION_EVIDENCE_READ_TIMEOUT_MS } from '../notificationEvidenceRead';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

const rawHealth = (degraded = false) => ({
  overall_healthy: !degraded,
  queue: {
    pending: 0,
    sending: 0,
    sent: 1,
    failed: 0,
    dlq: degraded ? 2 : 0,
    healthy: !degraded,
    status: degraded ? 'degraded' : 'healthy',
    attention_required: degraded ? 2 : 0,
    reason_codes: [],
    completed_retention_days: 7,
    dead_letter_retention_days: 30,
    counts_are_retention_bounded: true,
    retry_attempts_affect_health: false,
    terminal_failures_affect_health: true,
    failure_classes_7d: {
      authentication: 0,
      rate_limited: 0,
      connectivity: 0,
      tls: 0,
      configuration: 0,
      rejected: 0,
      server_error: 0,
      unknown: 0,
    },
    failure_classes_available: true,
    failure_class_window_days: 7,
  },
});
const rawLog = {
  entries: [],
  window_days: 7,
  completed_retention_days: 7,
  dead_letter_retention_days: 30,
};
const deferred = () => {
  let resolve!: (value: unknown) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<unknown>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const mount = () => {
  let dispose!: () => void;
  const state = createRoot((stop) => {
    dispose = stop;
    const log = useNotificationDeliveryLog();
    const health = useNotificationDeliveryHealth({ onAfterQueueAction: log.loadDeliveryLog });
    return { ...log, ...health };
  });
  return { state, dispose };
};
const endpoints = {
  health: '/api/notifications/health',
  attempts: '/api/notifications/delivery-log?limit=200',
  held: '/api/alerts/events?',
};
const settle = async () => {
  // The real adapter, independent reader and hook each settle asynchronously.
  for (let i = 0; i < 12; i++) await Promise.resolve();
};

describe('bounded notification evidence through the real API adapters', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetAllMocks();
    vi.mocked(apiFetchJSON).mockImplementation(async (url) => {
      if (url === endpoints.health) return rawHealth();
      if (url.startsWith(endpoints.held)) return [];
      return rawLog;
    });
  });
  afterEach(() => vi.useRealTimers());

  it.each(['health', 'attempts', 'held'] as const)(
    '%s expires independently, ignores late success/failure and recovers only on an explicit read',
    async (stream) => {
      const late = deferred();
      let signal!: AbortSignal;
      let blocked = true;
      vi.mocked(apiFetchJSON).mockImplementation(async (url, options) => {
        if (blocked && url.startsWith(endpoints[stream])) {
          signal = options!.signal as AbortSignal;
          return late.promise;
        }
        return url === endpoints.health
          ? rawHealth()
          : url.startsWith(endpoints.held)
            ? []
            : rawLog;
      });
      const { state, dispose } = mount();
      try {
        const pending = Promise.all([state.loadDeliveryHealth(), state.loadDeliveryLog()]);
        await settle();
        expect(signal.aborted).toBe(false);
        await vi.advanceTimersByTimeAsync(NOTIFICATION_EVIDENCE_READ_TIMEOUT_MS - 1);
        expect(signal.aborted).toBe(false);
        if (stream === 'health') expect(state.deliveryNeedsAttention()).toBe(false);
        if (stream === 'attempts') expect(state.refreshingDeliveryLog()).toBe(true);
        if (stream === 'held') expect(state.refreshingHeldEvents()).toBe(true);
        await vi.advanceTimersByTimeAsync(1);
        await pending;
        expect(signal.aborted).toBe(true);
        expect(state.refreshingDeliveryHealth()).toBe(false);
        expect(state.refreshingDeliveryLog()).toBe(false);
        expect(state.refreshingHeldEvents()).toBe(false);
        expect(state.deliveryHealthUnavailable()).toBe(stream === 'health');
        expect(state.deliveryNeedsAttention()).toBe(stream === 'health');
        expect(state.deliveryLogUnavailable()).toBe(stream === 'attempts');
        expect(state.heldEventsUnavailable()).toBe(stream === 'held');
        expect(apiFetchJSON).toHaveBeenCalledTimes(3);
        // A late transport is ignored, even if it never honoured abort.
        late.resolve(stream === 'health' ? rawHealth() : stream === 'held' ? [] : rawLog);
        await settle();
        expect(state.deliveryHealthUnavailable()).toBe(stream === 'health');
        expect(state.deliveryLogUnavailable()).toBe(stream === 'attempts');
        expect(state.heldEventsUnavailable()).toBe(stream === 'held');
        blocked = false;
        await state.loadDeliveryHealth();
        await state.loadDeliveryLog();
        await settle();
        expect(state.deliveryHealthUnavailable()).toBe(false);
        expect(state.deliveryLogUnavailable()).toBe(false);
        expect(state.heldEventsUnavailable()).toBe(false);
        expect(vi.getTimerCount()).toBe(0);
      } finally {
        dispose();
      }
    },
  );

  it('replacing a refresh aborts all older reads without waiting, preserving newer evidence', async () => {
    const late = deferred();
    const signals: AbortSignal[] = [];
    vi.mocked(apiFetchJSON)
      .mockImplementationOnce((_url, options) => {
        signals.push(options!.signal as AbortSignal);
        return late.promise;
      })
      .mockImplementationOnce((_url, options) => {
        signals.push(options!.signal as AbortSignal);
        return late.promise;
      })
      .mockImplementationOnce((_url, options) => {
        signals.push(options!.signal as AbortSignal);
        return late.promise;
      });
    const { state, dispose } = mount();
    try {
      const old = Promise.all([state.loadDeliveryHealth(), state.loadDeliveryLog()]);
      await Promise.all([state.loadDeliveryHealth(), state.loadDeliveryLog()]);
      await old;
      await settle();
      expect(signals).toHaveLength(3);
      expect(signals.every((signal) => signal.aborted)).toBe(true);
      expect(state.deliveryHealth()?.queue.status).toBe('healthy');
      expect(state.deliveryLog()?.entries).toEqual([]);
      expect(state.heldEventsUnavailable()).toBe(false);
      late.reject(new Error('old read failed after replacement'));
      await settle();
      expect(logger.error).not.toHaveBeenCalled();
      expect(state.deliveryNeedsAttention()).toBe(false);
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      dispose();
    }
  });

  it('disposal settles and aborts pending reads, clears timers and publishes nothing', async () => {
    const late = deferred();
    const signals: AbortSignal[] = [];
    vi.mocked(apiFetchJSON).mockImplementation((_url, options) => {
      signals.push(options!.signal as AbortSignal);
      return late.promise;
    });
    const { state, dispose } = mount();
    const pending = Promise.all([state.loadDeliveryHealth(), state.loadDeliveryLog()]);
    dispose();
    await pending;
    expect(signals).toHaveLength(3);
    expect(signals.every((signal) => signal.aborted)).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
    late.reject(new Error('retired transport failure'));
    await settle();
    expect(logger.error).not.toHaveBeenCalled();
    expect(state.deliveryHealthUnavailable()).toBe(false);
    expect(state.deliveryLogUnavailable()).toBe(false);
    expect(state.heldEventsUnavailable()).toBe(false);
  });

  it.each(['retryTerminalFailures', 'dismissTerminalFailures'] as const)(
    'an acknowledged %s is not reissued or called failed when its evidence readback stalls',
    async (action) => {
      const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true);
      const { state, dispose } = mount();
      try {
        vi.mocked(apiFetchJSON).mockResolvedValueOnce(rawHealth(true));
        await state.loadDeliveryHealth();
        vi.mocked(apiFetchJSON).mockImplementation(async (_url, options) => {
          if (options?.method === 'POST') return { success: true, affected: 2 };
          return new Promise(() => {});
        });
        const pending = state[action]();
        await settle();
        await vi.advanceTimersByTimeAsync(NOTIFICATION_EVIDENCE_READ_TIMEOUT_MS);
        await pending;
        expect(notificationStore.success).toHaveBeenCalledOnce();
        expect(notificationStore.error).not.toHaveBeenCalled();
        expect(state.queueActionFeedback()).toBeNull();
        expect(state.deliveryHealthUnavailable()).toBe(true);
        expect(state.deliveryLogUnavailable()).toBe(true);
        expect(state.heldEventsUnavailable()).toBe(true);
        expect(state.retryingTerminalFailures()).toBe(false);
        expect(state.dismissingTerminalFailures()).toBe(false);
        expect(
          vi.mocked(apiFetchJSON).mock.calls.filter(([, options]) => options?.method === 'POST'),
        ).toHaveLength(1);
        expect(vi.getTimerCount()).toBe(0);
      } finally {
        dispose();
        confirm.mockRestore();
      }
    },
  );
});
