import { describe, expect, it } from 'vitest';
import {
  getStoragePageBannerKind,
  isStoragePoolLoading,
} from '@/features/storageBackups/storagePageStatus';

describe('storagePageStatus', () => {
  it('chooses the canonical storage banner kind', () => {
    expect(
      getStoragePageBannerKind({
        loading: false,
        filteredRecordCount: 3,
        connected: true,
        initialDataReceived: true,
        reconnecting: true,
        hasFetchError: false,
      }),
    ).toBe('reconnecting');

    expect(
      getStoragePageBannerKind({
        loading: false,
        filteredRecordCount: 3,
        connected: true,
        initialDataReceived: true,
        reconnecting: false,
        hasFetchError: true,
      }),
    ).toBe('fetch-error');

    expect(
      getStoragePageBannerKind({
        loading: false,
        filteredRecordCount: 3,
        connected: false,
        initialDataReceived: true,
        reconnecting: false,
        hasFetchError: false,
      }),
    ).toBe('disconnected');

    expect(
      getStoragePageBannerKind({
        loading: true,
        filteredRecordCount: 0,
        connected: false,
        initialDataReceived: false,
        reconnecting: false,
        hasFetchError: false,
      }),
    ).toBe('waiting-for-data');

    expect(
      getStoragePageBannerKind({
        loading: false,
        filteredRecordCount: 2,
        connected: true,
        initialDataReceived: true,
        reconnecting: false,
        hasFetchError: false,
      }),
    ).toBeNull();
  });

  it('identifies pool-loading state canonically', () => {
    expect(isStoragePoolLoading(true, 'pools', 0)).toBe(true);
    expect(isStoragePoolLoading(true, 'disks', 0)).toBe(false);
    expect(isStoragePoolLoading(true, 'pools', 1)).toBe(false);
    expect(isStoragePoolLoading(false, 'pools', 0)).toBe(false);
  });
});
