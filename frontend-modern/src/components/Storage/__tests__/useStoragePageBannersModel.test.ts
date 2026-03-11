import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import { useStoragePageBannersModel } from '@/components/Storage/useStoragePageBannersModel';

describe('useStoragePageBannersModel', () => {
  it('keeps reconnect actions only on reconnecting/disconnected banners', () => {
    const [kind] = createSignal<'reconnecting' | 'fetch-error'>('reconnecting');

    const { result } = renderHook(() =>
      useStoragePageBannersModel({
        kind,
      }),
    );

    expect(result.reconnectActionKind()).toBe('reconnecting');
  });
});
