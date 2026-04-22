import type { ConnectionType } from '@/api/connections';
import { trackUpgradeMetricEvent, UPGRADE_METRIC_EVENTS } from '@/utils/upgradeMetrics';

export type InfrastructureOnboardingPath = 'api' | 'agent';
export type InfrastructureOnboardingProbeOutcome = 'detected' | 'no-match' | 'error';

export const INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE = 'settings_infrastructure_add';

let onboardingFlowCounter = 0;

function createOnboardingFlowId(): string {
  const uuid = globalThis.crypto?.randomUUID?.();
  if (uuid) {
    return `infra-onboarding:${uuid}`;
  }

  onboardingFlowCounter += 1;
  return `infra-onboarding:${Date.now().toString(36)}:${onboardingFlowCounter.toString(36)}`;
}

function buildFlowScopedKey(flowId: string, scope: string): string {
  return `${flowId}:${scope}`;
}

export interface InfrastructureOnboardingMetricsTracker {
  recordOpened: () => void;
  recordPathSelected: (path: InfrastructureOnboardingPath) => void;
  recordProbeResult: (outcome: InfrastructureOnboardingProbeOutcome) => void;
  recordCatalogSelected: (type: ConnectionType) => void;
  recordCredentialsOpened: (type: ConnectionType) => void;
}

export function createInfrastructureOnboardingMetricsTracker(
  surface = INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE,
): InfrastructureOnboardingMetricsTracker {
  const flowId = createOnboardingFlowId();
  const selectedPaths = new Set<InfrastructureOnboardingPath>();
  const selectedCatalogTypes = new Set<ConnectionType>();
  const openedCredentialTypes = new Set<ConnectionType>();
  let opened = false;
  let probeAttempt = 0;

  return {
    recordOpened() {
      if (opened) return;
      opened = true;
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.INFRASTRUCTURE_ONBOARDING_OPENED,
        surface,
        idempotencyKey: buildFlowScopedKey(flowId, 'opened'),
      });
    },

    recordPathSelected(path) {
      if (selectedPaths.has(path)) return;
      selectedPaths.add(path);
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.INFRASTRUCTURE_ONBOARDING_PATH_SELECTED,
        surface,
        capability: path,
        idempotencyKey: buildFlowScopedKey(flowId, `path:${path}`),
      });
    },

    recordProbeResult(outcome) {
      probeAttempt += 1;
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.INFRASTRUCTURE_ONBOARDING_PROBE_RESULT,
        surface,
        capability: outcome,
        idempotencyKey: buildFlowScopedKey(flowId, `probe:${probeAttempt}:${outcome}`),
      });
    },

    recordCatalogSelected(type) {
      if (selectedCatalogTypes.has(type)) return;
      selectedCatalogTypes.add(type);
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.INFRASTRUCTURE_ONBOARDING_CATALOG_SELECTED,
        surface,
        capability: type,
        idempotencyKey: buildFlowScopedKey(flowId, `catalog:${type}`),
      });
    },

    recordCredentialsOpened(type) {
      if (openedCredentialTypes.has(type)) return;
      openedCredentialTypes.add(type);
      trackUpgradeMetricEvent({
        type: UPGRADE_METRIC_EVENTS.INFRASTRUCTURE_ONBOARDING_CREDENTIALS_OPENED,
        surface,
        capability: type,
        idempotencyKey: buildFlowScopedKey(flowId, `credentials:${type}`),
      });
    },
  };
}
