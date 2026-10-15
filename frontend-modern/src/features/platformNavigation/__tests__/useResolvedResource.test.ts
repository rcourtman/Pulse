import { createRoot, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { useResolvedResource } from '../useResolvedResource';

const getResource = vi.hoisted(() => vi.fn());
vi.mock('@/api/resources', () => ({ ResourceAPI: { getResource } }));

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

afterEach(() => {
  getResource.mockReset();
});

describe('useResolvedResource', () => {
  it('returns the registry resource for the current id and null when it cannot resolve', async () => {
    const vm = { id: 'vm-1', name: 'web', type: 'vm' } as Resource;
    getResource.mockImplementation((id: string) =>
      id === 'vm-1' ? Promise.resolve(vm) : Promise.reject(new Error('not found')),
    );
    await createRoot(async (dispose) => {
      const [id, setId] = createSignal<string | undefined>(' vm-1 ');
      const resolved = useResolvedResource(id);
      expect(resolved()).toBeNull();
      await flush();
      expect(resolved()).toBe(vm);
      expect(getResource).toHaveBeenCalledWith('vm-1', expect.anything());

      setId('missing');
      expect(resolved()).toBeNull();
      await flush();
      expect(resolved()).toBeNull();
      dispose();
    });
  });

  // Switching records while a lookup is in flight must not show the earlier
  // record's resource for the new one.
  it('never returns an answer for an earlier id', async () => {
    let finishFirst: (resource: Resource) => void = () => undefined;
    getResource.mockImplementation((id: string) =>
      id === 'first'
        ? new Promise<Resource>((resolve) => {
            finishFirst = resolve;
          })
        : new Promise<Resource>(() => undefined),
    );
    await createRoot(async (dispose) => {
      const [id, setId] = createSignal<string | undefined>('first');
      const resolved = useResolvedResource(id);
      await flush();
      expect(getResource).toHaveBeenCalledWith('first', expect.anything());
      setId('second');
      finishFirst({ id: 'first', name: 'old', type: 'vm' } as Resource);
      await flush();
      expect(resolved()).toBeNull();
      dispose();
    });
  });
});
