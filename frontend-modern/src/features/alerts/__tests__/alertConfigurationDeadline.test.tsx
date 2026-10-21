import { renderHook } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { hasFeature } from '@/stores/license';
import { logger } from '@/utils/logger';
import { useAlertDestinationsState } from '../useAlertDestinationsState';
import { useAlertsConfigurationState } from '../useAlertsConfigurationState';
import { NOTIFICATION_EVIDENCE_READ_TIMEOUT_MS as LIMIT } from '../notificationEvidenceRead';

vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: vi.fn() }));
vi.mock('@/stores/license', () => ({ hasFeature: vi.fn() }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
const payloads: Record<string, unknown> = {
  '/api/alerts/config': { enabled: true, activationState: 'active', overrides: {} },
  '/api/notifications/email': { enabled: true, server: 'smtp.saved.example.test', port: 587 },
  '/api/notifications/apprise': { enabled: false },
  '/api/notifications/webhooks': [],
  '/api/alerts/deadman/config': { pingUrl: '', configured: false },
  '/api/settings/relay': { enabled: true, server_url: 'wss://relay.example.test' },
};
const paths = Object.keys(payloads).filter((path) => path !== '/api/alerts/config');
const deferred = () => {
  let resolve!: (value: unknown) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<unknown>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const settle = async () => {
  for (let i = 0; i < 20; i++) await Promise.resolve();
};
const mountPolicy = () =>
  renderHook(() =>
    useAlertsConfigurationState({
      activeTab: () => 'destinations',
      allResources: () => [],
      byType: () => [],
      children: () => [],
      activeAlerts: {},
      removeAlerts: vi.fn(),
      setOverviewOverrides: vi.fn(),
      hasUnsavedChanges: () => true,
      setHasUnsavedChanges: vi.fn(),
      alertsActivationState: () => 'active',
      alertsActivationConfig: () => ({ enabled: true }),
    }),
  );

describe('bounded alert configuration reads through the real adapters', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetAllMocks();
    vi.mocked(hasFeature).mockReturnValue(true);
    vi.mocked(apiFetchJSON).mockImplementation(async (path) => payloads[path]);
  });
  afterEach(() => vi.useRealTimers());

  it.each(paths)('releases a stalled %s without accepting defaults or retrying', async (path) => {
    const old = deferred();
    let signal!: AbortSignal;
    vi.mocked(apiFetchJSON).mockImplementation((url, options) => {
      if (url === path) {
        signal = options!.signal as AbortSignal;
        return old.promise as never;
      }
      return Promise.resolve(payloads[url]) as never;
    });
    const { result, cleanup } = renderHook(() =>
      useAlertDestinationsState({ activeTab: () => 'destinations' }),
    );
    try {
      const read = result.loadDestinations({ indicateLoading: true });
      await vi.advanceTimersByTimeAsync(LIMIT - 1);
      expect(result.isLoadingDestinations()).toBe(true);
      expect(result.destConfigLoadError()).toBeNull();
      expect(signal.aborted).toBe(false);
      await vi.advanceTimersByTimeAsync(1);
      await read;
      expect(result.isLoadingDestinations()).toBe(false);
      expect(result.destConfigLoadError()).toMatch(/could not be retrieved/);
      expect(signal.aborted).toBe(true);
      expect(apiFetchJSON).toHaveBeenCalledTimes(5);
      // The next read is explicit, not an automatic timeout retry.
      vi.mocked(apiFetchJSON).mockImplementation(async (url) => payloads[url]);
      await result.loadDestinations({ indicateLoading: true });
      expect(result.destConfigLoadError()).toBeNull();
      expect(result.emailConfig().server).toBe('smtp.saved.example.test');
      old.resolve(
        path === '/api/notifications/email'
          ? { enabled: true, server: 'smtp.stale.example.test' }
          : payloads[path],
      );
      await settle();
      expect(result.destConfigLoadError()).toBeNull();
      expect(result.emailConfig().server).toBe('smtp.saved.example.test');
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      cleanup();
    }
  });

  it.each(['reset', 'supersede', 'unmount'] as const)(
    'retires every stalled stream on %s',
    async (operation) => {
      const pending = paths.map(() => deferred());
      const signals: AbortSignal[] = [];
      vi.mocked(apiFetchJSON).mockImplementation((path, options) => {
        signals.push(options!.signal as AbortSignal);
        return pending[paths.indexOf(path)].promise as never;
      });
      const { result, cleanup } = renderHook(() =>
        useAlertDestinationsState({ activeTab: () => 'destinations' }),
      );
      const read = result.loadDestinations({ indicateLoading: true });
      if (operation === 'unmount') cleanup();
      else if (operation === 'reset') result.resetDestinations();
      else {
        vi.mocked(apiFetchJSON).mockImplementation(async (path) => payloads[path]);
        await result.loadDestinations();
      }
      await read;
      expect(signals).toHaveLength(5);
      expect(signals.every((signal) => signal.aborted)).toBe(true);
      expect(result.destConfigLoadError()).toBeNull();
      expect(logger.error).not.toHaveBeenCalled();
      expect(vi.getTimerCount()).toBe(0);
      pending.forEach((value) => value.reject(new Error('late retired read')));
      await settle();
      expect(logger.error).not.toHaveBeenCalled();
      if (operation !== 'unmount') cleanup();
      await result.loadDestinations();
      expect(apiFetchJSON).toHaveBeenCalledTimes(operation === 'supersede' ? 10 : 5);
    },
  );

  it.each(['/api/alerts/config', ...paths])(
    'blocks all policy writes after %s expires and permits explicit recovery',
    async (path) => {
      const stuck = deferred();
      let signal!: AbortSignal;
      vi.mocked(apiFetchJSON).mockImplementation((url, options) => {
        if (url === path) {
          signal = options!.signal as AbortSignal;
          return stuck.promise as never;
        }
        return Promise.resolve(payloads[url]) as never;
      });
      const { result, cleanup } = mountPolicy();
      try {
        await settle();
        expect(result.isReloadingConfig()).toBe(true);
        await vi.advanceTimersByTimeAsync(LIMIT);
        expect(result.isReloadingConfig()).toBe(false);
        expect(result.configLoadError() || result.destConfigLoadError()).toBeTruthy();
        expect(signal.aborted).toBe(true);
        await result.saveAlertConfiguration();
        expect(vi.mocked(apiFetchJSON).mock.calls.every(([, options]) => !options?.method)).toBe(
          true,
        );
        vi.mocked(apiFetchJSON).mockImplementation(async (url) => payloads[url]);
        await result.loadAlertConfiguration();
        expect(result.isConfigLoaded()).toBe(true);
        expect(result.destConfigLoadError()).toBeNull();
        expect(result.configLoadError()).toBeNull();
        expect(result.emailConfig().server).toBe('smtp.saved.example.test');
        stuck.reject(new Error('late read after retry'));
        await settle();
        expect(result.emailConfig().server).toBe('smtp.saved.example.test');
      } finally {
        cleanup();
      }
    },
  );

  it.each(['supersede', 'unmount'] as const)(
    'retires a policy read on %s before its deadline',
    async (operation) => {
      const old = deferred();
      let signal!: AbortSignal;
      vi.mocked(apiFetchJSON).mockImplementation((url, options) => {
        if (url === '/api/alerts/config') {
          signal = options!.signal as AbortSignal;
          return old.promise as never;
        }
        return Promise.resolve(payloads[url]) as never;
      });
      const { result, cleanup } = mountPolicy();
      await settle();
      if (operation === 'unmount') cleanup();
      else {
        vi.mocked(apiFetchJSON).mockImplementation(async (url) => payloads[url]);
        await result.loadAlertConfiguration();
        expect(result.isConfigLoaded()).toBe(true);
      }
      await settle();
      expect(signal.aborted).toBe(true);
      expect(result.configLoadError()).toBeNull();
      expect(logger.error).not.toHaveBeenCalled();
      expect(vi.getTimerCount()).toBe(0);
      old.reject(new Error('late retired policy'));
      await settle();
      expect(logger.error).not.toHaveBeenCalled();
      if (operation !== 'unmount') cleanup();
      const calls = vi.mocked(apiFetchJSON).mock.calls.length;
      await result.loadAlertConfiguration();
      expect(apiFetchJSON).toHaveBeenCalledTimes(calls);
    },
  );

  it('does not request retired Mobile configuration without its existing entitlement', async () => {
    vi.mocked(hasFeature).mockReturnValue(false);
    const { result, cleanup } = renderHook(() =>
      useAlertDestinationsState({ activeTab: () => 'destinations' }),
    );
    try {
      await result.loadDestinations();
      expect(apiFetchJSON).toHaveBeenCalledTimes(4);
      expect(
        vi.mocked(apiFetchJSON).mock.calls.some(([path]) => path === '/api/settings/relay'),
      ).toBe(false);
    } finally {
      cleanup();
    }
  });
});
