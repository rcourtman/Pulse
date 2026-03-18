import { describe, expect, it } from 'vitest';
import { createRoot } from 'solid-js';
import { useStoragePageBannerModel } from '@/components/Storage/useStoragePageBannerModel';

describe('useStoragePageBannerModel', () => {
  it('returns canonical banner copy and actions', () => {
    createRoot((dispose) => {
      const reconnecting = useStoragePageBannerModel({
        kind: () => 'reconnecting',
      });
      expect(reconnecting.message()).toBe('Reconnecting to backend data stream…');
      expect(reconnecting.actionLabel()).toBe('Retry now');

      const waiting = useStoragePageBannerModel({
        kind: () => 'waiting-for-data',
      });
      expect(waiting.message()).toBe('Waiting for storage data from connected platforms.');
      expect(waiting.actionLabel()).toBeNull();
      dispose();
    });
  });
});
