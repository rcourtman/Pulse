import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { eventBus } from '@/stores/events';
import { useAlertsActivation } from '@/stores/alertsActivation';
import type { AlertConfig } from '@/types/alerts';
import { ALERTS_DETECTION_EVENT } from '@/utils/alertsActivation';

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
});
