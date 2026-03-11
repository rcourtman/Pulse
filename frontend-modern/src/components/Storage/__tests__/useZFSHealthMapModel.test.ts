import { renderHook } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import type { ZFSPool } from '@/types/api';
import { useZFSHealthMapModel } from '@/components/Storage/useZFSHealthMapModel';

describe('useZFSHealthMapModel', () => {
  it('builds canonical health map state', () => {
    const pool = () =>
      ({
        scan: 'resilver in progress',
        devices: [
          {
            name: 'sda',
            type: 'disk',
            state: 'ONLINE',
            message: '',
            readErrors: 0,
            writeErrors: 0,
            checksumErrors: 0,
          },
        ],
      }) as unknown as ZFSPool;

    const { result } = renderHook(() => useZFSHealthMapModel(pool));

    expect(result.devices()).toHaveLength(1);
    expect(result.isResilvering(result.devices()[0])).toBe(true);
    expect(result.hoveredTooltip()).toBeNull();
  });
});
