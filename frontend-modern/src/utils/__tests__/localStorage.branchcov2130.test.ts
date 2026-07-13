/**
 * Branch-coverage tests for localStorage.ts — second pass.
 *
 * Scope: the parse/stringify ternary arms of the module-private
 * `createLocalStorageSignal` that are reached through the three exported
 * signal factories. The sibling localStorage.test.ts exercises the default-
 * value, null/undefined-revert, cross-instance-sync, cleanup, and broadcast-
 * error paths; this file focuses ONLY on the arms the v8 branch scan flagged:
 *
 *   - L39 initial-read parse arm: `stored !== null ? (parse ? parse(stored) : ...) : ...`
 *     Driven by seeding localStorage BEFORE creating a signal whose factory
 *     supplies a `parse` fn, so the synchronous first read runs `parse(stored)`.
 *
 *   - L46 reload/refresh parse arm (inside `applyRaw`):
 *     `raw !== null ? (parse ? parse(raw) : ...) : ...`
 *     Driven by dispatching a `storage` event or a `pulse-localstorage-sync`
 *     custom event carrying a non-null raw value to an already-created signal
 *     whose factory supplies a `parse` fn, so the handler runs `parse(raw)`.
 *
 *   - L78 write-path stringify arm: `stringify ? stringify(val) : String(val)`
 *     Driven by calling a signal's setter (factory supplies a `stringify` fn)
 *     and asserting the persisted localStorage string after the effect flushes.
 *
 * The module-private `createLocalStorageSignal` is not exported; every arm
 * below is reached through the real exported factories. The `typeof window ===
 * 'undefined'` SSR guard (L11/L44) is not cleanly reachable in jsdom and is
 * intentionally not targeted here.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'solid-js';
import {
  createLocalStorageBooleanSignal,
  createLocalStorageNumberSignal,
  createLocalStorageStringSignal,
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

describe('localStorage signal factories — parse/stringify branch coverage', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    while (disposables.length) disposables.pop()!();
    vi.restoreAllMocks();
  });

  // ---------------------------------------------------------------- L39 -----
  describe('createLocalStorageNumberSignal — L39 initial-read parse arm', () => {
    const finiteParseCases: Array<[string, number]> = [
      ['42', 42],
      ['0', 0],
      ['-5', -5],
      ['3.14', 3.14],
      ['1e3', 1000],
      ['0x10', 16],
      ['  7  ', 7], // Number() trims surrounding whitespace
      [String(Number.MAX_SAFE_INTEGER), Number.MAX_SAFE_INTEGER],
    ];

    it.each(finiteParseCases)(
      'runs parse(stored) on first read: %j -> %s',
      (stored, expected) => {
        localStorage.setItem('num-init', stored);

        const sig = makeSignal(() => createLocalStorageNumberSignal('num-init', 7));

        expect(sig.value()).toBe(expected);
      },
    );

    it.each([['abc'], ['Infinity'], ['-Infinity'], ['NaN'], ['-0x10']])(
      'runs parse(stored) and falls back to the default when %j is not finite',
      (stored) => {
        // Number('-0x10') is NaN: the Number() hex grammar rejects a leading
        // minus sign, so the parse fn's Number.isFinite guard fires.
        localStorage.setItem('num-init', stored);

        const sig = makeSignal(() => createLocalStorageNumberSignal('num-init', 7));

        expect(sig.value()).toBe(7);
      },
    );
  });

  // ---------------------------------------------------------------- L46 -----
  describe('createLocalStorageNumberSignal — L46 reload parse arm (applyRaw)', () => {
    it('runs parse(raw) when a storage event delivers a finite numeric string', async () => {
      const sig = makeSignal(() => createLocalStorageNumberSignal('num-reload', 7));
      await flush();
      expect(sig.value()).toBe(7);

      dispatchStorageEvent('num-reload', '256', window.localStorage);

      expect(sig.value()).toBe(256);
    });

    it('runs parse(raw) and falls back to the default when a storage event delivers a non-finite value', async () => {
      const sig = makeSignal(() => createLocalStorageNumberSignal('num-reload', 9));
      await flush();

      // Move off the default first so the fallback is observable.
      dispatchStorageEvent('num-reload', '100', window.localStorage);
      expect(sig.value()).toBe(100);

      dispatchStorageEvent('num-reload', 'Infinity', window.localStorage);

      expect(sig.value()).toBe(9);
    });

    it('runs parse(raw) when a custom sync event delivers a finite numeric string', async () => {
      const sig = makeSignal(() => createLocalStorageNumberSignal('num-reload', 7));
      await flush();

      dispatchCustomSync('num-reload', '4096');

      expect(sig.value()).toBe(4096);
    });

    it('runs parse(raw) and falls back to the default when a custom sync event delivers NaN', async () => {
      const sig = makeSignal(() => createLocalStorageNumberSignal('num-reload', 11));
      await flush();

      dispatchCustomSync('num-reload', '50');
      expect(sig.value()).toBe(50);

      dispatchCustomSync('num-reload', 'NaN');

      expect(sig.value()).toBe(11);
    });
  });

  // ---------------------------------------------------------------- L78 -----
  describe('createLocalStorageNumberSignal — L78 write-path stringify arm', () => {
    const stringifyCases: Array<[number, string]> = [
      [0, '0'],
      [-3.5, '-3.5'],
      [99, '99'],
      [Number.MAX_SAFE_INTEGER, String(Number.MAX_SAFE_INTEGER)],
      [1e21, '1e+21'], // String(1e21) uses exponential notation
    ];

    it.each(stringifyCases)(
      'runs stringify(val) on write: %s -> %j persisted',
      async (next, expected) => {
        const sig = makeSignal(() => createLocalStorageNumberSignal('num-write', 0));
        await flush();

        sig.setValue(next);
        await flush();

        expect(localStorage.getItem('num-write')).toBe(expected);
      },
    );
  });

  // ---------------------------------------------------------------- L39 -----
  describe('createLocalStorageBooleanSignal — L39 initial-read parse arm', () => {
    const boolParseCases: Array<[string, boolean]> = [
      ['true', true],
      ['false', false],
      ['1', false],
      ['0', false],
      ['TRUE', false], // parse uses strict val === 'true' (case-sensitive)
      ['True', false],
      ['', false],
      [' true ', false], // strict equality: surrounding whitespace fails the match
      ['true\n', false], // strict equality: trailing newline fails the match
      ['anything', false],
    ];

    it.each(boolParseCases)(
      'runs parse(stored) on first read: %j -> %s',
      (stored, expected) => {
        localStorage.setItem('bool-init', stored);

        const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-init', false));

        expect(sig.value()).toBe(expected);
      },
    );
  });

  // ---------------------------------------------------------------- L46 -----
  describe('createLocalStorageBooleanSignal — L46 reload parse arm (applyRaw)', () => {
    it('runs parse(raw) when a storage event delivers "true"', async () => {
      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-reload', false));
      await flush();
      expect(sig.value()).toBe(false);

      dispatchStorageEvent('bool-reload', 'true', window.localStorage);

      expect(sig.value()).toBe(true);
    });

    it('runs parse(raw) when a storage event delivers a non-"true" string (parses to false)', async () => {
      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-reload', true));
      await flush();
      expect(sig.value()).toBe(true);

      dispatchStorageEvent('bool-reload', '1', window.localStorage);

      expect(sig.value()).toBe(false);
    });

    it('runs parse(raw) when a custom sync event delivers "true"', async () => {
      const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-reload', false));
      await flush();

      dispatchCustomSync('bool-reload', 'true');

      expect(sig.value()).toBe(true);
    });
  });

  // ---------------------------------------------------------------- L78 -----
  describe('createLocalStorageBooleanSignal — L78 write-path stringify arm', () => {
    const boolStringifyCases: Array<[boolean, string]> = [
      [true, 'true'],
      [false, 'false'],
    ];

    it.each(boolStringifyCases)(
      'runs stringify(val) on write: %s -> %j persisted',
      async (next, expected) => {
        const sig = makeSignal(() => createLocalStorageBooleanSignal('bool-write', false));
        await flush();

        sig.setValue(next);
        await flush();

        expect(localStorage.getItem('bool-write')).toBe(expected);
      },
    );
  });

  // ---------------------------------------------------------------- L39 -----
  describe('createLocalStorageStringSignal — L39 initial-read parse arm', () => {
    const stringParseCases: Array<[string, string]> = [
      ['stored', 'stored'],
      ['', ''],
      ['  spaces  ', '  spaces  '], // String(val) is identity — whitespace is preserved
      ['héllo-wörld', 'héllo-wörld'], // unicode is preserved
      ['multi\nline', 'multi\nline'], // newlines are preserved
    ];

    it.each(stringParseCases)(
      'runs parse(stored)=String(stored) on first read: %j -> %j',
      (stored, expected) => {
        localStorage.setItem('str-init', stored);

        const sig = makeSignal(() => createLocalStorageStringSignal('str-init', 'default'));

        expect(sig.value()).toBe(expected);
      },
    );
  });

  // ---------------------------------------------------------------- L46 -----
  describe('createLocalStorageStringSignal — L46 reload parse arm (applyRaw)', () => {
    it('runs parse(raw) when a storage event delivers a new string', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-reload', 'def'));
      await flush();

      dispatchStorageEvent('str-reload', 'from-storage', window.localStorage);

      expect(sig.value()).toBe('from-storage');
    });

    it('runs parse(raw) when a custom sync event delivers a new string', async () => {
      const sig = makeSignal(() => createLocalStorageStringSignal('str-reload', 'def'));
      await flush();

      dispatchCustomSync('str-reload', 'via-custom');

      expect(sig.value()).toBe('via-custom');
    });
  });

  // ---------------------------------------------------------------- L78 -----
  describe('createLocalStorageStringSignal — L78 write-path stringify arm', () => {
    const stringStringifyCases: Array<[string, string]> = [
      ['new-value', 'new-value'],
      ['', ''],
      ['héllo', 'héllo'],
    ];

    it.each(stringStringifyCases)(
      'runs stringify(val)=String(val) on write: %j -> %j persisted',
      async (next, expected) => {
        const sig = makeSignal(() => createLocalStorageStringSignal('str-write', 'def'));
        await flush();

        sig.setValue(next);
        await flush();

        expect(localStorage.getItem('str-write')).toBe(expected);
      },
    );
  });
});
