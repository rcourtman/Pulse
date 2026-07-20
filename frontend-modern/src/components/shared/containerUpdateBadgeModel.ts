import type { DockerContainerUpdateStatus } from '@/types/api';
import type { ResourceActionReadiness } from '@/types/resource';

export interface ContainerUpdateBadgeProps {
  updateStatus?: DockerContainerUpdateStatus;
  compact?: boolean;
  showCurrent?: boolean;
}

export interface UpdateIconProps {
  updateStatus?: DockerContainerUpdateStatus;
}

export interface UpdateButtonProps {
  updateStatus?: DockerContainerUpdateStatus;
  agentId: string;
  containerId: string;
  containerName: string;
  // Unified resource id used to plan the audited update action. Without it
  // the button can only report that the update action is unavailable.
  resourceId?: string;
  // Server-evaluated readiness from the unified resource. When it carries an
  // unavailable 'update' entry the button renders disabled with the refusal
  // reason, instead of letting the click fail at POST /api/actions/plan.
  actionReadiness?: ResourceActionReadiness[];
  compact?: boolean;
  onUpdateTriggered?: () => void;
  externalState?: 'updating' | 'queued' | 'error';
}

// The update click plans a governed action and opens the review dialog, which
// is the confirmation surface; there is no separate in-row confirming state.
export type UpdateState = 'idle' | 'updating' | 'success' | 'error';

export interface ContainerUpdateButtonStoreState {
  startedAt: number;
  message?: string;
}

const UPDATE_BUTTON_BASE_CLASS =
  'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium transition-all';

function getDigestPreview(digest: string | undefined, length: number): string {
  return digest?.slice(0, length) || 'unknown';
}

export function hasContainerUpdate(updateStatus?: DockerContainerUpdateStatus): boolean {
  return updateStatus?.updateAvailable === true;
}

export function hasContainerUpdateError(updateStatus?: DockerContainerUpdateStatus): boolean {
  return Boolean(updateStatus?.error);
}

export function hasContainerUpdateCurrent(updateStatus?: DockerContainerUpdateStatus): boolean {
  return updateStatus?.updateAvailable === false && !hasContainerUpdateError(updateStatus);
}

export function getContainerUpdateErrorTooltip(updateStatus?: DockerContainerUpdateStatus): string {
  return `Update check failed: ${updateStatus?.error || 'Unknown error'}`;
}

export function getContainerUpdateCurrentTooltip(
  updateStatus?: DockerContainerUpdateStatus,
): string {
  if (!updateStatus?.currentDigest) return 'Image is current';

  const current = getDigestPreview(updateStatus.currentDigest, 12);
  return `Image is current\nDigest: ${current}...`;
}

export function getContainerUpdateBadgeTooltip(updateStatus?: DockerContainerUpdateStatus): string {
  const current = getDigestPreview(updateStatus?.currentDigest, 19);
  const latest = getDigestPreview(updateStatus?.latestDigest, 19);
  return `Image update available\nCurrent: ${current}...\nLatest: ${latest}...`;
}

export function getUpdateIconTooltip(updateStatus?: DockerContainerUpdateStatus): string {
  if (!updateStatus) return 'Image update available';

  const current = getDigestPreview(updateStatus.currentDigest, 12);
  const latest = getDigestPreview(updateStatus.latestDigest, 12);
  return `Update available\nCurrent: ${current}...\nLatest: ${latest}...`;
}

export function getUpdateButtonClass(state: UpdateState, unavailable = false): string {
  if (unavailable && state === 'idle') {
    return `${UPDATE_BUTTON_BASE_CLASS} bg-surface-alt text-muted cursor-not-allowed opacity-70`;
  }
  switch (state) {
    case 'updating':
      return `${UPDATE_BUTTON_BASE_CLASS} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-wait`;
    case 'success':
      return `${UPDATE_BUTTON_BASE_CLASS} bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300`;
    case 'error':
      return `${UPDATE_BUTTON_BASE_CLASS} bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300 cursor-help`;
    default:
      return `${UPDATE_BUTTON_BASE_CLASS} bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 cursor-pointer hover:bg-blue-200 dark:hover:bg-blue-900`;
  }
}

export function getUpdateButtonLabel(state: UpdateState, settingsLoaded: boolean): string {
  if (!settingsLoaded) return 'Update';

  switch (state) {
    case 'updating':
      return 'Updating...';
    case 'success':
      return 'Queued!';
    case 'error':
      return 'Failed';
    default:
      return 'Update';
  }
}

export function getUpdateButtonTooltip(options: {
  state: UpdateState;
  updateStatus?: DockerContainerUpdateStatus;
  storeState?: ContainerUpdateButtonStoreState;
  errorMessage?: string;
  now?: number;
}): string {
  const now = options.now ?? Date.now();

  switch (options.state) {
    case 'updating': {
      const elapsed = options.storeState
        ? Math.round((now - options.storeState.startedAt) / 1000)
        : 0;
      const step = options.storeState?.message || 'Processing...';
      if (elapsed > 60) {
        return `${step} (${Math.floor(elapsed / 60)}m ${elapsed % 60}s)`;
      }
      return `${step} (${elapsed}s)`;
    }
    case 'success':
      return '✓ Update completed successfully!';
    case 'error':
      return `✗ Update failed: ${options.storeState?.message || options.errorMessage || 'Unknown error'}`;
    default:
      if (!options.updateStatus) return 'Update container';

      const current = getDigestPreview(options.updateStatus.currentDigest, 12);
      const latest = getDigestPreview(options.updateStatus.latestDigest, 12);
      return `Click to review and update\nCurrent: ${current}...\nLatest: ${latest}...`;
  }
}

/**
 * Pick the user-facing message for a failed update plan request.
 *
 * Availability refusals carry the actionable explanation (agent disconnected,
 * agent too old, stale inventory) in the API error's details.reason; the
 * top-level message is just "Action execution is unavailable".
 */
export function getUpdatePlanErrorMessage(error: unknown): string {
  const apiError = error as (Error & { details?: Record<string, string> }) | null;
  return (
    apiError?.details?.reason?.trim() || apiError?.message?.trim() || 'Failed to plan the update'
  );
}
