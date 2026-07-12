/**
 * Branch-coverage tests for `url.ts` — second pass (0712c).
 *
 * Scope: ONLY these still-uncovered named functions:
 *   getPulseBaseUrl, getPulseWebSocketUrl, initKioskMode, isKioskMode,
 *   getKioskModePreference, setKioskMode, getPulseHostname
 *
 * The sibling `url.test.ts` exercises the happy paths. This file drives every
 * remaining branch: SSR early-returns, storage try/catch arms, the
 * `origin === 'null'` rebuild path in getPulseBaseUrl, the unparseable-origin
 * fallback in getPulseHostname / getPulseWebSocketUrl, the >16KB auth-length
 * guard, non-object / malformed-JSON auth payloads, and the kiosk URL-param
 * `false`/absent/`true` arms.
 *
 * SSR arms are reached with `vi.stubGlobal('window', undefined)` (the module
 * guards on `typeof window === 'undefined'` before touching window), restored
 * via `vi.unstubAllGlobals()` in afterEach. window.location / sessionStorage
 * are replaced with plain mocks exactly as the sibling test does.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  getPulseBaseUrl,
  getPulseHostname,
  getPulseWebSocketUrl,
  getKioskModePreference,
  initKioskMode,
  isKioskMode,
  setKioskMode,
  subscribeToKioskMode,
} from '../url';

const FALLBACK_BASE_URL = 'http://localhost:7655';

describe('url.ts branch coverage (0712c)', () => {
  const originalLocation = window.location;
  const originalSessionStorage = window.sessionStorage;

  beforeEach(() => {
    // Replace window.location with a plain writable object (same pattern as
    // the sibling url.test.ts). origin is a data prop here, so per-test code
    // can freely reassign it via `(window.location as any).origin = ...`.
    delete (window as any).location;
    window.location = {
      origin: 'http://localhost:3000',
      protocol: 'http:',
      hostname: 'localhost',
      port: '3000',
      host: 'localhost:3000',
      search: '',
    } as any;

    const storage: Record<string, string> = {};
    (window as any).sessionStorage = {
      getItem: vi.fn((key: string) => storage[key] ?? null),
      setItem: vi.fn((key: string, value: string) => {
        storage[key] = value;
      }),
      removeItem: vi.fn((key: string) => {
        delete storage[key];
      }),
      clear: vi.fn(() => {
        for (const key of Object.keys(storage)) delete storage[key];
      }),
    } as any;
  });

  afterEach(() => {
    // Restore the stubbed `window` first so reassigning location/sessionStorage
    // targets the real global, then restore originals + reset mocks.
    vi.unstubAllGlobals();
    (window as any).location = originalLocation;
    (window as any).sessionStorage = originalSessionStorage;
    vi.restoreAllMocks();
  });

  describe('getPulseBaseUrl', () => {
    it('returns the fallback when window is undefined (SSR arm of the guard)', () => {
      vi.stubGlobal('window', undefined);
      expect(getPulseBaseUrl()).toBe(FALLBACK_BASE_URL);
    });

    it('returns the fallback when window.location is missing (!window.location arm)', () => {
      (window as any).location = undefined;
      expect(getPulseBaseUrl()).toBe(FALLBACK_BASE_URL);
    });

    it('returns a non-default origin verbatim (truthy, !== "null" arm)', () => {
      (window.location as any).origin = 'https://ui.pulse.local:9000';
      expect(getPulseBaseUrl()).toBe('https://ui.pulse.local:9000');
    });

    it('rebuilds origin WITH port when location.origin is the string "null"', () => {
      (window.location as any).origin = 'null';
      (window.location as any).protocol = 'http:';
      (window.location as any).hostname = 'example.com';
      (window.location as any).port = '8080';
      expect(getPulseBaseUrl()).toBe('http://example.com:8080');
    });

    it('rebuilds origin WITHOUT port when location.origin is "null" and port is empty', () => {
      (window.location as any).origin = 'null';
      (window.location as any).protocol = 'https:';
      (window.location as any).hostname = 'host.io';
      (window.location as any).port = '';
      expect(getPulseBaseUrl()).toBe('https://host.io');
    });

    it('returns the fallback when origin is "null" and protocol/hostname are missing', () => {
      (window.location as any).origin = 'null';
      (window.location as any).protocol = '';
      (window.location as any).hostname = '';
      expect(getPulseBaseUrl()).toBe(FALLBACK_BASE_URL);
    });
  });

  describe('getPulseHostname', () => {
    it('returns the parsed hostname when origin is a valid URL', () => {
      (window.location as any).origin = 'https://nav.pulse.io:7000';
      expect(getPulseHostname()).toBe('nav.pulse.io');
    });

    it('returns "localhost" when the origin cannot be parsed (origin?.hostname || "localhost" fallback)', () => {
      // 'not-a-valid-origin' is truthy and !== 'null' so getPulseBaseUrl returns
      // it verbatim, but `new URL(...)` throws -> getPulseOriginUrl() === null.
      (window.location as any).origin = 'not-a-valid-origin';
      expect(getPulseHostname()).toBe('localhost');
    });
  });

  describe('getPulseWebSocketUrl', () => {
    it('returns ws://localhost<path> when the origin is unparseable (!origin arm)', () => {
      (window.location as any).origin = 'not-a-valid-origin';
      expect(getPulseWebSocketUrl('/events')).toBe('ws://localhost/events');
    });

    it('normalizes a path without a leading slash into a ws:// URL (http -> ws)', () => {
      // origin is the default http://localhost:3000; no auth stored.
      expect(getPulseWebSocketUrl('stream')).toBe('ws://localhost:3000/stream');
    });

    it('omits the token when the stored pulse_auth payload exceeds 16KB (>MAX guard)', () => {
      const huge = JSON.stringify({ type: 'token', value: 'x'.repeat(20 * 1024) });
      vi.mocked(window.sessionStorage.getItem).mockImplementation((key: string) =>
        key === 'pulse_auth' ? huge : null,
      );
      expect(getPulseWebSocketUrl('/ws')).toBe('ws://localhost:3000/ws');
    });

    it('omits the token for a non-object auth payload (parsed?.type optional-chain arm)', () => {
      vi.mocked(window.sessionStorage.getItem).mockImplementation((key: string) =>
        key === 'pulse_auth' ? JSON.stringify('just-a-string') : null,
      );
      expect(getPulseWebSocketUrl('/ws')).toBe('ws://localhost:3000/ws');
    });

    it('swallows malformed JSON in pulse_auth (JSON.parse -> catch arm)', () => {
      vi.mocked(window.sessionStorage.getItem).mockImplementation((key: string) =>
        key === 'pulse_auth' ? '{not-json' : null,
      );
      expect(getPulseWebSocketUrl('/ws')).toBe('ws://localhost:3000/ws');
    });

    it('swallows a throwing sessionStorage.getItem (storage catch arm)', () => {
      vi.mocked(window.sessionStorage.getItem).mockImplementation(() => {
        throw new Error('SecurityError');
      });
      expect(getPulseWebSocketUrl('/ws')).toBe('ws://localhost:3000/ws');
    });

    it('uses the fallback host and skips token lookup in SSR (storage === null arm)', () => {
      vi.stubGlobal('window', undefined);
      expect(getPulseWebSocketUrl('/ws')).toBe('ws://localhost:7655/ws');
    });
  });

  describe('initKioskMode', () => {
    it('is a no-op in SSR (typeof window === "undefined" early return)', () => {
      const storage = window.sessionStorage;
      vi.stubGlobal('window', undefined);
      initKioskMode();
      expect(vi.mocked(storage.setItem)).not.toHaveBeenCalled();
      expect(vi.mocked(storage.removeItem)).not.toHaveBeenCalled();
    });

    it('removes kiosk mode when the URL param is "false" (kioskParam === "false" arm)', () => {
      (window.location as any).search = '?kiosk=false';
      initKioskMode();
      expect(window.sessionStorage.removeItem).toHaveBeenCalledWith('pulse_kiosk_mode');
      expect(window.sessionStorage.setItem).not.toHaveBeenCalled();
    });

    it('does nothing when no kiosk param is present (neither if arm fires)', () => {
      (window.location as any).search = '?foo=bar';
      initKioskMode();
      expect(window.sessionStorage.setItem).not.toHaveBeenCalled();
      expect(window.sessionStorage.removeItem).not.toHaveBeenCalled();
    });

    it('swallows a throwing sessionStorage.setItem (storage catch arm)', () => {
      (window.location as any).search = '?kiosk=1';
      vi.mocked(window.sessionStorage.setItem).mockImplementation(() => {
        throw new Error('QuotaExceeded');
      });
      expect(() => initKioskMode()).not.toThrow();
      // The throw happens inside the setItem branch, so removeItem never runs.
      expect(window.sessionStorage.removeItem).not.toHaveBeenCalled();
    });
  });

  describe('isKioskMode', () => {
    it('returns false in SSR (typeof window === "undefined" early return)', () => {
      vi.stubGlobal('window', undefined);
      expect(isKioskMode()).toBe(false);
    });

    it('returns true via the URL "kiosk=true" arm when nothing is stored', () => {
      vi.mocked(window.sessionStorage.getItem).mockReturnValue(null);
      (window.location as any).search = '?kiosk=true';
      expect(isKioskMode()).toBe(true);
    });

    it('returns false when nothing is stored and no kiosk param is present (URL fallback -> false)', () => {
      vi.mocked(window.sessionStorage.getItem).mockReturnValue(null);
      (window.location as any).search = '';
      expect(isKioskMode()).toBe(false);
    });

    it('returns false when sessionStorage.getItem throws (catch arm)', () => {
      vi.mocked(window.sessionStorage.getItem).mockImplementation(() => {
        throw new Error('SecurityError');
      });
      expect(isKioskMode()).toBe(false);
    });
  });

  describe('getKioskModePreference', () => {
    it('returns null in SSR (typeof window === "undefined" early return)', () => {
      vi.stubGlobal('window', undefined);
      expect(getKioskModePreference()).toBeNull();
    });

    it('returns null when sessionStorage.getItem throws (catch arm)', () => {
      vi.mocked(window.sessionStorage.getItem).mockImplementation(() => {
        throw new Error('SecurityError');
      });
      expect(getKioskModePreference()).toBeNull();
    });
  });

  describe('setKioskMode', () => {
    it('is a no-op in SSR (no storage write; typeof window === "undefined" early return)', () => {
      const storage = window.sessionStorage;
      vi.stubGlobal('window', undefined);
      setKioskMode(true);
      expect(vi.mocked(storage.setItem)).not.toHaveBeenCalled();
    });

    it('swallows a throwing sessionStorage.setItem and skips listener notification (catch arm)', () => {
      const listener = vi.fn();
      const unsubscribe = subscribeToKioskMode(listener);

      vi.mocked(window.sessionStorage.setItem).mockImplementation(() => {
        throw new Error('QuotaExceeded');
      });

      expect(() => setKioskMode(true)).not.toThrow();
      // Confirms the enabled branch was entered, then the throw diverted to catch.
      expect(window.sessionStorage.setItem).toHaveBeenCalledWith('pulse_kiosk_mode', 'true');
      // notifyKioskListeners sits after setItem in the try block, so it must be skipped.
      expect(listener).not.toHaveBeenCalled();

      unsubscribe();
    });
  });
});
