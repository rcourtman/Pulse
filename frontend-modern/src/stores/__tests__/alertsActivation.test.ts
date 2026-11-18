import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { AlertConfig } from '@/types/alerts';
import { ALERTS_DETECTION_EVENT } from '@/utils/alertsActivation';
import { getOrgID, setOrgID } from '@/utils/apiClient';

const alertConfig = (
  activationState: AlertConfig['activationState'],
  enabled = true,
): AlertConfig =>
  ({
    enabled,
    activationState,
  }) as AlertConfig;

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
}

function switchWorkspace(orgID: string) {
  setOrgID(orgID);
  eventBus.emit('org_switched', orgID);
}

describe('alerts activation ownership boundary', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.__pulseAlertsDetectionEnabled = null;
    eventBus.emit('org_switched', 'test-org');
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    setOrgID(null);
  });

  it('keeps detection enabled when external notifications are pending review', async () => {
    vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue(alertConfig('pending_review'));

    const store = useAlertsActivation();
    await store.refreshConfig();

    expect(store.activationState()).toBe('pending_review');
    expect(store.detectionEnabled()).toBe(true);
    expect(store.notificationDeliveryEnabled()).toBe(false);
    expect(window.__pulseAlertsDetectionEnabled).toBe(true);
  });

  it('pauses notification delivery without dispatching a detection-state change', async () => {
    vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue(alertConfig('active'));
    vi.spyOn(AlertsAPI, 'updateConfig').mockResolvedValue({ success: true });
    const detectionEvent = vi.fn();
    window.addEventListener(ALERTS_DETECTION_EVENT, detectionEvent);

    try {
      const store = useAlertsActivation();
      await store.refreshConfig();
      detectionEvent.mockClear();

      expect(await store.deactivate()).toBe(true);
      expect(store.activationState()).toBe('pending_review');
      expect(store.detectionEnabled()).toBe(true);
      expect(store.notificationDeliveryEnabled()).toBe(false);
      expect(detectionEvent).not.toHaveBeenCalled();
    } finally {
      window.removeEventListener(ALERTS_DETECTION_EVENT, detectionEvent);
    }
  });

  it('propagates the actual global detection switch independently of notification state', async () => {
    vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue(alertConfig('active', false));

    const store = useAlertsActivation();
    await store.refreshConfig();

    expect(store.detectionEnabled()).toBe(false);
    expect(store.notificationDeliveryEnabled()).toBe(false);
    expect(window.__pulseAlertsDetectionEnabled).toBe(false);
  });
  // An Alerts page save re-reads this config so backup shields and metric
  // colours follow a saved limit; that read must not land in another
  // organisation, nor let an older read overwrite a newer one.
  it('drops a config read that an organisation switch has overtaken', async () => {
    let resolveRead: (config: AlertConfig) => void = () => undefined;
    vi.spyOn(AlertsAPI, 'getConfig').mockReturnValue(
      new Promise<AlertConfig>((resolve) => {
        resolveRead = resolve;
      }),
    );
    const store = useAlertsActivation();
    const read = store.refreshConfig();

    eventBus.emit('org_switched', 'other-org');
    resolveRead(alertConfig('active'));
    await read;

    expect(store.config()).toBeNull();
    expect(store.activationState()).toBeNull();
  });

  it('keeps the newest config read when an older one answers last', async () => {
    const reads: Array<(config: AlertConfig) => void> = [];
    vi.spyOn(AlertsAPI, 'getConfig').mockImplementation(
      () =>
        new Promise<AlertConfig>((resolve) => {
          reads.push(resolve);
        }),
    );
    const store = useAlertsActivation();
    const older = store.refreshConfig();
    const newer = store.refreshConfig();

    reads[1]?.(alertConfig('snoozed'));
    await newer;
    reads[0]?.(alertConfig('active'));
    await older;

    expect(store.activationState()).toBe('snoozed');
    expect(store.isLoading()).toBe(false);
  });

  it.each([
    { name: 'delayed config', response: () => Response.json(alertConfig('active')) },
    {
      name: 'retired workspace response',
      response: () => Response.json({ error: 'invalid_org' }, { status: 400 }),
    },
  ])('does not let a $name change the destination state or selection', async ({ response }) => {
    let finishOutgoing!: (value: Response) => void;
    const outgoing = new Promise<Response>((resolve) => (finishOutgoing = resolve));
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockReturnValueOnce(outgoing)
      .mockResolvedValueOnce(Response.json(alertConfig('active', false)));
    vi.stubGlobal('fetch', fetchMock);
    setOrgID('outgoing');
    const store = useAlertsActivation();
    const previous = store.refreshConfig();

    setOrgID('destination');
    eventBus.emit('org_switched', 'destination');
    expect(store.config()).toBeNull();
    expect(fetchMock.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
    await store.refreshConfig();
    finishOutgoing(response());
    await previous;

    expect(
      fetchMock.mock.calls.map(([url, init]) => [
        url,
        init?.method ?? 'GET',
        new Headers(init?.headers).get('X-Pulse-Org-ID'),
        init?.cache,
      ]),
    ).toEqual([
      ['/api/alerts/config', 'GET', 'outgoing', 'no-store'],
      ['/api/alerts/config', 'GET', 'destination', 'no-store'],
    ]);
    expect(getOrgID()).toBe('destination');
    expect(store.config()).toEqual(alertConfig('active', false));
    expect(store.detectionEnabled()).toBe(false);
    expect(store.notificationDeliveryEnabled()).toBe(false);
    expect(window.__pulseAlertsDetectionEnabled).toBe(false);
    expect(store.isLoading()).toBe(false);
    expect(store.lastError()).toBeNull();
  });

  it('reads alert policy without creating a workspace binding on unbound startup', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(Response.json(alertConfig('active')));
    vi.stubGlobal('fetch', fetchMock);
    setOrgID(null);

    await useAlertsActivation().refreshConfig();

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(new Headers(fetchMock.mock.calls[0]?.[1]?.headers).has('X-Pulse-Org-ID')).toBe(false);
    expect(getOrgID()).toBeNull();
  });

  it('does not install a retired delivery save over the destination policy', async () => {
    switchWorkspace('west');
    const west = alertConfig('active');
    const east = alertConfig('snoozed', false);
    vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValueOnce(west).mockResolvedValueOnce(east);
    const pending = deferred<{ success: boolean }>();
    const save = vi.spyOn(AlertsAPI, 'updateConfig').mockReturnValue(pending.promise);
    const store = useAlertsActivation();
    await store.refreshConfig();

    const outgoing = store.deactivate();
    expect(store.isLoading()).toBe(true);
    expect(save).toHaveBeenCalledWith(
      { ...west, activationState: 'pending_review' },
      { expectedOrgID: 'west', retry: false, signal: expect.any(AbortSignal) },
    );
    const signal = save.mock.calls[0]?.[1]?.signal;
    switchWorkspace('east');
    expect(signal?.aborted).toBe(true);
    expect(store.isLoading()).toBe(false);
    await store.refreshConfig();

    // Even a response which was already queued before abort must be ignored.
    pending.resolve({ success: true });
    expect(await outgoing).toBe(false);
    expect(store.config()).toBe(east);
    expect(store.detectionEnabled()).toBe(false);
    expect(store.activationState()).toBe('snoozed');
    expect(store.notificationDeliveryEnabled()).toBe(false);
    expect(window.__pulseAlertsDetectionEnabled).toBe(false);
    expect(store.lastError()).toBeNull();
    expect(save).toHaveBeenCalledTimes(1);
  });

  it('does not report a retired write error or clear a destination read in progress', async () => {
    switchWorkspace('west');
    const east = deferred<AlertConfig>();
    vi.spyOn(AlertsAPI, 'getConfig')
      .mockResolvedValueOnce(alertConfig('active'))
      .mockReturnValueOnce(east.promise);
    const pending = deferred<{ success: boolean }>();
    vi.spyOn(AlertsAPI, 'updateConfig').mockReturnValue(pending.promise);
    const store = useAlertsActivation();
    await store.refreshConfig();

    const outgoing = store.snooze();
    switchWorkspace('east');
    const destination = store.refreshConfig();
    pending.reject(new Error('outgoing save failed'));
    expect(await outgoing).toBe(false);
    expect(store.lastError()).toBeNull();
    expect(store.isLoading()).toBe(true);
    east.resolve(alertConfig('active'));
    await destination;
    expect(store.isLoading()).toBe(false);
    expect(store.notificationDeliveryEnabled()).toBe(true);
  });

  it('does not issue an unsent confirmation with the new workspace config', async () => {
    switchWorkspace('west');
    const westRead = deferred<AlertConfig>();
    const east = alertConfig('active', false);
    vi.spyOn(AlertsAPI, 'getConfig')
      .mockReturnValueOnce(westRead.promise)
      .mockResolvedValueOnce(east);
    const save = vi.spyOn(AlertsAPI, 'updateConfig');
    const store = useAlertsActivation();

    const confirmation = store.deactivate();
    switchWorkspace('east');
    await store.refreshConfig();
    westRead.resolve(alertConfig('active'));

    expect(await confirmation).toBe(false);
    expect(store.config()).toBe(east);
    expect(save).not.toHaveBeenCalled();
    expect(store.lastError()).toBeNull();
  });

  it('does not refresh from an old activation or clear a newer write after switching back', async () => {
    switchWorkspace('west');
    const west = alertConfig('active');
    const read = vi.spyOn(AlertsAPI, 'getConfig').mockResolvedValue(west);
    const activation = deferred<Awaited<ReturnType<typeof AlertsAPI.activate>>>();
    const activate = vi.spyOn(AlertsAPI, 'activate').mockReturnValue(activation.promise);
    const nextSave = deferred<{ success: boolean }>();
    const update = vi.spyOn(AlertsAPI, 'updateConfig').mockReturnValue(nextSave.promise);
    const store = useAlertsActivation();

    const outgoing = store.activate();
    expect(activate).toHaveBeenCalledWith({
      expectedOrgID: 'west',
      retry: false,
      signal: expect.any(AbortSignal),
    });
    switchWorkspace('east');
    switchWorkspace('west');
    await store.refreshConfig();
    const current = store.deactivate();
    // The header and resume card share the write owner.
    expect(await store.activate()).toBe(false);
    expect(activate).toHaveBeenCalledTimes(1);
    activation.resolve({ success: true, state: 'active' });
    expect(await outgoing).toBe(false);
    expect(read).toHaveBeenCalledTimes(1);
    expect(store.isWritingActivation()).toBe(true);
    expect(store.isLoading()).toBe(true);
    expect(store.config()).toBe(west);

    nextSave.resolve({ success: true });
    expect(await current).toBe(true);
    expect(update).toHaveBeenCalledTimes(1);
    expect(store.isLoading()).toBe(false);
    expect(store.notificationDeliveryEnabled()).toBe(false);
  });

  it.each([
    ['west', 400, 'invalid_org'],
    [null, 403, 'csrf_failure'],
  ] as const)(
    'does not replay a rejected confirmation in workspace %s',
    async (org, status, code) => {
      setOrgID(org);
      const transport = vi
        .fn<typeof fetch>()
        .mockResolvedValueOnce(Response.json(alertConfig('active')))
        .mockResolvedValueOnce(
          Response.json(
            { error: code },
            { status, headers: { 'X-CSRF-Token': 'fixture-refresh' } },
          ),
        );
      vi.stubGlobal('fetch', transport);
      const store = useAlertsActivation();
      await store.refreshConfig();

      expect(await store.deactivate()).toBe(false);
      expect(getOrgID()).toBe(org);
      expect(transport).toHaveBeenCalledTimes(2);
      const [route, put] = transport.mock.calls[1]!;
      expect(route).toBe('/api/alerts/config');
      expect(put?.method).toBe('PUT');
      expect(new Headers(put?.headers).get('X-Pulse-Org-ID')).toBe(org);
      expect(store.lastError()).not.toBeNull();
      expect(store.notificationDeliveryEnabled()).toBe(true);
      expect(store.isLoading()).toBe(false);
    },
  );
});
