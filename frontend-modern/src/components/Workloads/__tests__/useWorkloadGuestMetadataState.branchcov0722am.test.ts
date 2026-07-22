import { createRoot } from 'solid-js';
import { renderHook } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  RESOURCE_METADATA_CHANGED_EVENT,
  type ResourceMetadataChangedDetail,
} from '@/utils/resourceMetadataEvents';

import { useWorkloadGuestMetadataState } from '../useWorkloadGuestMetadataState';

// This suite targets the branch arms the base suite leaves cold: the
// metadata-changed event subscription (no/nil metadataId, missing kind or
// customUrl, docker kind), refresh (rejection + null payload), the org_switched
// subscription body, onMount teardown, the handleCustomUrlUpdate clearing/trim
// arms, and the localStorage persist queue. Each test is isolated by giving the
// hook a unique org scope (unique storage key) so the module-level metadata
// cache never leaks state across tests.

type OrgSwitchedHandler = (orgID?: string) => void;

const mocks = vi.hoisted(() => ({
  orgID: 'default',
  getAllMetadata: vi.fn().mockResolvedValue({}),
  unsubscribeOrgSwitched: vi.fn(),
  orgSwitchedHandler: { current: null as OrgSwitchedHandler | null },
}));

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: {
    getAllMetadata: () => mocks.getAllMetadata(),
  },
}));

vi.mock('@/utils/apiClient', () => ({
  getOrgID: () => mocks.orgID,
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: vi.fn((_event: unknown, handler: OrgSwitchedHandler) => {
      mocks.orgSwitchedHandler.current = handler;
      return mocks.unsubscribeOrgSwitched;
    }),
  },
}));

const STORAGE_PREFIX = 'pulseGuestMetadata.';
const keyFor = (orgScope: string) => `${STORAGE_PREFIX}${encodeURIComponent(orgScope)}`;
const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

const dispatchMetadataEvent = (detail?: ResourceMetadataChangedDetail) =>
  window.dispatchEvent(
    new CustomEvent<ResourceMetadataChangedDetail>(RESOURCE_METADATA_CHANGED_EVENT, { detail }),
  );

describe('useWorkloadGuestMetadataState (branch coverage 0722am)', () => {
  beforeEach(() => {
    window.localStorage.clear();
    mocks.orgID = 'default';
    mocks.getAllMetadata.mockReset();
    mocks.getAllMetadata.mockResolvedValue({});
    mocks.unsubscribeOrgSwitched.mockReset();
    mocks.orgSwitchedHandler.current = null;
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  describe('handleCustomUrlUpdate clearing / trim arms', () => {
    it('trims surrounding whitespace from the incoming url before storing it', () => {
      mocks.orgID = 'trim';
      createRoot((dispose) => {
        const state = useWorkloadGuestMetadataState();
        state.handleCustomUrlUpdate('guest:trim', '   https://trim.example   ');
        expect(state.guestMetadata()['guest:trim']?.customUrl).toBe('https://trim.example');
        dispose();
      });
    });

    it('no-ops (keeps the same signal value reference) when re-applying the current url', () => {
      mocks.orgID = 'noop-same-url';
      createRoot((dispose) => {
        const state = useWorkloadGuestMetadataState();
        state.handleCustomUrlUpdate('guest:noop', 'https://a.example');
        const afterSet = state.guestMetadata();
        state.handleCustomUrlUpdate('guest:noop', 'https://a.example');
        expect(Object.is(state.guestMetadata(), afterSet)).toBe(true);
        dispose();
      });
    });

    it('omits a non-stable entry entirely when cleared and it only carried customUrl', () => {
      mocks.orgID = 'omit';
      createRoot((dispose) => {
        const state = useWorkloadGuestMetadataState();
        state.handleCustomUrlUpdate('guest:omit', 'https://o.example');
        expect(state.guestMetadata()['guest:omit']?.customUrl).toBe('https://o.example');
        state.handleCustomUrlUpdate('guest:omit', '');
        expect(state.guestMetadata()['guest:omit']).toBeUndefined();
        dispose();
      });
    });

    it('keeps a non-stable entry (dropping only customUrl) when it carries additional metadata', () => {
      mocks.orgID = 'keep-extra';
      window.localStorage.setItem(
        keyFor('keep-extra'),
        JSON.stringify({
          'guest:keep': {
            id: 'guest:keep',
            customUrl: 'https://k.example',
            description: 'desc',
            tags: ['t'],
          },
        }),
      );
      createRoot((dispose) => {
        const state = useWorkloadGuestMetadataState();
        state.handleCustomUrlUpdate('guest:keep', '');
        expect(state.guestMetadata()['guest:keep']).toEqual({
          id: 'guest:keep',
          description: 'desc',
          tags: ['t'],
        });
        expect(state.guestMetadata()['guest:keep']?.customUrl).toBeUndefined();
        dispose();
      });
    });

    it('no-ops when clearing a non-stable id that has no entry at all', () => {
      mocks.orgID = 'clear-empty';
      createRoot((dispose) => {
        const state = useWorkloadGuestMetadataState();
        state.handleCustomUrlUpdate('guest:none', '');
        expect(state.guestMetadata()['guest:none']).toBeUndefined();
        expect(Object.keys(state.guestMetadata())).toHaveLength(0);
        dispose();
      });
    });

    it('detects a stable app-container id even with surrounding whitespace, keying the record by the raw id', () => {
      mocks.orgID = 'stable-ws';
      createRoot((dispose) => {
        const state = useWorkloadGuestMetadataState();
        const rawId = '   app-container:docker:name:grafana   ';
        state.handleCustomUrlUpdate(rawId, '');
        const meta = state.guestMetadata();
        expect(meta[rawId]).toEqual({ id: rawId, customUrl: '' });
        expect(meta[rawId.trim()]).toBeUndefined();
        dispose();
      });
    });

    it('does not treat an app-container id lacking :name: as stable', () => {
      mocks.orgID = 'not-stable';
      createRoot((dispose) => {
        const state = useWorkloadGuestMetadataState();
        state.handleCustomUrlUpdate('app-container:docker:runtime-id', '');
        expect(state.guestMetadata()['app-container:docker:runtime-id']).toBeUndefined();
        dispose();
      });
    });
  });

  describe('handleMetadataChanged event subscription arms', () => {
    it('refreshes from the API when a metadata-changed event carries an empty metadataId', async () => {
      mocks.orgID = 'evt-no-id';
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      mocks.getAllMetadata.mockResolvedValue({
        'guest:after': { id: 'guest:after', customUrl: 'https://after.example' },
      });
      mocks.getAllMetadata.mockClear();
      dispatchMetadataEvent({
        metadataKind: 'guest',
        metadataId: '',
        customUrl: 'https://ignored.example',
      });
      await flush();
      expect(mocks.getAllMetadata).toHaveBeenCalledTimes(1);
      expect(result.guestMetadata()['guest:after']?.customUrl).toBe('https://after.example');
      cleanup();
    });

    it('refreshes when a metadata-changed event has no detail payload at all', async () => {
      mocks.orgID = 'evt-nil-detail';
      const { cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      mocks.getAllMetadata.mockClear();
      dispatchMetadataEvent(undefined);
      await flush();
      expect(mocks.getAllMetadata).toHaveBeenCalledTimes(1);
      cleanup();
    });

    it('routes events with an undefined metadataKind through the non-agent path', () => {
      mocks.orgID = 'evt-no-kind';
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      dispatchMetadataEvent({
        metadataKind: undefined,
        metadataId: 'app-container:host:name:srv',
        customUrl: 'https://nk.example',
      } as unknown as ResourceMetadataChangedDetail);
      expect(result.guestMetadata()['app-container:host:name:srv']?.customUrl).toBe(
        'https://nk.example',
      );
      cleanup();
    });

    it('applies docker-kind events via handleCustomUrlUpdate instead of refreshing', async () => {
      mocks.orgID = 'evt-docker';
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      mocks.getAllMetadata.mockClear();
      dispatchMetadataEvent({
        metadataKind: 'docker',
        metadataId: 'docker:ctr1',
        customUrl: 'https://d.example',
      });
      expect(result.guestMetadata()['docker:ctr1']?.customUrl).toBe('https://d.example');
      expect(mocks.getAllMetadata).not.toHaveBeenCalled();
      cleanup();
    });

    it('treats a missing customUrl on a metadata-changed event as an empty string', () => {
      mocks.orgID = 'evt-no-url';
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      dispatchMetadataEvent({
        metadataKind: 'guest',
        metadataId: 'app-container:host:name:clear',
      });
      expect(result.guestMetadata()['app-container:host:name:clear']).toEqual({
        id: 'app-container:host:name:clear',
        customUrl: '',
      });
      cleanup();
    });
  });

  describe('refreshGuestMetadata arms', () => {
    it('swallows a refresh failure without throwing and leaves state empty', async () => {
      mocks.orgID = 'refresh-err';
      mocks.getAllMetadata.mockRejectedValue(new Error('boom'));
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      expect(Object.keys(result.guestMetadata())).toHaveLength(0);
      cleanup();
    });

    it('coerces a null API payload to an empty record on refresh', async () => {
      mocks.orgID = 'refresh-null';
      window.localStorage.setItem(
        keyFor('refresh-null'),
        JSON.stringify({
          'guest:seed': { id: 'guest:seed', customUrl: 'https://s.example' },
        }),
      );
      mocks.getAllMetadata.mockResolvedValue(null);
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      expect(result.guestMetadata()['guest:seed']?.customUrl).toBe('https://s.example');
      await flush();
      expect(result.guestMetadata()['guest:seed']).toBeUndefined();
      expect(result.guestMetadata()).toEqual({});
      cleanup();
    });
  });

  describe('org_switched subscription', () => {
    it('re-runs refresh when the org_switched event fires', async () => {
      mocks.orgID = 'orgswitch';
      const { cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      expect(mocks.orgSwitchedHandler.current).toBeTypeOf('function');
      mocks.getAllMetadata.mockClear();
      mocks.orgSwitchedHandler.current?.('org-2');
      await flush();
      expect(mocks.getAllMetadata).toHaveBeenCalledTimes(1);
      cleanup();
    });

    it('re-reads the cache under the new org scope on org_switched', async () => {
      mocks.orgID = 'scope-default';
      mocks.getAllMetadata.mockRejectedValue(new Error('no api'));
      window.localStorage.setItem(
        keyFor('org-target'),
        JSON.stringify({
          'guest:target': { id: 'guest:target', customUrl: 'https://t.example' },
        }),
      );
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      expect(result.guestMetadata()['guest:target']).toBeUndefined();
      mocks.orgSwitchedHandler.current?.('org-target');
      await vi.waitFor(() => {
        expect(result.guestMetadata()['guest:target']?.customUrl).toBe('https://t.example');
      });
      cleanup();
    });
  });

  describe('onMount teardown', () => {
    it('removes the metadata-changed window listener on cleanup', async () => {
      mocks.orgID = 'teardown';
      const { cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      mocks.getAllMetadata.mockClear();
      cleanup();
      dispatchMetadataEvent({ metadataKind: 'guest', metadataId: 'whatever:id' });
      await flush();
      expect(mocks.getAllMetadata).not.toHaveBeenCalled();
    });

    it('invokes the org_switched unsubscribe returned by eventBus.on on cleanup', async () => {
      mocks.orgID = 'teardown-org';
      const { cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      await flush();
      expect(mocks.unsubscribeOrgSwitched).not.toHaveBeenCalled();
      cleanup();
      expect(mocks.unsubscribeOrgSwitched).toHaveBeenCalledTimes(1);
    });
  });

  describe('localStorage persist queue (no requestIdleCallback -> setTimeout fallback)', () => {
    it('persists queued metadata to localStorage via the setTimeout fallback', () => {
      vi.stubGlobal('requestIdleCallback', undefined);
      vi.stubGlobal('cancelIdleCallback', undefined);
      vi.useFakeTimers();
      mocks.orgID = 'persist-ok';
      mocks.getAllMetadata.mockReturnValue(new Promise(() => {}));
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      result.handleCustomUrlUpdate('guest:persist', 'https://p.example');
      vi.advanceTimersByTime(1);
      const raw = window.localStorage.getItem(keyFor('persist-ok'));
      expect(raw).not.toBeNull();
      expect(raw).toContain('guest:persist');
      expect(raw).toContain('https://p.example');
      cleanup();
    });

    it('swallows a localStorage.setItem failure during persist without throwing', () => {
      vi.stubGlobal('requestIdleCallback', undefined);
      vi.stubGlobal('cancelIdleCallback', undefined);
      vi.useFakeTimers();
      mocks.orgID = 'persist-err';
      mocks.getAllMetadata.mockReturnValue(new Promise(() => {}));
      const setItemSpy = vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
        throw new Error('quota exceeded');
      });
      const { result, cleanup } = renderHook(() => useWorkloadGuestMetadataState());
      expect(() => {
        result.handleCustomUrlUpdate('guest:persist-err', 'https://pe.example');
        vi.advanceTimersByTime(1);
      }).not.toThrow();
      setItemSpy.mockRestore();
      cleanup();
    });
  });
});
