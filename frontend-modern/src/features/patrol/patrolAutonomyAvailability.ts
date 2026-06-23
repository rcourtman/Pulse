import type { LicenseRuntimeCapabilityBlock, LicenseRuntimeIdentity } from '@/api/license';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';

export const PATROL_AUTONOMY_FEATURE_KEY = 'ai_autofix';
export const PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON = 'paid_runtime_required';

export type PatrolAutonomyAvailabilityKind = 'available' | 'plan_locked' | 'runtime_locked';

export interface PatrolAutonomyAvailabilityInput {
  autoFixLocked: boolean;
  upgradePromptsHidden?: boolean;
  commercialSurfacesHidden?: boolean;
  runtimeCapabilityBlock?: LicenseRuntimeCapabilityBlock;
  runtime?: LicenseRuntimeIdentity;
  planUpgradeDestination: UpgradeDestination;
}

export interface PatrolAutonomyAvailabilityPresentation {
  kind: PatrolAutonomyAvailabilityKind;
  locked: boolean;
  title: string;
  body: string;
  actionLabel?: string;
  destination?: UpgradeDestination;
}

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

export function getPatrolAutonomyAvailabilityPresentation(
  input: PatrolAutonomyAvailabilityInput,
): PatrolAutonomyAvailabilityPresentation {
  if (!input.autoFixLocked) {
    return {
      kind: 'available',
      locked: false,
      title: 'Patrol mode available',
      body: 'Choose the mode for this install.',
    };
  }

  if (input.runtimeCapabilityBlock?.reason === PATROL_AUTONOMY_RUNTIME_REQUIRED_REASON) {
    return {
      kind: 'runtime_locked',
      locked: true,
      title: 'Pulse Pro runtime required',
      body: `This install is running ${getRuntimeLabel(input.runtime)}. Install the Pulse Pro runtime to use Patrol modes.`,
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

  if (input.commercialSurfacesHidden) {
    return {
      kind: 'plan_locked',
      locked: true,
      title: 'Watch only',
      body: 'This install watches infrastructure and shows issues.',
    };
  }

  return {
    kind: 'plan_locked',
    locked: true,
    title: 'Watch only',
    body: 'This install watches infrastructure and shows issues.',
    ...(input.upgradePromptsHidden
      ? {}
      : {
          actionLabel: 'Plans & Billing',
          destination: input.planUpgradeDestination,
        }),
  };
}
