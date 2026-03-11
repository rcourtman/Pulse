import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';
import type { StorageRecord } from '@/features/storageBackups/models';
import { parseStorageLinkSearch } from '@/routing/resourceLinks';

type UseStorageResourceHighlightOptions = {
  locationSearch: Accessor<string>;
  records: Accessor<StorageRecord[]>;
  isStorageRecordCeph: (record: StorageRecord) => boolean;
  setExpandedPoolId: (value: string | null | ((current: string | null) => string | null)) => void;
};

export const findHighlightedStorageRecord = (
  records: StorageRecord[],
  resource: string | null | undefined,
): StorageRecord | null => {
  if (!resource) return null;
  return records.find((record) => record.id === resource || record.name === resource) ?? null;
};

export const useStorageResourceHighlight = (
  options: UseStorageResourceHighlightOptions,
): Accessor<string | null> => {
  const [highlightedRecordId, setHighlightedRecordId] = createSignal<string | null>(null);
  const [handledResourceId, setHandledResourceId] = createSignal<string | null>(null);
  let highlightTimer: number | undefined;

  createEffect(() => {
    const { resource } = parseStorageLinkSearch(options.locationSearch());
    if (!resource || resource === handledResourceId()) return;

    const match = findHighlightedStorageRecord(options.records(), resource);
    if (!match) return;

    if (options.isStorageRecordCeph(match)) {
      options.setExpandedPoolId(match.id);
    }

    setHighlightedRecordId(match.id);
    setHandledResourceId(resource);

    if (highlightTimer) window.clearTimeout(highlightTimer);
    highlightTimer = window.setTimeout(() => setHighlightedRecordId(null), 2000);
  });

  onCleanup(() => {
    if (highlightTimer) window.clearTimeout(highlightTimer);
  });

  return highlightedRecordId;
};
