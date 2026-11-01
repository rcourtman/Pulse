import { createSignal } from 'solid-js';
import { renderHook } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { GuestMetadata } from '@/api/guestMetadata';
import { eventBus } from '@/stores/events';
import { RESOURCE_METADATA_CHANGED_EVENT } from '@/utils/resourceMetadataEvents';
import { useWorkloadGuestMetadataState } from '../useWorkloadGuestMetadataState';

const mocks = vi.hoisted(() => ({ org: 'default', read: vi.fn() }));
vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: { getAllMetadata: () => mocks.read() },
}));
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => mocks.org }));

type Metadata = Record<string, GuestMetadata>;
const record = (url: string): Metadata => ({ guest: { id: 'guest', customUrl: url } });
const deferred = () => {
  let resolve!: (value: Metadata) => void;
  const promise = new Promise<Metadata>((finish) => {
    resolve = finish;
  });
  return { promise, resolve };
};

describe('Workloads metadata owner lifetime', () => {
  beforeEach(() => {
    localStorage.clear();
    mocks.org = 'default';
    mocks.read.mockReset().mockResolvedValue({});
  });
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it.each(['resolve', 'deny'])(
    "does not expose a previous credential's disk data while the new read will %s",
    async (result) => {
      localStorage.setItem('pulseGuestMetadata', JSON.stringify(record('https://legacy.example')));
      localStorage.setItem(
        'pulseGuestMetadata.default',
        JSON.stringify(record('https://private.example')),
      );
      const pending = deferred();
      mocks.read.mockImplementation(() =>
        result === 'deny' ? Promise.reject(new Error('Forbidden')) : pending.promise,
      );
      const { result: state, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      expect(state.guestMetadata()).toEqual({});
      await Promise.resolve();
      expect(state.guestMetadata()).toEqual({});
      if (result === 'resolve') {
        pending.resolve(record('https://permitted.example'));
        await Promise.resolve();
        expect(state.guestMetadata()).toEqual(record('https://permitted.example'));
      }
      cleanup();
    },
  );

  it.each(['idle', 'timeout'])(
    'never queues a %s write that can restore removed metadata',
    async (scheduler) => {
      vi.useFakeTimers();
      const idle = vi.fn();
      vi.stubGlobal('requestIdleCallback', scheduler === 'idle' ? idle : undefined);
      const writes = vi.spyOn(Storage.prototype, 'setItem');
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await Promise.resolve();
      result.handleCustomUrlUpdate('guest', 'https://confirmed.example');
      expect(result.guestMetadata()).toEqual(record('https://confirmed.example'));
      cleanup();
      vi.runAllTimers();
      expect(idle).not.toHaveBeenCalled();
      expect(writes).not.toHaveBeenCalled();
      expect(localStorage.length).toBe(0);
    },
  );

  it('rejects a disposed read and callback without seeding the next owner in the same org', async () => {
    const old = deferred();
    const current = deferred();
    mocks.read.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
    const first = renderHook(() => useWorkloadGuestMetadataState());
    first.result.handleCustomUrlUpdate('guest', 'https://confirmed.example');
    first.cleanup();
    const second = renderHook(() => useWorkloadGuestMetadataState());
    expect(second.result.guestMetadata()).toEqual({});
    old.resolve(record('https://obsolete.example'));
    first.result.handleCustomUrlUpdate('guest', 'https://late-edit.example');
    await Promise.resolve();
    expect(first.result.guestMetadata()).toEqual({});
    expect(second.result.guestMetadata()).toEqual({});
    current.resolve(record('https://current.example'));
    await Promise.resolve();
    expect(second.result.guestMetadata()).toEqual(record('https://current.example'));
    second.cleanup();
  });

  it('does not reuse even a completed read in a new owner, or disturb an independent owner', async () => {
    mocks.read
      .mockResolvedValueOnce(record('https://first.example'))
      .mockResolvedValueOnce(record('https://independent.example'))
      .mockReturnValueOnce(new Promise(() => {}));
    const first = renderHook(() => useWorkloadGuestMetadataState());
    const independent = renderHook(() => useWorkloadGuestMetadataState());
    await Promise.resolve();
    first.cleanup();
    const replacement = renderHook(() => useWorkloadGuestMetadataState());
    expect(replacement.result.guestMetadata()).toEqual({});
    expect(independent.result.guestMetadata()).toEqual(record('https://independent.example'));
    independent.cleanup();
    replacement.cleanup();
  });

  it('clears outgoing metadata and rejects an old A read after an A → B → A switch', async () => {
    const oldA = deferred();
    const b = deferred();
    const newA = deferred();
    mocks.read
      .mockReturnValueOnce(oldA.promise)
      .mockReturnValueOnce(b.promise)
      .mockReturnValueOnce(newA.promise);
    const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
    result.handleCustomUrlUpdate('guest', 'https://confirmed.example');
    eventBus.emit('org_switched', 'b');
    expect(result.guestMetadata()).toEqual({});
    eventBus.emit('org_switched', 'default');
    oldA.resolve(record('https://old-a.example'));
    b.resolve(record('https://b.example'));
    await Promise.resolve();
    expect(result.guestMetadata()).toEqual({});
    newA.resolve(record('https://new-a.example'));
    await Promise.resolve();
    expect(result.guestMetadata()).toEqual(record('https://new-a.example'));
    cleanup();
  });

  it('cannot refresh or accept metadata events after disposal', async () => {
    const [ids, setIds] = createSignal<ReadonlySet<string>>(new Set(['guest']));
    const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState(ids));
    await Promise.resolve();
    cleanup();
    mocks.read.mockClear();
    setIds(new Set(['moved-guest']));
    eventBus.emit('org_switched', 'another');
    window.dispatchEvent(
      new CustomEvent(RESOURCE_METADATA_CHANGED_EVENT, {
        detail: { metadataId: 'guest', customUrl: 'https://late.example' },
      }),
    );
    expect(mocks.read).not.toHaveBeenCalled();
    expect(result.guestMetadata()).toEqual({});
  });
});
