import { createSignal } from 'solid-js';
import { LicenseAPI, type LicenseRuntimeCapabilities } from '@/api/license';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';

const FREE_RUNTIME_CAPABILITIES_FALLBACK: LicenseRuntimeCapabilities = {
  capabilities: ['update_alerts', 'sso', 'ai_patrol'],
  limits: [],
  hosted_mode: false,
  max_history_days: 7,
};

const [runtimeCapabilities, setRuntimeCapabilities] =
  createSignal<LicenseRuntimeCapabilities | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);
const [loadError, setLoadError] = createSignal<Error | null>(null);

/**
 * Load the canonical runtime capability payload from the server.
 */
export async function loadLicenseStatus(force = false): Promise<void> {
  if (loaded() && !force) return;

  setLoading(true);
  try {
    const next = await LicenseAPI.getRuntimeCapabilities();
    setRuntimeCapabilities(next);
    setLoadError(null);
    setLoaded(true);
    logger.debug('[licenseStore] Runtime capabilities loaded', {
      capability_count: next.capabilities.length,
      limit_count: next.limits.length,
      max_history_days: next.max_history_days,
    });
  } catch (err) {
    logger.error('[licenseStore] Failed to load runtime capabilities', err);
    setLoadError(err instanceof Error ? err : new Error(String(err)));
    setRuntimeCapabilities(FREE_RUNTIME_CAPABILITIES_FALLBACK);
    setLoaded(true);
  } finally {
    setLoading(false);
  }
}

/**
 * Check if a specific feature is enabled by the current license/runtime.
 */
export function hasFeature(feature: string): boolean {
  const current = runtimeCapabilities();
  if (!current) return false;
  return current.capabilities.includes(feature);
}

export function isMultiTenantEnabled(): boolean {
  return hasFeature('multi_tenant');
}

export function isHostedModeEnabled(): boolean {
  return Boolean(runtimeCapabilities()?.hosted_mode);
}

export function getLimit(key: string) {
  const current = runtimeCapabilities();
  if (!current?.limits?.length) return undefined;
  return current.limits.find((limit) => limit.key === key);
}

/** Default max history days when runtime capabilities aren't loaded yet. */
const DEFAULT_MAX_HISTORY_DAYS = 7;

function parseRangeDays(range: string): number {
  const match = range.match(/^(\d+)(h|d)$/);
  if (!match) return 0;
  const val = parseInt(match[1], 10);
  return match[2] === 'd' ? val : val / 24;
}

/**
 * Return the runtime max history days from the canonical capability payload.
 */
export function maxHistoryDays(): number {
  return runtimeCapabilities()?.max_history_days ?? DEFAULT_MAX_HISTORY_DAYS;
}

/**
 * Check if a time range exceeds the current runtime history limit.
 */
export function isRangeLocked(range: string): boolean {
  return parseRangeDays(range) > maxHistoryDays();
}

// Ensure org-scoped runtime capabilities do not leak across tenant switches.
eventBus.on('org_switched', () => {
  setRuntimeCapabilities(null);
  setLoaded(false);
  setLoadError(null);
  void loadLicenseStatus(true);
});

export {
  runtimeCapabilities,
  loading as licenseLoading,
  loaded as licenseLoaded,
  loadError as licenseLoadError,
};
