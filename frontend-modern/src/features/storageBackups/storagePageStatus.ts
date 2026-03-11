import type { StoragePageBannerKind } from './storagePagePresentation';

export type StoragePageBannerStateInput = {
  loading: boolean;
  filteredRecordCount: number;
  connected: boolean;
  initialDataReceived: boolean;
  reconnecting: boolean;
  hasFetchError: boolean;
};

export const getStoragePageBannerKind = (
  input: StoragePageBannerStateInput,
): StoragePageBannerKind | null => {
  if (input.reconnecting) return 'reconnecting';
  if (input.hasFetchError) return 'fetch-error';
  if (!input.connected && input.initialDataReceived) return 'disconnected';
  if (
    input.loading &&
    input.filteredRecordCount === 0 &&
    !input.connected &&
    !input.initialDataReceived
  ) {
    return 'waiting-for-data';
  }
  return null;
};

export const isStoragePoolLoading = (
  loading: boolean,
  view: 'pools' | 'disks',
  filteredRecordCount: number,
): boolean => loading && view === 'pools' && filteredRecordCount === 0;
