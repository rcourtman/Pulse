import type { LicenseRuntimeCapabilityBlock, LicenseRuntimeIdentity } from '@/api/license';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';

export const PATROL_AUTONOMY_FEATURE_KEY = 'ai_autofix';
export const PATROL_ALERT_ANALYSIS_FEATURE_KEY = 'ai_alerts';
export const PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON = 'paid_runtime_required';

export type PatrolFeatureAvailabilityKind = 'available' | 'plan_locked' | 'runtime_locked';

// Presentation policy shared by every plan-gated Patrol capability. Each
// capability supplies only its lock state and its own copy; the precedence
// (hidden commercial surfaces, then runtime lock, then plan lock) and the
// action labels stay canonical here. Hidden commercial surfaces win over the
// runtime lock so demo and white-label sessions never see Pro wording.
interface PatrolFeatureAvailabilityPolicy {
  upgradePromptsHidden?: boolean;
  commercialSurfacesHidden?: boolean;
  runtimeCapabilityBlock?: LicenseRuntimeCapabilityBlock;
  runtime?: LicenseRuntimeIdentity;
  planUpgradeDestination: UpgradeDestination;
}

export interface PatrolAutonomyAvailabilityInput extends PatrolFeatureAvailabilityPolicy {
  autoFixLocked: boolean;
}

export interface PatrolAlertAnalysisAvailabilityInput extends PatrolFeatureAvailabilityPolicy {
  alertAnalysisLocked: boolean;
}

export interface PatrolFeatureAvailabilityPresentation {
  kind: PatrolFeatureAvailabilityKind;
  locked: boolean;
  title: string;
  body: string;
  actionLabel?: string;
  destination?: UpgradeDestination;
}

interface PatrolFeatureAvailabilityCopy {
  available: { title: string; body: string };
  planLocked: { title: string; body: string };
  // Plan-locked copy for sessions that hide commercial surfaces. Defaults to
  // planLocked, so it is only needed when that copy mentions plans.
  planLockedCommercialHidden?: { title: string; body: string };
  // Completes "Install the Pulse Pro runtime to use ...".
  runtimeFeatureLabel: string;
}

const PATROL_AUTONOMY_COPY: PatrolFeatureAvailabilityCopy = {
  available: {
    title: 'Patrol mode available',
    body: 'Choose the mode for this install.',
  },
  planLocked: {
    title: 'Watch only',
    body: 'This install watches infrastructure and shows issues.',
  },
  runtimeFeatureLabel: 'Patrol modes',
};

const PATROL_ALERT_ANALYSIS_COPY: PatrolFeatureAvailabilityCopy = {
  available: {
    title: 'Container update risk available',
    body: 'Assess risk when container-update alerts fire.',
  },
  planLocked: {
    title: 'Higher license plan required',
    body: "This install's plan does not include container update risk.",
  },
  planLockedCommercialHidden: {
    title: 'Not available',
    body: 'This install does not include container update risk.',
  },
  runtimeFeatureLabel: 'container update risk',
};

function getRuntimeDownloadDestination(
  block: LicenseRuntimeCapabilityBlock | undefined,
  runtime: LicenseRuntimeIdentity | undefined,
): UpgradeDestination | undefined {
  const href = block?.action_url?.trim() || runtime?.download_url?.trim();
  return href ? resolveUpgradeDestination(href) : undefined;
}

function getRuntimeLabel(runtime: LicenseRuntimeIdentity | undefined): string {
  return runtime?.label?.trim() || 'this runtime';
}

function getPatrolFeatureAvailabilityPresentation(
  locked: boolean,
  input: PatrolFeatureAvailabilityPolicy,
  copy: PatrolFeatureAvailabilityCopy,
): PatrolFeatureAvailabilityPresentation {
  if (!locked) {
    return {
      kind: 'available',
      locked: false,
      ...copy.available,
    };
  }

  if (input.commercialSurfacesHidden) {
    return {
      kind: 'plan_locked',
      locked: true,
      ...(copy.planLockedCommercialHidden ?? copy.planLocked),
    };
  }

  if (input.runtimeCapabilityBlock?.reason === PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON) {
    return {
      kind: 'runtime_locked',
      locked: true,
      title: 'Pulse Pro runtime required',
      body: `This install is running ${getRuntimeLabel(input.runtime)}. Install the Pulse Pro runtime to use ${copy.runtimeFeatureLabel}.`,
      ...(input.upgradePromptsHidden
        ? {}
        : {
            actionLabel: 'Open Pro downloads',
            destination:
              getRuntimeDownloadDestination(input.runtimeCapabilityBlock, input.runtime) ??
              input.planUpgradeDestination,
          }),
    };
  }

  return {
    kind: 'plan_locked',
    locked: true,
    ...copy.planLocked,
    ...(input.upgradePromptsHidden
      ? {}
      : {
          actionLabel: 'Plans & Billing',
          destination: input.planUpgradeDestination,
        }),
  };
}

export function getPatrolAutonomyAvailabilityPresentation(
  input: PatrolAutonomyAvailabilityInput,
): PatrolFeatureAvailabilityPresentation {
  return getPatrolFeatureAvailabilityPresentation(input.autoFixLocked, input, PATROL_AUTONOMY_COPY);
}

export function getPatrolAlertAnalysisAvailabilityPresentation(
  input: PatrolAlertAnalysisAvailabilityInput,
): PatrolFeatureAvailabilityPresentation {
  return getPatrolFeatureAvailabilityPresentation(
    input.alertAnalysisLocked,
    input,
    PATROL_ALERT_ANALYSIS_COPY,
  );
}
