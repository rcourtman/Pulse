import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  RESOURCE_METADATA_CHANGED_EVENT,
  dispatchResourceMetadataChanged,
} from '@/utils/resourceMetadataEvents';
import type { ResourceMetadataChangedDetail } from '@/utils/resourceMetadataEvents';

// Branch-coverage companion for resourceMetadataEvents.ts. The module exposes a
// single side-effecting function `dispatchResourceMetadataChanged` plus the
// exported event-name constant; there is no sibling unit test for it, so this
// file drives every branch of the guard (`typeof window === 'undefined'` and
// `!detail.metadataId`), the happy-path `try` success arm across all three
// `metadataKind` values and the optional `customUrl`, and the defensive
// `catch` arm that swallows dispatch failures. Assertions are concrete event
// shapes / spy call counts rather than truthiness.

// Capture the real jsdom window once, at module load, so listeners stay wired
// to a stable object even when an individual test stubs the global `window`
// binding to undefined. The function under test reads the global `window`;
// when that binding is the real jsdom window it is the SAME object as `win`,
// so dispatches still reach listeners wired here. Spies are created locally in
// each test (matching the sibling convention) so their precise return types
// are inferred without a fragile field annotation.
const win: Window & typeof globalThis = window;

describe('dispatchResourceMetadataChanged', () => {
  let listener: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    listener = vi.fn();
    win.addEventListener(RESOURCE_METADATA_CHANGED_EVENT, listener);
  });

  afterEach(() => {
    // Restore globals FIRST so a stubbed-out `window`/`CustomEvent` is back in
    // place before we touch listeners; otherwise the next line could throw and
    // leak into the following test.
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    win.removeEventListener(RESOURCE_METADATA_CHANGED_EVENT, listener);
  });

  describe('guard early-return branches (no event is dispatched)', () => {
    it('returns without dispatching when `typeof window === "undefined"` (first guard arm)', () => {
      const dispatchSpy = vi.spyOn(win, 'dispatchEvent');
      // The guard's left operand short-circuits before `!detail.metadataId`
      // is even evaluated, so a perfectly valid detail still yields no dispatch.
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'agent',
        metadataId: 'agent-7',
      };
      vi.stubGlobal('window', undefined);

      dispatchResourceMetadataChanged(detail);

      expect(dispatchSpy).not.toHaveBeenCalled();
      expect(listener).not.toHaveBeenCalled();
    });

    it('returns without dispatching when metadataId is the empty string (`!detail.metadataId` arm)', () => {
      const dispatchSpy = vi.spyOn(win, 'dispatchEvent');
      // `typeof window` is defined, but the right guard operand is true for '',
      // so the function bails before constructing a CustomEvent.
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'guest',
        metadataId: '',
      };

      dispatchResourceMetadataChanged(detail);

      expect(dispatchSpy).not.toHaveBeenCalled();
      expect(listener).not.toHaveBeenCalled();
    });

    it('treats a whitespace-only metadataId as truthy and dispatches (documents that the guard is not a trim check)', () => {
      // The guard uses a raw truthiness check on metadataId, NOT a trim check,
      // so '  ' is a non-empty string and clears the guard. Pinning the actual
      // (arguably surprising) behaviour: the event is dispatched verbatim.
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'docker',
        metadataId: '  ',
      };

      dispatchResourceMetadataChanged(detail);

      expect(listener).toHaveBeenCalledTimes(1);
      const event = listener.mock.calls[0][0] as CustomEvent<ResourceMetadataChangedDetail>;
      expect(event.type).toBe(RESOURCE_METADATA_CHANGED_EVENT);
      expect(event.detail).toStrictEqual(detail);
    });
  });

  describe('happy-path try-success arm (event is dispatched with the exact detail)', () => {
    it.each([
      ['agent', { metadataKind: 'agent', metadataId: 'agent-1' } as const],
      ['guest', { metadataKind: 'guest', metadataId: 'vm-100' } as const],
      ['docker', { metadataKind: 'docker', metadataId: 'container-abc' } as const],
    ])('dispatches a CustomEvent for metadataKind=%s without customUrl', (_label, detail) => {
      const dispatchSpy = vi.spyOn(win, 'dispatchEvent');

      dispatchResourceMetadataChanged(detail);

      expect(dispatchSpy).toHaveBeenCalledTimes(1);
      expect(listener).toHaveBeenCalledTimes(1);
      const event = listener.mock.calls[0][0] as CustomEvent<ResourceMetadataChangedDetail>;
      // The exported constant is threaded through verbatim as the event type.
      expect(event.type).toBe(RESOURCE_METADATA_CHANGED_EVENT);
      // The detail object is forwarded by reference, including metadataKind.
      expect(event.detail).toStrictEqual(detail);
      // A fresh CustomEvent instance is constructed (not, e.g., a reused one).
      expect(event).toBeInstanceOf(CustomEvent);
    });

    it('forwards an optional customUrl when present on the detail', () => {
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'agent',
        metadataId: 'agent-2',
        customUrl: 'https://host:9090/agent/agent-2',
      };

      dispatchResourceMetadataChanged(detail);

      expect(listener).toHaveBeenCalledTimes(1);
      const event = listener.mock.calls[0][0] as CustomEvent<ResourceMetadataChangedDetail>;
      expect(event.detail).toStrictEqual(detail);
      expect(event.detail.customUrl).toBe('https://host:9090/agent/agent-2');
    });

    it('omits customUrl from the forwarded detail when it was not supplied', () => {
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'guest',
        metadataId: 'vm-101',
      };

      dispatchResourceMetadataChanged(detail);

      const event = listener.mock.calls[0][0] as CustomEvent<ResourceMetadataChangedDetail>;
      // toStrictEqual asserts the key is genuinely absent, not just undefined.
      expect(event.detail).toStrictEqual({ metadataKind: 'guest', metadataId: 'vm-101' });
      expect('customUrl' in event.detail).toBe(false);
    });

    it('passes the detail object by reference (no defensive clone)', () => {
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'docker',
        metadataId: 'container-ref',
      };

      dispatchResourceMetadataChanged(detail);

      const event = listener.mock.calls[0][0] as CustomEvent<ResourceMetadataChangedDetail>;
      // Pin the by-reference forwarding: the same object identity reaches the
      // listener. A future clone would break this test deliberately.
      expect(event.detail).toBe(detail);
    });
  });

  describe('catch arm (dispatch failures are swallowed, not re-thrown)', () => {
    it('swallows a throw from window.dispatchEvent and does not propagate', () => {
      const dispatchSpy = vi.spyOn(win, 'dispatchEvent').mockImplementation(() => {
        throw new Error('dispatch boom');
      });
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'agent',
        metadataId: 'agent-3',
      };

      // The catch block is a no-op; the function must resolve to undefined.
      const result = dispatchResourceMetadataChanged(detail);

      expect(result).toBeUndefined();
      // The throw happens *inside* dispatchEvent, proving we reached the try
      // body — i.e. the guard did NOT short-circuit.
      expect(dispatchSpy).toHaveBeenCalledTimes(1);
      // No real listener receives the event because dispatchEvent threw before
      // any delivery could occur.
      expect(listener).not.toHaveBeenCalled();
    });

    it('swallows a throw from the CustomEvent constructor itself', () => {
      const dispatchSpy = vi.spyOn(win, 'dispatchEvent');
      // Drive the catch via the other throwable site: make `new CustomEvent`
      // reject. This confirms the whole try expression is guarded, not just
      // the dispatchEvent call.
      const original = globalThis.CustomEvent;
      vi.stubGlobal(
        'CustomEvent',
        class {
          constructor() {
            throw new Error('ctor boom');
          }
        },
      );
      const detail: ResourceMetadataChangedDetail = {
        metadataKind: 'guest',
        metadataId: 'vm-102',
      };

      expect(() => dispatchResourceMetadataChanged(detail)).not.toThrow();
      // dispatchEvent is never reached because the constructor threw first.
      expect(dispatchSpy).not.toHaveBeenCalled();
      expect(listener).not.toHaveBeenCalled();

      vi.stubGlobal('CustomEvent', original);
    });
  });
});
