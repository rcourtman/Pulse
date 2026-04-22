import { beforeEach, describe, expect, it, vi } from 'vitest';

const { trackUpgradeMetricEventMock } = vi.hoisted(() => ({
  trackUpgradeMetricEventMock: vi.fn(),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackUpgradeMetricEvent: trackUpgradeMetricEventMock,
  UPGRADE_METRIC_EVENTS: {
    INFRASTRUCTURE_ONBOARDING_OPENED: 'infrastructure_onboarding_opened',
    INFRASTRUCTURE_ONBOARDING_PATH_SELECTED: 'infrastructure_onboarding_path_selected',
    INFRASTRUCTURE_ONBOARDING_PROBE_RESULT: 'infrastructure_onboarding_probe_result',
    INFRASTRUCTURE_ONBOARDING_CATALOG_SELECTED: 'infrastructure_onboarding_catalog_selected',
    INFRASTRUCTURE_ONBOARDING_CREDENTIALS_OPENED: 'infrastructure_onboarding_credentials_opened',
  },
}));

import { createInfrastructureOnboardingMetricsTracker } from '@/utils/infrastructureOnboardingMetrics';

describe('infrastructureOnboardingMetrics', () => {
  beforeEach(() => {
    trackUpgradeMetricEventMock.mockClear();
  });

  it('deduplicates flow-scoped onboarding steps inside one flow', () => {
    const tracker = createInfrastructureOnboardingMetricsTracker();

    tracker.recordOpened();
    tracker.recordOpened();
    tracker.recordPathSelected('api');
    tracker.recordPathSelected('api');
    tracker.recordCatalogSelected('truenas');
    tracker.recordCatalogSelected('truenas');
    tracker.recordCredentialsOpened('truenas');
    tracker.recordCredentialsOpened('truenas');

    expect(trackUpgradeMetricEventMock).toHaveBeenCalledTimes(4);
    expect(trackUpgradeMetricEventMock).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        type: 'infrastructure_onboarding_opened',
        surface: 'settings_infrastructure_add',
      }),
    );
    expect(trackUpgradeMetricEventMock).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        type: 'infrastructure_onboarding_path_selected',
        capability: 'api',
      }),
    );
    expect(trackUpgradeMetricEventMock).toHaveBeenNthCalledWith(
      3,
      expect.objectContaining({
        type: 'infrastructure_onboarding_catalog_selected',
        capability: 'truenas',
      }),
    );
    expect(trackUpgradeMetricEventMock).toHaveBeenNthCalledWith(
      4,
      expect.objectContaining({
        type: 'infrastructure_onboarding_credentials_opened',
        capability: 'truenas',
      }),
    );
  });

  it('records each probe attempt separately and isolates flow ids between trackers', () => {
    const firstTracker = createInfrastructureOnboardingMetricsTracker();
    const secondTracker = createInfrastructureOnboardingMetricsTracker();

    firstTracker.recordProbeResult('no-match');
    firstTracker.recordProbeResult('no-match');
    secondTracker.recordOpened();

    expect(trackUpgradeMetricEventMock).toHaveBeenCalledTimes(3);

    const firstProbe = trackUpgradeMetricEventMock.mock.calls[0][0];
    const secondProbe = trackUpgradeMetricEventMock.mock.calls[1][0];
    const secondFlowOpen = trackUpgradeMetricEventMock.mock.calls[2][0];

    expect(firstProbe.idempotencyKey).not.toBe(secondProbe.idempotencyKey);
    expect(firstProbe.idempotencyKey).not.toBe(secondFlowOpen.idempotencyKey);
    expect(firstProbe.capability).toBe('no-match');
    expect(secondProbe.capability).toBe('no-match');
  });
});
