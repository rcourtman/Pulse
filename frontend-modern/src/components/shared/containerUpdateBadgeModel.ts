import type { DockerContainerUpdateStatus } from '@/types/api';

export interface ContainerUpdateBadgeProps {
  updateStatus?: DockerContainerUpdateStatus;
  compact?: boolean;
}

export interface UpdateIconProps {
  updateStatus?: DockerContainerUpdateStatus;
}

export interface UpdateButtonProps {
  updateStatus?: DockerContainerUpdateStatus;
  agentId: string;
  containerId: string;
  containerName: string;
  compact?: boolean;
  onUpdateTriggered?: () => void;
  externalState?: 'updating' | 'queued' | 'error';
}

export type UpdateState = 'idle' | 'confirming' | 'updating' | 'success' | 'error';

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

export function getContainerUpdateErrorTooltip(
  updateStatus?: DockerContainerUpdateStatus,
): string {
  return `Update check failed: ${updateStatus?.error || 'Unknown error'}`;
}

export function getContainerUpdateBadgeTooltip(
  updateStatus?: DockerContainerUpdateStatus,
): string {
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

export function getUpdateButtonClass(state: UpdateState): string {
  switch (state) {
    case 'confirming':
      return `${UPDATE_BUTTON_BASE_CLASS} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300 cursor-pointer hover:bg-amber-200 dark:hover:bg-amber-900`;
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
    case 'confirming':
      return 'Confirm?';
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
    case 'confirming':
      return 'Click again to confirm update';
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
      return `Click to update\nCurrent: ${current}...\nLatest: ${latest}...`;
  }
}
