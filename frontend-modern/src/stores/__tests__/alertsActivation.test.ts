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
});
