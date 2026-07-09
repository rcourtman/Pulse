import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'solid-js';
import {
  createLocalStorageBooleanSignal,
  createLocalStorageNumberSignal,
  createLocalStorageStringSignal,
  STORAGE_KEYS,
} from '@/utils/localStorage';

const SYNC_EVENT = 'pulse-localstorage-sync';

// createEffect defers its first run to a microtask; await this after setting up
// or mutating a signal so the sync-to-storage effect has flushed.
const flush = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

type AnySignal<T> = [() => T, (v: T) => void];

const disposables: Array<() => void> = [];

function makeSignal<T>(factory: () => AnySignal<T>): {
  value: () => T;
  setValue: (v: T) => void;
  setValueAny: (v: unknown) => void;
  dispose: () => void;
} {
  let dispose!: () => void;
  let value!: () => T;
  let setValue!: (v: T) => void;
  createRoot((d) => {
    dispose = d;
    [value, setValue] = factory() as AnySignal<T>;
  });
  const disposeFn = () => dispose();
  disposables.push(disposeFn);
  return {
    value: () => value(),
    setValue: (v: T) => setValue(v),
    setValueAny: (v: unknown) => (setValue as (v: unknown) => void)(v),
    dispose: disposeFn,
  };
}

function dispatchCustomSync(key: string, value: string | null) {
  window.dispatchEvent(
    new CustomEvent(SYNC_EVENT, { detail: { key, value } }),
  );
}

// jsdom's StorageEvent constructor rejects non-jsdom Storage instances for
// storageArea (the shared in-memory storage from setup is a plain class). The
// handler only reads key/newValue/storageArea, so a plain Event with those
// props is sufficient and faithful for exercising the handler logic.
function dispatchStorageEvent(key: string, newValue: string | null, storageArea: Storage | null) {
  const event = new Event('storage');
  Object.defineProperty(event, 'key', { value: key, configurable: true });
  Object.defineProperty(event, 'newValue', { value: newValue, configurable: true });
  Object.defineProperty(event, 'storageArea', { value: storageArea, configurable: true });
  window.dispatchEvent(event);
}

describe('localStorage signals', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    while (disposables.length) disposables.pop()!();
    vi.restoreAllMocks();
  });

  describe('STORAGE_KEYS', () => {
    it('exposes the expected string keys', () => {
      expect(STORAGE_KEYS.AUTH).toBe('pulse_auth');
      expect(STORAGE_KEYS.AUTH_USER).toBe('pulse_auth_user');
      expect(STORAGE_KEYS.THEME_PREFERENCE).toBe('pulseThemePreference');
      expect(STORAGE_KEYS.SIDEBAR_COLLAPSED).toBe('sidebarCollapsed');
      expect(STORAGE_KEYS.TEMPERATURE_UNIT).toBe('temperatureUnit');
      expect(STORAGE_KEYS.DEBUG_MODE).toBe('pulse_debug_mode');
      expect(STORAGE_KEYS.AUDIT_PAGE_SIZE).toBe('pulse-audit-page-size');
    });

    it('exposes a broad set of keys across feature areas', () => {
      expect(Object.keys(STORAGE_KEYS).length).toBeGreaterThan(40);
      expect(STORAGE_KEYS.WORKLOADS_SEARCH_HISTORY).toBe('workloadsSearchHistory');
      expect(STORAGE_KEYS.GITHUB_STAR_SNOOZED_UNTIL).toBe('pulse-github-star-snoozed-until');
    });
  });

  describe('createLocalStorageStringSignal', () => {
    it('uses the default value when storage is empty', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-key', 'default'));

      expect(sig.value()).toBe('default');
    });

    it('writes the default value to storage once the effect flushes', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-key', 'default'));
      await flush();

      expect(localStorage.getItem('str-key')).toBe('default');
      sig.dispose();
    });

    it('defaults to an empty string when no default is provided', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-key'));
      await flush();

      expect(sig.value()).toBe('');
      expect(localStorage.getItem('str-key')).toBe('');
    });

    it('reads a stored value synchronously', () => {
      localStorage.setItem('str-key', 'stored');

      const sig = makeSignal(() => createLocalStorageStringSignal('str-key', 'default'));

      expect(sig.value()).toBe('stored');
    });

    it('writes to localStorage when the value changes', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-key', 'default'));
      await flush();

      sig.setValue('new-value');
      await flush();

      expect(sig.value()).toBe('new-value');
      expect(localStorage.getItem('str-key')).toBe('new-value');
    });

    it('persists an empty string', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-key', 'default'));
      await flush();

      sig.setValue('');
      await flush();

      expect(localStorage.getItem('str-key')).toBe('');
    });

    // NOTE: setting null/undefined does NOT persist a removal. The effect removes
    // the item and broadcasts null, but the signal's own same-tab custom-event
    // listener receives that null broadcast and reverts the signal to its
    // defaultValue (applyRaw maps null -> defaultValue). The effect then writes
    // the default back to storage. See GLM_REPORT.md (suspected bug).
    it('reverts to the default when set to null (current behavior)', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-key', 'default'));
      await flush();
      sig.setValue('changed');
      await flush();
      expect(localStorage.getItem('str-key')).toBe('changed');

      sig.setValueAny(null);
      await flush();

      expect(sig.value()).toBe('default');
      expect(localStorage.getItem('str-key')).toBe('default');
    });

    it('reverts to the default when set to undefined (current behavior)', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-key', 'default'));
      await flush();
      sig.setValue('changed');
      await flush();
      expect(localStorage.getItem('str-key')).toBe('changed');

      sig.setValueAny(undefined);
      await flush();

      expect(sig.value()).toBe('default');
      expect(localStorage.getItem('str-key')).toBe('default');
    });

    it('broadcasts a sync event when the value changes', async () => {
      const details: Array<{ key: string; value: string | null }> = [];
      const handler = (e: Event) => {
        const evt = e as CustomEvent<{ key: string; value: string | null }>;
        details.push(evt.detail);
      };
      window.addEventListener(SYNC_EVENT, handler);

      const sig = makeSignal(() => createLocalStorageStringSignal('broadcast-key', 'def'));
      await flush();
      expect(details.at(-1)).toEqual({ key: 'broadcast-key', value: 'def' });

      sig.setValue('updated');
      await flush();
      expect(details.at(-1)).toEqual({ key: 'broadcast-key', value: 'updated' });

      sig.setValueAny(null);
      await flush();
      // A null broadcast is emitted (the clear intent), but the same-tab
      // self-sync reverts the signal to its default (suspected bug).
      expect(details).toContainEqual({ key: 'broadcast-key', value: null });
      expect(details.at(-1)).toEqual({ key: 'broadcast-key', value: 'def' });

      window.removeEventListener(SYNC_EVENT, handler);
    });
  });

  describe('createLocalStorageBooleanSignal', () => {
    const parseCases: Array<[string, boolean]> = [
      ['true', true],
      ['false', false],
      ['1', false],
      ['0', false],
      ['TRUE', false],
      ['True', false],
      ['', false],
      ['anything', false],
    ];

    it.each(parseCases)('parses stored %j as %s', (stored, expected) => {
      localStorage.setItem('bool-key', stored);

      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-key', false));

      expect(sig.value()).toBe(expected);
    });

    it('defaults to false when no default is given and storage is empty', async () => {
      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-key'));
      await flush();

      expect(sig.value()).toBe(false);
      expect(localStorage.getItem('bool-key')).toBe('false');
    });

    it('defaults to true when provided and storage is empty', async () => {
      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-key', true));
      await flush();

      expect(sig.value()).toBe(true);
      expect(localStorage.getItem('bool-key')).toBe('true');
    });

    it.each([
      [true, 'true'],
      [false, 'false'],
    ])('writes %s to storage as %j', async (next, expected) => {
      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-key', false));
      await flush();

      sig.setValue(next);
      await flush();

      expect(localStorage.getItem('bool-key')).toBe(expected);
    });

    it('reverts to the default when set to null (current behavior)', async () => {
      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-key', true));
      await flush();
      expect(localStorage.getItem('bool-key')).toBe('true');

      sig.setValueAny(null);
      await flush();

      expect(sig.value()).toBe(true);
      expect(localStorage.getItem('bool-key')).toBe('true');
    });
  });

  describe('createLocalStorageNumberSignal', () => {
    const defaultParseCases: Array<[string, number]> = [
      ['42', 42],
      ['3.14', 3.14],
      ['-5', -5],
      ['0', 0],
      ['1e3', 1000],
      ['0x10', 16],
      ['', 0],
      ['  ', 0],
    ];

    it.each(defaultParseCases)('parses stored %j as %s', (stored, expected) => {
      localStorage.setItem('num-key', stored);

      const sig = makeSignal(() => createLocalStorageNumberSignal('num-key', 7));

      expect(sig.value()).toBe(expected);
    });

    it.each([
      ['abc'],
      ['Infinity'],
      ['NaN'],
    ])('falls back to the default when stored %j is not finite', (stored) => {
      localStorage.setItem('num-key', stored);

      const sig = makeSignal(() => createLocalStorageNumberSignal('num-key', 7));

      expect(sig.value()).toBe(7);
    });

    it('heals an invalid stored value by writing the default back to storage', async () => {
      localStorage.setItem('num-key', 'abc');

      const sig = makeSignal(() => createLocalStorageNumberSignal('num-key', 7));
      await flush();

      expect(sig.value()).toBe(7);
      expect(localStorage.getItem('num-key')).toBe('7');
    });

    it('defaults to 0 when no default is given and storage is empty', async () => {
      const sig = makeSignal(() => createLocalStorageNumberSignal('num-key'));
      await flush();

      expect(sig.value()).toBe(0);
      expect(localStorage.getItem('num-key')).toBe('0');
    });

    it.each([
      [99, '99'],
      [0, '0'],
      [-3.5, '-3.5'],
    ])('writes %s to storage as %j', async (next, expected) => {
      const sig = makeSignal(() => createLocalStorageNumberSignal('num-key', 0));
      await flush();

      sig.setValue(next);
      await flush();

      expect(localStorage.getItem('num-key')).toBe(expected);
    });

    it('reverts to the default when set to null (current behavior)', async () => {
      const sig = makeSignal(() => createLocalStorageNumberSignal('num-key', 5));
      await flush();
      expect(localStorage.getItem('num-key')).toBe('5');

      sig.setValueAny(null);
      await flush();

      expect(sig.value()).toBe(5);
      expect(localStorage.getItem('num-key')).toBe('5');
    });
  });

  describe('cross-instance sync', () => {
    it('keeps two signals for the same key in sync', async () => {
      const a = makeSignal(() => createLocalStorageStringSignal('sync-key', 'def'));
      await flush();
      const b = makeSignal(() => createLocalStorageStringSignal('sync-key', 'def'));
      await flush();

      expect(a.value()).toBe('def');
      expect(b.value()).toBe('def');

      a.setValue('from-a');
      await flush();
      expect(b.value()).toBe('from-a');
      expect(localStorage.getItem('sync-key')).toBe('from-a');

      b.setValue('from-b');
      await flush();
      expect(a.value()).toBe('from-b');
    });
  });

  describe('storage event sync', () => {
    it('updates the signal from a storage event for its key', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('se-key', 'def'));
      await flush();

      dispatchStorageEvent('se-key', 'from-storage', window.localStorage);

      expect(sig.value()).toBe('from-storage');
    });

    it('ignores storage events for other keys', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('se-key', 'def'));
      await flush();

      dispatchStorageEvent('other-key', 'nope', window.localStorage);

      expect(sig.value()).toBe('def');
    });

    it('ignores storage events from a different storage area', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('se-key', 'def'));
      await flush();

      dispatchStorageEvent('se-key', 'nope', null);

      expect(sig.value()).toBe('def');
    });

    it('reverts to the default when a storage event clears the key', async () => {
      localStorage.setItem('se-key', 'present');
      const sig = makeSignal(() => createLocalStorageStringSignal('se-key', 'def'));
      await flush();
      expect(sig.value()).toBe('present');

      dispatchStorageEvent('se-key', null, window.localStorage);

      expect(sig.value()).toBe('def');
    });
  });

  describe('custom sync event', () => {
    it('updates the signal from a custom sync event for its key', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('cs-key', 'def'));
      await flush();

      dispatchCustomSync('cs-key', 'via-custom');

      expect(sig.value()).toBe('via-custom');
    });

    it('ignores custom sync events for other keys', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('cs-key', 'def'));
      await flush();

      dispatchCustomSync('other-key', 'nope');

      expect(sig.value()).toBe('def');
    });

    it('ignores custom sync events without detail', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('cs-key', 'def'));
      await flush();

      window.dispatchEvent(new CustomEvent(SYNC_EVENT));

      expect(sig.value()).toBe('def');
    });

    it('reverts to the default when the custom event value is null', async () => {
      localStorage.setItem('cs-key', 'present');
      const sig = makeSignal(() => createLocalStorageStringSignal('cs-key', 'def'));
      await flush();
      expect(sig.value()).toBe('present');

      dispatchCustomSync('cs-key', null);

      expect(sig.value()).toBe('def');
    });

    it('does not change when the custom event carries the current value', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('cs-key', 'def'));
      await flush();

      dispatchCustomSync('cs-key', 'def');

      expect(sig.value()).toBe('def');
    });
  });

  describe('cleanup', () => {
    it('stops listening for sync events after disposal', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('cu-key', 'def'));
      await flush();
      expect(sig.value()).toBe('def');

      sig.dispose();

      dispatchCustomSync('cu-key', 'after-dispose');
      expect(sig.value()).toBe('def');
    });
  });

  describe('broadcast error handling', () => {
    it('does not throw when dispatching the sync event fails', async () => {
      const dispatchSpy = vi.spyOn(window, 'dispatchEvent').mockImplementation((event: Event) => {
        if (event.type === SYNC_EVENT) {
          throw new Error('dispatch failed');
        }
        return true;
      });

      const sig = makeSignal(() => createLocalStorageStringSignal('err-key', 'def'));
      await flush();
      // setItem runs before the (swallowed) broadcast error
      expect(localStorage.getItem('err-key')).toBe('def');

      sig.setValue('next');
      await flush();
      expect(localStorage.getItem('err-key')).toBe('next');

      dispatchSpy.mockRestore();
    });
  });
});
