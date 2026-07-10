import { createSignal } from 'solid-js';
import { UpdatesAPI } from '@/api/updates';
import type { UpdateInfo, VersionInfo } from '@/api/updates';
import { notificationStore } from '@/stores/notifications';
import { getPulseHostname } from '@/utils/url';
import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { getStartUpdateErrorMessage } from '@/utils/systemSettingsPresentation';

const CHECK_INTERVAL = 24 * 60 * 60 * 1000; // 24 hours
const TRANSIENT_RETRY_MAX_ATTEMPTS = 3;
const TRANSIENT_RETRY_BASE_DELAY_MS = import.meta.env.MODE === 'test' ? 0 : 400;

// Persisted before POSTing an apply so the boot after the post-update page
// reload can confirm the update landed (in-memory state does not survive it).
interface PendingApply {
  fromVersion: string;
  toVersion: string;
  startedAt: number;
}

interface UpdateState {
  lastCheck: number;
  dismissedVersion?: string;
  updateInfo?: UpdateInfo;
  pendingApply?: PendingApply;
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null;

const isFiniteNumber = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value);

const normalizeUpdateInfo = (value: unknown): UpdateInfo | undefined => {
  if (!isRecord(value)) return undefined;

  const {
    available,
    currentVersion,
    latestVersion,
    releaseNotes,
    releaseDate,
    downloadUrl,
    isPrerelease,
    isMajorUpgrade,
    warning,
  } = value;

  if (
    typeof available !== 'boolean' ||
    typeof currentVersion !== 'string' ||
    typeof latestVersion !== 'string' ||
    typeof releaseNotes !== 'string' ||
    typeof releaseDate !== 'string' ||
    typeof downloadUrl !== 'string' ||
    typeof isPrerelease !== 'boolean'
  ) {
    return undefined;
  }

  if (warning !== undefined && typeof warning !== 'string') {
    return undefined;
  }

  return {
    available,
    currentVersion,
    latestVersion,
    releaseNotes,
    releaseDate,
    downloadUrl,
    isPrerelease,
    isMajorUpgrade: typeof isMajorUpgrade === 'boolean' ? isMajorUpgrade : false,
    ...(warning !== undefined ? { warning } : {}),
  };
};

const normalizePendingApply = (value: unknown): PendingApply | undefined => {
  if (!isRecord(value)) return undefined;

  const { fromVersion, toVersion, startedAt } = value;
  if (
    typeof fromVersion !== 'string' ||
    typeof toVersion !== 'string' ||
    !isFiniteNumber(startedAt)
  ) {
    return undefined;
  }

  return { fromVersion, toVersion, startedAt };
};

const normalizeUpdateState = (value: unknown): UpdateState => {
  if (!isRecord(value)) {
    return { lastCheck: 0 };
  }

  const normalized: UpdateState = {
    lastCheck: isFiniteNumber(value.lastCheck) && value.lastCheck >= 0 ? value.lastCheck : 0,
  };

  if (typeof value.dismissedVersion === 'string') {
    normalized.dismissedVersion = value.dismissedVersion;
  }

  const normalizedUpdateInfo = normalizeUpdateInfo(value.updateInfo);
  if (normalizedUpdateInfo) {
    normalized.updateInfo = normalizedUpdateInfo;
  }

  const normalizedPendingApply = normalizePendingApply(value.pendingApply);
  if (normalizedPendingApply) {
    normalized.pendingApply = normalizedPendingApply;
  }

  return normalized;
};

// Load state from localStorage
const loadState = (): UpdateState => {
  try {
    const stored = localStorage.getItem(STORAGE_KEYS.UPDATES);
    if (stored) {
      return normalizeUpdateState(JSON.parse(stored));
    }
  } catch (e) {
    logger.error('Failed to load update state:', e);
  }
  return { lastCheck: 0 };
};

// Save state to localStorage
const saveState = (state: UpdateState) => {
  try {
    localStorage.setItem(STORAGE_KEYS.UPDATES, JSON.stringify(state));
  } catch (e) {
    logger.error('Failed to save update state:', e);
  }
};

// Create signals
const [updateAvailable, setUpdateAvailable] = createSignal(false);
const [updateInfo, setUpdateInfo] = createSignal<UpdateInfo | null>(null);
const [versionInfo, setVersionInfo] = createSignal<VersionInfo | null>(null);
const [isChecking, setIsChecking] = createSignal(false);
const [isDismissed, setIsDismissed] = createSignal(false);
const [lastError, setLastError] = createSignal<string | null>(null);

const sleep = (ms: number): Promise<void> =>
  new Promise((resolve) => {
    setTimeout(resolve, ms);
  });

const isTransientUpdateCheckError = (error: unknown): boolean => {
  const message = (error instanceof Error ? error.message : String(error)).toLowerCase();
  if (!message) {
    return false;
  }

  if (
    message.includes('failed to fetch') ||
    message.includes('networkerror') ||
    message.includes('timeout')
  ) {
    return true;
  }

  return /\bstatus (408|429|5\d\d)\b/.test(message);
};

export const withTransientRetry = async <T>(
  label: string,
  operation: () => Promise<T>,
): Promise<T> => {
  let attempt = 1;
  for (;;) {
    try {
      return await operation();
    } catch (error) {
      const shouldRetry =
        isTransientUpdateCheckError(error) && attempt < TRANSIENT_RETRY_MAX_ATTEMPTS;
      if (!shouldRetry) {
        throw error;
      }
      const delay = TRANSIENT_RETRY_BASE_DELAY_MS * 2 ** (attempt - 1);
      logger.warn(`Transient update ${label} failure, retrying`, {
        attempt,
        maxAttempts: TRANSIENT_RETRY_MAX_ATTEMPTS,
        delayMs: delay,
      });
      await sleep(delay);
      attempt += 1;
    }
  }
};

const markApplyStarted = (fromVersion: string, toVersion: string) => {
  const state = loadState();
  saveState({ ...state, pendingApply: { fromVersion, toVersion, startedAt: Date.now() } });
};

const clearPendingApply = () => {
  const state = loadState();
  if (!state.pendingApply) return;
  delete state.pendingApply;
  saveState(state);
};

const formatVersionLabel = (version: string) =>
  version.startsWith('v') ? version : `v${version}`;

// One-time post-update confirmation: consume the pending-apply marker on the
// first version fetch after an apply. When the running version moved off the
// recorded pre-update version, the update landed; toast it once. When it did
// not (apply failed or rolled back), clear the marker silently. Mutates the
// caller's state object so later saveState(state) calls in the same check
// cannot resurrect the consumed marker.
const confirmPendingApply = (currentVersion: string, state: UpdateState) => {
  const pending = state.pendingApply;
  if (!pending) return;

  delete state.pendingApply;
  saveState(state);

  if (currentVersion && currentVersion !== pending.fromVersion) {
    notificationStore.success(`Updated to ${formatVersionLabel(currentVersion)}`);
  }
};

// Shared confirm/apply action for both update surfaces (banner and Settings).
// Persists the pre-update version so the post-restart boot can confirm the
// update, and owns the single error-handling path. Returns true when the
// backend accepted the apply request.
const applyUpdate = async (): Promise<boolean> => {
  const info = updateInfo();
  if (!info?.downloadUrl) return false;

  markApplyStarted(versionInfo()?.version || info.currentVersion, info.latestVersion);
  try {
    await UpdatesAPI.applyUpdate(info.downloadUrl);
    return true;
  } catch (error) {
    clearPendingApply();
    logger.error('Failed to start update', error);
    notificationStore.error(getStartUpdateErrorMessage());
    return false;
  }
};

// Check for updates
const checkForUpdates = async (force = false): Promise<void> => {
  // Don't check if already checking
  if (isChecking()) return;

  const state = loadState();
  const now = Date.now();

  // Skip if checked recently (unless forced)
  if (!force && state.lastCheck && now - state.lastCheck < CHECK_INTERVAL) {
    // Use cached data if available
    if (state.updateInfo) {
      // First check if version matches (in case user updated)
      try {
        const currentVersion = await withTransientRetry('version check', () =>
          UpdatesAPI.getVersion(),
        );
        confirmPendingApply(currentVersion.version, state);
        if (state.updateInfo.currentVersion !== currentVersion.version) {
          // Version changed, invalidate cache and check again
          state.updateInfo = undefined;
          state.dismissedVersion = undefined;
          state.lastCheck = 0;
          saveState(state);
          // Continue to check for updates
        } else {
          // Version matches, use cached data
          setVersionInfo(currentVersion);
          setUpdateInfo(state.updateInfo);
          setUpdateAvailable(state.updateInfo.available);
          setLastError(null);

          // Check if this version was dismissed
          setIsDismissed(state.dismissedVersion === state.updateInfo.latestVersion);
          return;
        }
      } catch (e) {
        // If we can't get version, continue with normal check
        logger.error('Failed to verify version for cache:', e);
      }
    } else {
      // Cached payload is missing/corrupted; perform a fresh fetch.
    }
  }

  setIsChecking(true);
  setLastError(null);

  try {
    // First get version info to check deployment type
    const version = await withTransientRetry('version check', () => UpdatesAPI.getVersion());
    setVersionInfo(version);
    confirmPendingApply(version.version, state);

    // Clear cache if version has changed (user updated)
    if (state.updateInfo && state.updateInfo.currentVersion !== version.version) {
      // Version changed, clear the cache
      state.updateInfo = undefined;
      state.dismissedVersion = undefined;
      saveState(state);
    }

    // For development or source builds, skip update checks entirely
    if (version.isDevelopment || version.isSourceBuild) {
      setUpdateAvailable(false);
      setUpdateInfo(null);
      setIsDismissed(false);
      return;
    }
    // For Docker, we still check for available updates so users know a new version exists.
    // The update mechanism is different (docker pull), but the user should see the notification.

    // Skip dev builds unless forcing (contains -dirty or commit hash after version)
    const isDirtyBuild =
      version.version.includes('-dirty') || /v\d+\.\d+\.\d+.*-g[0-9a-f]+/.test(version.version);
    if (isDirtyBuild && !force) {
      setUpdateAvailable(false);
      setUpdateInfo(null);
      setIsDismissed(false);
      return;
    }

    // Get the saved update channel from system settings
    const info = await withTransientRetry('check request', () => UpdatesAPI.checkForUpdates());

    setUpdateInfo(info);
    setUpdateAvailable(info.available);

    // Check if this version was dismissed
    if (state.dismissedVersion === info.latestVersion) {
      setIsDismissed(true);
    } else {
      setIsDismissed(false);
    }

    // Save to cache
    saveState({
      ...state,
      lastCheck: now,
      updateInfo: info,
    });
  } catch (error) {
    logger.error('Failed to check for updates:', error);
    setLastError(error instanceof Error ? error.message : 'Failed to check for updates');
    const fallbackInfo = state.updateInfo ?? updateInfo();
    if (fallbackInfo) {
      logger.warn('Using cached update info after update check failure');
      setUpdateInfo(fallbackInfo);
      setUpdateAvailable(fallbackInfo.available);
      setIsDismissed(state.dismissedVersion === fallbackInfo.latestVersion);
    } else {
      setUpdateAvailable(false);
      setIsDismissed(false);
    }
  } finally {
    setIsChecking(false);
  }
};

// Dismiss current update
const dismissUpdate = () => {
  const info = updateInfo();
  if (!info) return;

  const state = loadState();
  saveState({
    ...state,
    dismissedVersion: info.latestVersion,
  });

  setIsDismissed(true);
};

// Clear dismissed version (useful when user wants to see the update again)
const clearDismissed = () => {
  const state = loadState();
  delete state.dismissedVersion;
  saveState(state);
  setIsDismissed(false);
};

// Check if update is visible (available and not dismissed)
const isUpdateVisible = () => updateAvailable() && !isDismissed();

// Export store
export const updateStore = {
  // State
  updateAvailable,
  updateInfo,
  versionInfo,
  isChecking,
  isDismissed,
  lastError,
  isUpdateVisible,

  // Actions
  checkForUpdates,
  applyUpdate,
  dismissUpdate,
  clearDismissed,

  // Manual testing helpers
  simulateUpdate: (version: string = 'v6.0.0') => {
    setUpdateInfo({
      available: true,
      currentVersion: versionInfo()?.version || 'v6.0.0',
      latestVersion: version,
      releaseNotes: 'Test update notification',
      releaseDate: new Date().toISOString(),
      downloadUrl: '#',
      isPrerelease: false,
      isMajorUpgrade: false,
    });
    setUpdateAvailable(true);
    setIsDismissed(false);
  },
};

// Expose for testing in development
declare global {
  interface Window {
    updateStore?: typeof updateStore;
  }
}

const pulseHostname = getPulseHostname();

if (import.meta.env.DEV || pulseHostname === 'localhost' || pulseHostname.startsWith('192.168')) {
  window.updateStore = updateStore;
}
