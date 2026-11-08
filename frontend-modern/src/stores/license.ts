import { batch, createSignal } from 'solid-js';
import { LicenseAPI, type LicenseRuntimeCapabilities } from '@/api/license';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';

const FREE_RUNTIME_CAPABILITIES_FALLBACK: LicenseRuntimeCapabilities = {
  capabilities: ['update_alerts', 'sso', 'ai_patrol'],
  limits: [],
  hosted_mode: false,
  max_history_days: 7,
  runtime: {
    build: 'community',
    label: 'Pulse Community runtime',
  },
  blocked_capabilities: [],
};

const [runtimeCapabilities, setRuntimeCapabilities] =
  createSignal<LicenseRuntimeCapabilities | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);
const [loadError, setLoadError] = createSignal<Error | null>(null);

let loadGeneration = 0;
let inFlight: Promise<void> | null = null;

/**
 * Share ordinary reads, but let an explicit refresh or organisation switch
 * supersede them. Arrival order must never recreate withdrawn capabilities or
 * replace a newer accepted runtime with an old response or fallback.
 */
export function loadRuntimeCapabilities(force = false): Promise<void> {
  if (!force && inFlight) return inFlight;
  if (!force && loaded()) return Promise.resolve();

  const generation = ++loadGeneration;
  setLoading(true);
  inFlight = (async () => {
    try {
      const next = await LicenseAPI.getRuntimeCapabilities();
      if (generation !== loadGeneration) return;
      batch(() => {
        setRuntimeCapabilities(next);
        setLoadError(null);
        setLoaded(true);
      });
      logger.debug('[licenseStore] Runtime capabilities loaded', {
        capability_count: next.capabilities.length,
        limit_count: next.limits.length,
        max_history_days: next.max_history_days,
        runtime_build: next.runtime?.build,
      });
    } catch (err) {
      if (generation !== loadGeneration) return;
      logger.error('[licenseStore] Failed to load runtime capabilities', err);
      batch(() => {
        setLoadError(err instanceof Error ? err : new Error(String(err)));
        setRuntimeCapabilities(FREE_RUNTIME_CAPABILITIES_FALLBACK);
        setLoaded(true);
      });
    } finally {
      if (generation === loadGeneration) {
        inFlight = null;
        setLoading(false);
      } else {
        // Awaiting consumers read the singleton after this promise settles.
        // Do not release them while its current replacement is still loading.
        await inFlight;
      }
    }
  })();
  return inFlight;
}

/**
 * Check if a specific feature is enabled by the current license/runtime.
 */
export function hasFeature(feature: string): boolean {
  const current = runtimeCapabilities();
  if (!current) return false;
  return current.capabilities.includes(feature);
}

export function getRuntimeCapabilityBlock(feature: string) {
  const normalized = feature.trim();
  const current = runtimeCapabilities();
  if (!normalized || !current?.blocked_capabilities?.length) return undefined;
  return current.blocked_capabilities.find((block) => block.key === normalized);
}

export function isRuntimeCapabilityBlocked(feature: string, reason?: string): boolean {
  const block = getRuntimeCapabilityBlock(feature);
  if (!block) return false;
  return reason ? block.reason === reason : true;
}

export function isMultiTenantEnabled(): boolean {
  return hasFeature('multi_tenant');
}

export function isHostedModeEnabled(): boolean {
  return Boolean(runtimeCapabilities()?.hosted_mode);
}

export function getRuntimeLimit(key: string) {
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
  batch(() => {
    setRuntimeCapabilities(null);
    setLoaded(false);
    setLoadError(null);
    void loadRuntimeCapabilities(true);
  });
});

export {
  runtimeCapabilities,
  loading as runtimeCapabilitiesLoading,
  loaded as runtimeCapabilitiesLoaded,
  loadError as runtimeCapabilitiesLoadError,
};
