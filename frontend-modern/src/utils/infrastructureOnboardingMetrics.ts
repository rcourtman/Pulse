import type { ConnectionType } from '@/api/connections';
import { trackUpgradeMetricEvent, UPGRADE_METRIC_EVENTS } from '@/utils/upgradeMetrics';

export type InfrastructureOnboardingPath = 'api' | 'agent';
export type InfrastructureOnboardingProbeOutcome = 'detected' | 'no-match' | 'error';

export const INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE = 'settings_infrastructure_add';

let onboardingFlowCounter = 0;
const sharedTrackers = new Map<string, InfrastructureOnboardingMetricsTracker>();
const SHARED_FLOW_STORAGE_KEY_PREFIX = 'pulse.infrastructure-onboarding.flow';

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
  flowId = createOnboardingFlowId(),
): InfrastructureOnboardingMetricsTracker {
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

function sharedFlowStorageKey(surface: string): string {
  return `${SHARED_FLOW_STORAGE_KEY_PREFIX}:${surface}`;
}

function readSharedFlowId(surface: string): string | null {
  try {
    return globalThis.sessionStorage?.getItem(sharedFlowStorageKey(surface)) ?? null;
  } catch {
    return null;
  }
}

function writeSharedFlowId(surface: string, flowId: string): void {
  try {
    globalThis.sessionStorage?.setItem(sharedFlowStorageKey(surface), flowId);
  } catch {
    // Session storage persistence is best-effort only.
  }
}

export function getSharedInfrastructureOnboardingMetricsTracker(
  surface = INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE,
): InfrastructureOnboardingMetricsTracker {
  const tracker = sharedTrackers.get(surface);
  if (tracker) return tracker;

  const flowId = readSharedFlowId(surface) ?? createOnboardingFlowId();
  writeSharedFlowId(surface, flowId);
  const nextTracker = createInfrastructureOnboardingMetricsTracker(surface, flowId);
  sharedTrackers.set(surface, nextTracker);
  return nextTracker;
}

export function clearSharedInfrastructureOnboardingMetricsTracker(
  surface = INFRASTRUCTURE_ONBOARDING_METRICS_SURFACE,
): void {
  sharedTrackers.delete(surface);
  try {
    globalThis.sessionStorage?.removeItem(sharedFlowStorageKey(surface));
  } catch {
    // Session storage persistence is best-effort only.
  }
}
