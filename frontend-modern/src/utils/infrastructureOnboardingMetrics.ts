import type { ConnectionType } from '@/api/connections';

export type InfrastructureOnboardingPath = 'api' | 'agent';
export type InfrastructureOnboardingProbeOutcome = 'detected' | 'no-match' | 'error';

export const INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE = 'settings_infrastructure_add';

export interface InfrastructureOnboardingMetricsTracker {
  recordOpened: () => void;
  recordPathSelected: (path: InfrastructureOnboardingPath) => void;
  recordProbeResult: (outcome: InfrastructureOnboardingProbeOutcome) => void;
  recordCatalogSelected: (type: ConnectionType) => void;
  recordCredentialsOpened: (type: ConnectionType) => void;
}

const NOOP_INFRASTRUCTURE_ONBOARDING_METRICS_TRACKER: InfrastructureOnboardingMetricsTracker = {
  recordOpened() {
    // Compatibility no-op.
  },
  recordPathSelected(_path) {
    // Compatibility no-op.
  },
  recordProbeResult(_outcome) {
    // Compatibility no-op.
  },
  recordCatalogSelected(_type) {
    // Compatibility no-op.
  },
  recordCredentialsOpened(_type) {
    // Compatibility no-op.
  },
};

export function createInfrastructureOnboardingMetricsTracker(
  _surface = INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE,
  _flowId?: string,
): InfrastructureOnboardingMetricsTracker {
  return NOOP_INFRASTRUCTURE_ONBOARDING_METRICS_TRACKER;
}

export function getSharedInfrastructureOnboardingMetricsTracker(
  _surface = INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE,
): InfrastructureOnboardingMetricsTracker {
  return NOOP_INFRASTRUCTURE_ONBOARDING_METRICS_TRACKER;
}

export function clearSharedInfrastructureOnboardingMetricsTracker(
  _surface = INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE,
): void {
  // Compatibility no-op.
}
