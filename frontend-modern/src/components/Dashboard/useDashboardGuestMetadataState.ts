import { createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import type { GuestMetadata } from '@/api/guestMetadata';
import { getOrgID } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { normalizeOrgScope } from '@/utils/orgScope';
import { eventBus } from '@/stores/events';

type GuestMetadataRecord = Record<string, GuestMetadata>;
type IdleCallbackHandle = number;
type IdleCallback = (deadline?: { didTimeout: boolean; timeRemaining: () => number }) => void;
type IdleCapableWindow = Window & {
  requestIdleCallback?: (
    callback: IdleCallback,
    options?: { timeout: number },
  ) => IdleCallbackHandle;
  cancelIdleCallback?: (handle: IdleCallbackHandle) => void;
};

let cachedGuestMetadata: GuestMetadataRecord | null = null;
let cachedGuestMetadataStorageKey: string | null = null;
let lastPersistedGuestMetadataJSON: string | null = null;
let lastPersistedGuestMetadataStorageKey: string | null = null;
let pendingPersistMetadata: GuestMetadataRecord | null = null;
let pendingPersistStorageKey: string | null = null;
let persistHandle: number | null = null;
let persistHandleType: 'idle' | 'timeout' | null = null;

const instrumentationEnabled = import.meta.env.DEV && typeof performance !== 'undefined';

const guestMetadataStorageKeyForOrg = (orgScope: string): string =>
  `${STORAGE_KEYS.GUEST_METADATA}.${encodeURIComponent(orgScope)}`;

const readGuestMetadataCache = (storageKey: string): GuestMetadataRecord => {
  if (cachedGuestMetadata && cachedGuestMetadataStorageKey === storageKey) {
    return cachedGuestMetadata;
  }

  if (typeof window === 'undefined') {
    cachedGuestMetadata = {};
    cachedGuestMetadataStorageKey = storageKey;
    return cachedGuestMetadata;
  }

  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) {
      cachedGuestMetadata = {};
      cachedGuestMetadataStorageKey = storageKey;
      lastPersistedGuestMetadataJSON = null;
      lastPersistedGuestMetadataStorageKey = storageKey;
      return cachedGuestMetadata;
    }

    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === 'object') {
      cachedGuestMetadata = parsed as GuestMetadataRecord;
      cachedGuestMetadataStorageKey = storageKey;
      lastPersistedGuestMetadataJSON = raw;
      lastPersistedGuestMetadataStorageKey = storageKey;
      return cachedGuestMetadata;
    }
  } catch (error) {
    logger.warn('Failed to parse cached guest metadata', error);
  }

  cachedGuestMetadata = {};
  cachedGuestMetadataStorageKey = storageKey;
  lastPersistedGuestMetadataJSON = null;
  lastPersistedGuestMetadataStorageKey = storageKey;
  return cachedGuestMetadata;
};

const clearPendingPersistHandle = (idleWindow: IdleCapableWindow) => {
  if (persistHandle === null || persistHandleType === null) {
    return;
  }

  if (persistHandleType === 'idle' && idleWindow.cancelIdleCallback) {
    idleWindow.cancelIdleCallback(persistHandle);
  } else if (persistHandleType === 'timeout') {
    window.clearTimeout(persistHandle);
  }

  persistHandle = null;
  persistHandleType = null;
};

const runGuestMetadataPersist = () => {
  if (typeof window === 'undefined' || !pendingPersistMetadata || !pendingPersistStorageKey) {
    pendingPersistMetadata = null;
    pendingPersistStorageKey = null;
    return;
  }

  const metadata = pendingPersistMetadata;
  const storageKey = pendingPersistStorageKey;
  pendingPersistMetadata = null;
  pendingPersistStorageKey = null;

  const markBase = instrumentationEnabled ? `guest-metadata:persist:${Date.now()}` : null;
  if (markBase) {
    performance.mark(`${markBase}:start`);
  }

  let serialized: string;
  try {
    serialized = JSON.stringify(metadata);
  } catch (error) {
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
    logger.warn('Failed to serialize guest metadata cache', error);
    return;
  }

  if (
    serialized === lastPersistedGuestMetadataJSON &&
    storageKey === lastPersistedGuestMetadataStorageKey
  ) {
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      const entries = performance.getEntriesByName(markBase);
      const entry = entries[entries.length - 1];
      if (entry) {
        logger.debug('[guestMetadataCache] skipped persist (unchanged)', {
          durationMs: entry.duration,
        });
      }
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
    return;
  }

  try {
    window.localStorage.setItem(storageKey, serialized);
    lastPersistedGuestMetadataJSON = serialized;
    lastPersistedGuestMetadataStorageKey = storageKey;
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      const entries = performance.getEntriesByName(markBase);
      const entry = entries[entries.length - 1];
      if (entry) {
        logger.debug('[guestMetadataCache] persisted entries', {
          count: Object.keys(metadata).length,
          durationMs: entry.duration,
        });
      }
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
  } catch (error) {
    if (markBase) {
      performance.mark(`${markBase}:end`);
      performance.measure(markBase, `${markBase}:start`, `${markBase}:end`);
      performance.clearMarks(`${markBase}:start`);
      performance.clearMarks(`${markBase}:end`);
      performance.clearMeasures(markBase);
    }
    logger.warn('Failed to persist guest metadata cache', error);
  }
};

const queueGuestMetadataPersist = (storageKey: string, metadata: GuestMetadataRecord) => {
  cachedGuestMetadata = metadata;
  cachedGuestMetadataStorageKey = storageKey;

  if (typeof window === 'undefined') {
    return;
  }

  pendingPersistMetadata = metadata;
  pendingPersistStorageKey = storageKey;
  const idleWindow = window as IdleCapableWindow;

  clearPendingPersistHandle(idleWindow);

  const schedule: IdleCallback = () => {
    persistHandle = null;
    persistHandleType = null;
    runGuestMetadataPersist();
  };

  if (idleWindow.requestIdleCallback) {
    persistHandleType = 'idle';
    persistHandle = idleWindow.requestIdleCallback(schedule, { timeout: 750 });
  } else {
    persistHandleType = 'timeout';
    persistHandle = window.setTimeout(schedule, 0);
  }
};

export function useDashboardGuestMetadataState() {
  const [orgScope, setOrgScope] = createSignal(normalizeOrgScope(getOrgID()));
  const guestMetadataStorageKey = createMemo(() => guestMetadataStorageKeyForOrg(orgScope()));
  const [guestMetadata, setGuestMetadata] = createSignal<GuestMetadataRecord>(
    readGuestMetadataCache(guestMetadataStorageKey()),
  );

  const updateGuestMetadataState = (updater: (prev: GuestMetadataRecord) => GuestMetadataRecord) =>
    setGuestMetadata((prev) => {
      const next = updater(prev);
      if (next === prev) {
        return prev;
      }
      queueGuestMetadataPersist(guestMetadataStorageKey(), next);
      return next;
    });

  const refreshGuestMetadata = async () => {
    try {
      const metadata = await GuestMetadataAPI.getAllMetadata();
      updateGuestMetadataState(() => metadata || {});
      logger.debug('Guest metadata refreshed');
    } catch (error) {
      logger.debug('Failed to refresh guest metadata', error);
    }
  };

  const handleCustomUrlUpdate = (guestId: string, url: string) => {
    const trimmedUrl = url.trim();
    const nextUrl = trimmedUrl === '' ? undefined : trimmedUrl;
    const currentUrl = guestMetadata()[guestId]?.customUrl;
    if (currentUrl === nextUrl) {
      return;
    }

    updateGuestMetadataState((prev) => {
      const previousEntry = prev[guestId];

      if (nextUrl === undefined) {
        if (!previousEntry || typeof previousEntry.customUrl === 'undefined') {
          return prev;
        }
        const { customUrl: _removed, ...restEntry } = previousEntry;
        const hasAdditionalMetadata = Object.entries(restEntry).some(
          ([key, value]) => key !== 'id' && value !== undefined,
        );

        if (!hasAdditionalMetadata) {
          const { [guestId]: _omit, ...rest } = prev;
          return rest;
        }

        return {
          ...prev,
          [guestId]: {
            ...restEntry,
            id: restEntry.id ?? guestId,
          },
        };
      }

      if (previousEntry && previousEntry.customUrl === nextUrl) {
        return prev;
      }

      const nextEntry: GuestMetadata = {
        ...(previousEntry || { id: guestId }),
        customUrl: nextUrl,
      };

      return {
        ...prev,
        [guestId]: nextEntry,
      };
    });
  };

  onMount(() => {
    void refreshGuestMetadata();

    const handleMetadataChanged = (event: Event) => {
      const customEvent = event as CustomEvent;
      logger.debug('[Dashboard] Metadata changed event received', customEvent.detail);

      if (customEvent.detail?.payload) {
        let { guestId, url } = customEvent.detail.payload;
        if (guestId) {
          if (guestId.includes(':')) {
            const parts = guestId.split(':');
            if (parts.length === 3) {
              const [instance, _node, vmid] = parts;
              guestId = `${instance}-${vmid}`;
              logger.debug('[Dashboard] Normalized optimistic guestId', {
                original: customEvent.detail.payload.guestId,
                normalized: guestId,
              });
            }
          }

          logger.debug('[Dashboard] Applying optimistic metadata update', { guestId, url });
          handleCustomUrlUpdate(guestId, url || '');
          return;
        }
      }

      logger.debug('Metadata changed event received, refreshing...');
      void refreshGuestMetadata();
    };

    logger.debug('[Dashboard] Adding pulse:metadata-changed listener');
    window.addEventListener('pulse:metadata-changed', handleMetadataChanged);
    const unsubscribeOrgSwitched = eventBus.on('org_switched', (nextOrgID) => {
      setOrgScope(normalizeOrgScope(nextOrgID));
      void refreshGuestMetadata();
    });

    onCleanup(() => {
      window.removeEventListener('pulse:metadata-changed', handleMetadataChanged);
      unsubscribeOrgSwitched();
    });
  });

  createEffect(() => {
    const storageKey = guestMetadataStorageKey();
    setGuestMetadata(readGuestMetadataCache(storageKey));
  });

  return {
    guestMetadata,
    handleCustomUrlUpdate,
  };
}
