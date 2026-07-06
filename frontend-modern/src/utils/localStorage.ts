import { createSignal, createEffect, onCleanup, Signal } from 'solid-js';

const LOCAL_STORAGE_SYNC_EVENT = 'pulse-localstorage-sync';

type LocalStorageSyncDetail = {
  key: string;
  value: string | null;
};

function broadcastLocalStorageChange(key: string, value: string | null): void {
  if (typeof window === 'undefined') return;
  try {
    window.dispatchEvent(
      new CustomEvent<LocalStorageSyncDetail>(LOCAL_STORAGE_SYNC_EVENT, {
        detail: { key, value },
      }),
    );
  } catch {
    // Ignore event dispatch errors
  }
}

/**
 * Creates a signal that syncs with localStorage
 * @param key - The localStorage key
 * @param defaultValue - Default value if nothing in localStorage
 * @param parse - Optional parser function for complex types
 * @param stringify - Optional stringify function for complex types
 */
function createLocalStorageSignal<T>(
  key: string,
  defaultValue: T,
  parse?: (value: string) => T,
  stringify?: (value: T) => string,
): Signal<T> {
  // Get initial value from localStorage
  const stored = localStorage.getItem(key);
  const initialValue =
    stored !== null ? (parse ? parse(stored) : (stored as unknown as T)) : defaultValue;

  const [value, setValue] = createSignal<T>(initialValue);

  // Keep multiple instances in sync (and reflect updates performed elsewhere in the same tab).
  if (typeof window !== 'undefined') {
    const applyRaw = (raw: string | null) => {
      const next = raw !== null ? (parse ? parse(raw) : (raw as unknown as T)) : defaultValue;
      if (Object.is(next, value())) return;
      setValue(() => next);
    };

    const handleStorage = (e: StorageEvent) => {
      if (e.storageArea !== window.localStorage) return;
      if (e.key !== key) return;
      applyRaw(e.newValue);
    };

    const handleCustom = (e: Event) => {
      const evt = e as CustomEvent<LocalStorageSyncDetail>;
      if (!evt.detail || evt.detail.key !== key) return;
      applyRaw(evt.detail.value);
    };

    window.addEventListener('storage', handleStorage);
    window.addEventListener(LOCAL_STORAGE_SYNC_EVENT, handleCustom);
    onCleanup(() => {
      window.removeEventListener('storage', handleStorage);
      window.removeEventListener(LOCAL_STORAGE_SYNC_EVENT, handleCustom);
    });
  }

  // Sync to localStorage on changes
  createEffect(() => {
    const val = value();
    if (val === null || val === undefined) {
      localStorage.removeItem(key);
      broadcastLocalStorageChange(key, null);
    } else {
      const raw = stringify ? stringify(val) : String(val);
      localStorage.setItem(key, raw);
      broadcastLocalStorageChange(key, raw);
    }
  });

  return [value, setValue];
}

/**
 * Creates a boolean signal that syncs with localStorage
 * @param key - The localStorage key
 * @param defaultValue - Default value if nothing in localStorage
 */
export function createLocalStorageBooleanSignal(
  key: string,
  defaultValue: boolean = false,
): Signal<boolean> {
  return createLocalStorageSignal(
    key,
    defaultValue,
    (val) => val === 'true',
    (val) => String(val),
  );
}

/**
 * Creates a number signal that syncs with localStorage
 * @param key - The localStorage key
 * @param defaultValue - Default value if nothing in localStorage
 */
export function createLocalStorageNumberSignal(
  key: string,
  defaultValue: number = 0,
): Signal<number> {
  return createLocalStorageSignal(
    key,
    defaultValue,
    (val) => {
      const parsed = Number(val);
      return Number.isFinite(parsed) ? parsed : defaultValue;
    },
    (val) => String(val),
  );
}

/**
 * Creates a string signal that syncs with localStorage
 * @param key - The localStorage key
 * @param defaultValue - Default value if nothing in localStorage
 */
export function createLocalStorageStringSignal(
  key: string,
  defaultValue: string = '',
): Signal<string> {
  return createLocalStorageSignal(
    key,
    defaultValue,
    (val) => String(val),
    (val) => String(val),
  );
}

/**
 * Creates a number signal that syncs with localStorage
 * @param key - The localStorage key
 * @param defaultValue - Default value if nothing in localStorage
 */
// Storage keys used throughout the application
export const STORAGE_KEYS = {
  // Authentication
  AUTH: 'pulse_auth',
  AUTH_USER: 'pulse_auth_user',
  REMEMBERED_LOGIN_USERNAME: 'pulse_remembered_login_username',
  ORG_ID: 'pulse_org_id',
  SETUP_CREDENTIALS: 'pulse_setup_credentials',
  SETUP_HANDOFF: 'pulse_setup_handoff',

  // UI preferences
  THEME_PREFERENCE: 'pulseThemePreference',
  LOCALE_PREFERENCE: 'pulseLocalePreference',
  SIDEBAR_COLLAPSED: 'sidebarCollapsed',
  FULL_WIDTH_MODE: 'fullWidthMode',

  // Metadata
  GUEST_METADATA: 'pulseGuestMetadata',
  DOCKER_METADATA: 'pulseDockerMetadata',

  // Updates
  UPDATES: 'pulse-updates',

  // Alert settings
  ALERT_HISTORY_TIME_FILTER: 'alertHistoryTimeFilter',
  ALERT_HISTORY_SEVERITY_FILTER: 'alertHistorySeverityFilter',

  // Recovery settings
  RECOVERY_SEARCH_HISTORY: 'backupsSearchHistory',

  // Workloads settings
  WORKLOADS_SEARCH_HISTORY: 'workloadsSearchHistory',
  WORKLOADS_METRIC_DISPLAY_MODE: 'workloadsMetricDisplayMode',
  WORKLOADS_METRIC_HISTORY_RANGE: 'workloadsMetricHistoryRange',
  AGENTS_SEARCH_HISTORY: 'agentsSearchHistory',

  // Machines settings
  MACHINES_SEARCH_HISTORY: 'machinesSearchHistory',

  // Storage search
  STORAGE_SEARCH_HISTORY: 'storageSearchHistory',

  // Alerts search
  ALERTS_SEARCH_HISTORY: 'alertsSearchHistory',

  // Temperature display
  TEMPERATURE_UNIT: 'temperatureUnit', // 'celsius' | 'fahrenheit'

  // Column visibility
  WORKLOADS_HIDDEN_COLUMNS: 'workloadsHiddenColumns',
  RECOVERY_HIDDEN_COLUMNS: 'backupsHiddenColumns',
  STORAGE_HIDDEN_COLUMNS: 'storageHiddenColumns',

  // Resources search
  RESOURCES_SEARCH_HISTORY: 'resourcesSearchHistory',

  // Feature discovery
  DISMISSED_FEATURE_TIPS: 'pulse-dismissed-feature-tips',
  DEBUG_MODE: 'pulse_debug_mode',

  // GitHub star prompt
  GITHUB_STAR_DISMISSED: 'pulse-github-star-dismissed',
  GITHUB_STAR_FIRST_SEEN: 'pulse-github-star-first-seen',
  GITHUB_STAR_SNOOZED_UNTIL: 'pulse-github-star-snoozed-until',

  // Billing / migration guidance
  AGENT_MIGRATION_NOTICE_DISMISSED: 'pulse-agent-migration-notice-dismissed',

  // Audit log
  AUDIT_AUTO_VERIFY: 'pulse-audit-auto-verify',
  AUDIT_AUTO_VERIFY_LIMIT: 'pulse-audit-auto-verify-limit',
  AUDIT_PAGE_SIZE: 'pulse-audit-page-size',
  AUDIT_PAGE_OFFSET: 'pulse-audit-page-offset',
  AUDIT_VERIFICATION_FILTER: 'pulse-audit-verification-filter',
  AUDIT_EVENT_FILTER: 'pulse-audit-event-filter',
  AUDIT_USER_FILTER: 'pulse-audit-user-filter',
  AUDIT_SUCCESS_FILTER: 'pulse-audit-success-filter',
} as const;
