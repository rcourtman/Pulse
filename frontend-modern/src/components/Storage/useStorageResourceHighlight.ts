import { createEffect, createSignal, on, onCleanup, type Accessor } from 'solid-js';
import { ResourceAPI } from '@/api/resources';
import type { StorageRecord } from '@/features/storageBackups/models';
import { resolveStorageRecordMetricResourceId } from '@/features/storageBackups/storageMetricsIdentity';
import { parseStorageLinkSearch, STORAGE_QUERY_PARAMS } from '@/routing/resourceLinks';

type UseStorageResourceHighlightOptions = {
  locationPathname: Accessor<string>;
  locationSearch: Accessor<string>;
  navigate: (path: string, options: { replace: true }) => void;
  records: Accessor<StorageRecord[]>;
  expandedPoolId: Accessor<string | null>;
  setExpandedPoolId: (value: string | null | ((current: string | null) => string | null)) => void;
  // A page that keeps Storage mounted behind another tab reads that tab's
  // `?resource=` too; the deep link only applies while Storage is showing.
  active?: Accessor<boolean>;
};

// A deep link names a record by its id, by the metrics id alerts and history
// use, or by name. A name only counts when one record carries it: every
// Proxmox node can have a `local-zfs`.
export const findHighlightedStorageRecord = (
  records: StorageRecord[],
  resource: string | null | undefined,
): StorageRecord | null => {
  if (!resource) return null;
  const byId = records.find((record) => record.id === resource);
  if (byId) return byId;
  const byMetricId = records.filter(
    (record) => resolveStorageRecordMetricResourceId(record) === resource,
  );
  if (byMetricId.length > 0) return byMetricId.length === 1 ? byMetricId[0] : null;
  const byName = records.filter((record) => record.name === resource);
  return byName.length === 1 ? byName[0] : null;
};

export const useStorageResourceHighlight = (
  options: UseStorageResourceHighlightOptions,
): Accessor<string | null> => {
  const active = () => options.active?.() ?? true;
  const [highlightedRecordId, setHighlightedRecordId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  // Other ids (an alert's source id, a superseded id) are resolved by the
  // resource registry, which owns aliases, succession and ambiguity.
  const [resolvedReference, setResolvedReference] = createSignal<{
    reference: string;
    id: string;
  } | null>(null);
  let pendingLookup: { reference: string; controller: AbortController } | undefined;
  // A visit is one stretch of Storage showing one `?resource=`. A refusal
  // lasts for the visit, so live record updates do not repeat the request
  // but a later visit retries. Once the visit has needed a lookup, any pool
  // opened or closed since is the operator's choice and outranks the link.
  let visitReference: string | null = null;
  let refusedReference: string | null = null;
  let lookupReference: string | null = null;
  let operatorChoseDuringLookup = false;
  let highlightTimer: number | undefined;

  const cancelLookup = () => {
    pendingLookup?.controller.abort();
    pendingLookup = undefined;
  };

  const startVisit = (reference: string | null) => {
    if (reference === visitReference) return;
    cancelLookup();
    visitReference = reference;
    refusedReference = null;
    lookupReference = null;
    operatorChoseDuringLookup = false;
    setResolvedReference(null);
  };

  createEffect(
    on(
      options.expandedPoolId,
      () => {
        if (lookupReference) operatorChoseDuringLookup = true;
      },
      { defer: true },
    ),
  );

  const operatorChoseAnotherPool = (reference: string) =>
    lookupReference === reference && operatorChoseDuringLookup;

  const lookUpReference = (reference: string) => {
    if (pendingLookup?.reference === reference || refusedReference === reference) return;
    cancelLookup();
    const controller = new AbortController();
    pendingLookup = { reference, controller };
    lookupReference ??= reference;
    void ResourceAPI.getResource(reference, { signal: controller.signal })
      .then((resource) => {
        if (controller.signal.aborted) return;
        pendingLookup = undefined;
        const id = resource.id?.trim();
        if (!id) {
          refusedReference = reference;
          return;
        }
        if (operatorChoseAnotherPool(reference)) {
          setHandledResourceId(reference);
          return;
        }
        setResolvedReference({ reference, id });
      })
      .catch(() => {
        // An unresolved or refused reference must not open another pool.
        if (controller.signal.aborted) return;
        pendingLookup = undefined;
        refusedReference = reference;
      });
  };

  createEffect(() => {
    if (!active()) return;
    const search = options.locationSearch();
    const params = new URLSearchParams(search);
    const rawResource = params.get(STORAGE_QUERY_PARAMS.resource);
    if (rawResource === null) return;

    const canonicalResource = parseStorageLinkSearch(search).resource;
    if (rawResource === canonicalResource) return;

    if (canonicalResource) {
      params.set(STORAGE_QUERY_PARAMS.resource, canonicalResource);
    } else {
      params.delete(STORAGE_QUERY_PARAMS.resource);
    }

    const nextSearch = params.toString();
    const nextPath = nextSearch
      ? `${options.locationPathname()}?${nextSearch}`
      : options.locationPathname();
    const currentPath = `${options.locationPathname()}${search || ''}`;
    if (nextPath !== currentPath) {
      options.navigate(nextPath, { replace: true });
    }
  });

  createEffect(() => {
    const { resource } = parseStorageLinkSearch(options.locationSearch());
    const handled = handledResourceId();
    // Handling one link does not carry over to another, even one followed
    // while Storage was hidden; hiding and showing the same link keeps it.
    if (handled && handled !== resource) setHandledResourceId(null);
    if (!active() || !resource) {
      // Leaving the link (or the tab) ends this visit.
      startVisit(null);
      return;
    }
    startVisit(resource);
    if (resource === handled) return;

    const records = options.records();
    const resolved = resolvedReference();
    const match =
      findHighlightedStorageRecord(records, resource) ??
      (resolved?.reference === resource
        ? (records.find((record) => record.id === resolved.id) ?? null)
        : null);
    if (!match) {
      // A resolved pool not listed yet opens when its record arrives. The
      // registry resolves ids, not names; a name several pools share stays
      // unopened rather than costing a lookup that cannot succeed.
      if (resolved?.reference === resource) return;
      if (!records.some((record) => record.name === resource)) lookUpReference(resource);
      return;
    }

    cancelLookup();
    if (operatorChoseAnotherPool(resource)) {
      setHandledResourceId(resource);
      return;
    }
    // Opening the pool lets the page's focus bridge reveal its row and details.
    options.setExpandedPoolId(match.id);
    setHighlightedRecordId(match.id);
    setHandledResourceId(resource);

    if (highlightTimer) window.clearTimeout(highlightTimer);
    highlightTimer = window.setTimeout(() => setHighlightedRecordId(null), 2000);
  });

  onCleanup(() => {
    cancelLookup();
    if (highlightTimer) window.clearTimeout(highlightTimer);
  });

  return highlightedRecordId;
};
