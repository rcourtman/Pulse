import { describe, expect, it } from 'vitest';

import {
  clearSharedInfrastructureOnboardingMetricsTracker,
  createInfrastructureOnboardingMetricsTracker,
  getSharedInfrastructureOnboardingMetricsTracker,
} from '@/utils/infrastructureOnboardingMetrics';
import infrastructureOnboardingMetricsSource from '@/utils/infrastructureOnboardingMetrics.ts?raw';

describe('infrastructureOnboardingMetrics', () => {
  it('does not bridge infrastructure onboarding to maintainer analytics', () => {
    expect(infrastructureOnboardingMetricsSource).not.toContain('trackUpgradeMetricEvent');
    expect(infrastructureOnboardingMetricsSource).not.toContain('UPGRADE_METRIC_EVENTS');
    expect(infrastructureOnboardingMetricsSource).not.toContain('/api/upgrade-metrics/events');
    expect(infrastructureOnboardingMetricsSource).not.toContain('sessionStorage');
  });

  it('keeps the tracker contract callable as a compatibility no-op', () => {
    const tracker = createInfrastructureOnboardingMetricsTracker();

    tracker.recordOpened();
    tracker.recordOpened();
    tracker.recordPathSelected('api');
    tracker.recordPathSelected('api');
    tracker.recordProbeResult('no-match');
    tracker.recordProbeResult('error');
    tracker.recordCatalogSelected('truenas');
    tracker.recordCatalogSelected('truenas');
    tracker.recordCredentialsOpened('truenas');
    tracker.recordCredentialsOpened('truenas');
  });

  it('returns the same no-op tracker for created and shared flows', () => {
    const firstTracker = createInfrastructureOnboardingMetricsTracker();
    const secondTracker = getSharedInfrastructureOnboardingMetricsTracker();

    expect(firstTracker).toBe(secondTracker);

    clearSharedInfrastructureOnboardingMetricsTracker();
    const thirdTracker = getSharedInfrastructureOnboardingMetricsTracker();

    expect(thirdTracker).toBe(firstTracker);
  });
});
