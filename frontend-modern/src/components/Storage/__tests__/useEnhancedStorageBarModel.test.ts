import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { ZFSPool } from '@/types/api';
import { useEnhancedStorageBarModel } from '@/components/Storage/useEnhancedStorageBarModel';

describe('useEnhancedStorageBarModel', () => {
  it('builds canonical storage bar model state', () => {
    const [used] = createSignal(400);
    const [total] = createSignal(1000);
    const [free] = createSignal(600);
    const [zfsPool] = createSignal<ZFSPool | undefined>({
      state: 'ONLINE',
      scan: 'scrub in progress',
      readErrors: 0,
      writeErrors: 0,
      checksumErrors: 0,
    } as any);

    const { result } = renderHook(() =>
      useEnhancedStorageBarModel({
        used,
        total,
        free,
        zfsPool,
      }),
    );

    expect(result.usagePercent()).toBe(40);
    expect(result.label()).toBe('40% (400 B/1000 B)');
    expect(result.tooltipRows()).toEqual([
      { label: 'Used', value: '400 B' },
      { label: 'Free', value: '600 B' },
      { label: 'Total', value: '1000 B', bordered: true },
    ]);
    expect(result.zfsSummary()).toEqual({
      hasErrors: false,
      isScrubbing: true,
      isResilvering: false,
      state: 'ONLINE',
      scan: 'scrub in progress',
      errorSummary: '',
    });
  });
});
