import { describe, expect, it } from 'vitest';
import { isStoragePoolLoading } from '@/features/storageBackups/storagePageStatus';

describe('storagePageStatus', () => {
  it('identifies pool-loading state canonically', () => {
    expect(isStoragePoolLoading(true, 'pools', 0)).toBe(true);
    expect(isStoragePoolLoading(true, 'disks', 0)).toBe(false);
    expect(isStoragePoolLoading(true, 'pools', 1)).toBe(false);
    expect(isStoragePoolLoading(false, 'pools', 0)).toBe(false);
  });
});
