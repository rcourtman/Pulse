import {
  createEffect,
  createSignal,
  on,
  onCleanup,
  onMount,
  type Accessor,
} from 'solid-js';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import type { GuestMetadata } from '@/api/guestMetadata';
import { getOrgID } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { normalizeOrgScope } from '@/utils/orgScope';
import {
  RESOURCE_METADATA_CHANGED_EVENT,
  type ResourceMetadataChangedDetail,
} from '@/utils/resourceMetadataEvents';
import { eventBus } from '@/stores/events';

type GuestMetadataRecord = Record<string, GuestMetadata>;
// Saved metadata belongs to the server. Keep only this mounted owner's copy:
// an org-only disk/module cache can outlive the credential that read it, and
// delayed persistence can recreate it after sign-out removed it.
const isStableAppContainerMetadataId = (guestId: string): boolean =>
  guestId.trim().startsWith('app-container:') && guestId.includes(':name:');

export function useWorkloadGuestMetadataState(metadataIds?: Accessor<ReadonlySet<string>>) {
  let refreshRevision = 0;
  let disposed = false;
  const [orgScope, setOrgScope] = createSignal(normalizeOrgScope(getOrgID()));
  const [guestMetadata, setGuestMetadata] = createSignal<GuestMetadataRecord>({});

  // The authenticated app shell disposes this owner on confirmed sign-out,
  // even when provider navigation or browser cleanup cannot complete.
  onCleanup(() => {
    disposed = true;
    refreshRevision += 1;
    setGuestMetadata({});
  });

  const updateGuestMetadataState = (updater: (prev: GuestMetadataRecord) => GuestMetadataRecord) => {
    if (!disposed) setGuestMetadata(updater);
  };

  const refreshGuestMetadata = async () => {
    if (disposed) return;
    const revision = ++refreshRevision;
    const requestOrg = orgScope();
    try {
      const metadata = await GuestMetadataAPI.getAllMetadata();
      if (disposed || revision !== refreshRevision || requestOrg !== orgScope()) return;
      updateGuestMetadataState(() => metadata || {});
      logger.debug('Guest metadata refreshed');
    } catch (error) {
      logger.debug('Failed to refresh guest metadata', error);
    }
  };

  const handleCustomUrlUpdate = (guestId: string, url: string) => {
    if (disposed) return;
    const trimmedUrl = url.trim();
    const nextUrl = trimmedUrl === '' ? undefined : trimmedUrl;
    const currentUrl = guestMetadata()[guestId]?.customUrl;
    const shouldKeepStableClearRecord =
      nextUrl === undefined && isStableAppContainerMetadataId(guestId);
    if (currentUrl === nextUrl && !shouldKeepStableClearRecord) {
      return;
    }

    refreshRevision += 1;
    updateGuestMetadataState((prev) => {
      const previousEntry = prev[guestId];

      if (nextUrl === undefined) {
        if (isStableAppContainerMetadataId(guestId)) {
          return {
            ...prev,
            [guestId]: {
              ...(previousEntry || { id: guestId }),
              customUrl: '',
            },
          };
        }

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

  // The server migrates provider metadata keys before publishing a node move.
  // Refetch when that boundary coordinate changes, rather than retaining only
  // the initial map under the previous node key. Metric-only ticks keep it.
  if (metadataIds)
    createEffect(on(metadataIds, () => void refreshGuestMetadata(), { defer: true }));

  onMount(() => {
    void refreshGuestMetadata();

    const handleMetadataChanged = (event: Event) => {
      const detail = (event as CustomEvent<ResourceMetadataChangedDetail>).detail;
      logger.debug('[Workloads] Metadata changed event received', detail);

      // Agent (host machine) metadata never keys workload rows; only guest and
      // docker app-container URLs surface in the workloads table.
      if (detail?.metadataKind === 'agent') return;

      if (detail?.metadataId) {
        handleCustomUrlUpdate(detail.metadataId, detail.customUrl ?? '');
        return;
      }

      void refreshGuestMetadata();
    };

    window.addEventListener(RESOURCE_METADATA_CHANGED_EVENT, handleMetadataChanged);
    const unsubscribeOrgSwitched = eventBus.on('org_switched', (nextOrgID) => {
      refreshRevision += 1;
      setGuestMetadata({});
      setOrgScope(normalizeOrgScope(nextOrgID));
      void refreshGuestMetadata();
    });

    onCleanup(() => {
      window.removeEventListener(RESOURCE_METADATA_CHANGED_EVENT, handleMetadataChanged);
      unsubscribeOrgSwitched();
    });
  });

  return {
    guestMetadata,
    handleCustomUrlUpdate,
  };
}
