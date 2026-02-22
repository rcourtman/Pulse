import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { usePersistentSignal } from '../usePersistentSignal';

const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value;
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key];
    }),
    clear: vi.fn(() => {
      store = {};
    }),
    get length() {
      return Object.keys(store).length;
    },
  };
})();

describe('usePersistentSignal', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', localStorageMock);
    localStorageMock.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns default value when storage is empty', () => {
    const [value] = usePersistentSignal('test-key', 'default');

    expect(value()).toBe('default');
  });

  it('returns value from localStorage if exists', () => {
    localStorageMock.getItem.mockReturnValueOnce('stored-value');

    const [value] = usePersistentSignal('test-key', 'default');

    expect(value()).toBe('stored-value');
  });

  it('updates localStorage when value changes', () => {
    const [value, setValue] = usePersistentSignal('test-key', 'default');

    setValue('new-value');

    expect(localStorageMock.setItem).toHaveBeenCalledWith('test-key', 'new-value');
    expect(value()).toBe('new-value');
  });

  it('removes item from storage when value is null', () => {
    const [value, setValue] = usePersistentSignal('test-key', 'default');

    setValue(null as any);

    expect(localStorageMock.removeItem).toHaveBeenCalledWith('test-key');
  });

  it('removes item from storage when value is undefined', () => {
    const [value, setValue] = usePersistentSignal('test-key', 'default');

    setValue(undefined as any);

    expect(localStorageMock.removeItem).toHaveBeenCalledWith('test-key');
  });

  it('uses custom serialize function', () => {
    const serialize = vi.fn((val: object) => JSON.stringify(val));
    const [value, setValue] = usePersistentSignal('test-key', { a: 1 }, { serialize });

    setValue({ b: 2 });

    expect(serialize).toHaveBeenCalledWith({ b: 2 });
    expect(localStorageMock.setItem).toHaveBeenCalledWith('test-key', '{"b":2}');
  });

  it('uses custom deserialize function', () => {
    const deserialize = vi.fn((val: string) => parseInt(val, 10));
    localStorageMock.getItem.mockReturnValueOnce('42');

    const [value] = usePersistentSignal('test-key', 0, { deserialize });

    expect(deserialize).toHaveBeenCalledWith('42');
    expect(value()).toBe(42);
  });

  it('uses equals option for comparison', () => {
    const equals = vi.fn((prev: number, next: number) => prev === next);
    const [value, setValue] = usePersistentSignal('test-key', 0, { equals });

    setValue(0);

    expect(equals).toHaveBeenCalledWith(0, 0);
  });

  it('handles custom storage implementation', () => {
    const customStorage = {
      getItem: vi.fn(() => 'custom-stored'),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    };

    const [value] = usePersistentSignal('test-key', 'default', { storage: customStorage as any });

    expect(customStorage.getItem).toHaveBeenCalledWith('test-key');
    expect(value()).toBe('custom-stored');
  });

  it('handles storage errors gracefully', () => {
    const storage = {
      getItem: vi.fn(() => {
        throw new Error('Storage error');
      }),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    };

    const consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

    const [value] = usePersistentSignal('test-key', 'default', { storage: storage as any });

    expect(value()).toBe('default');
    expect(consoleWarnSpy).toHaveBeenCalled();

    consoleWarnSpy.mockRestore();
  });

  it('handles numeric values', () => {
    const [value, setValue] = usePersistentSignal('test-key', 0);

    setValue(42);

    expect(localStorageMock.setItem).toHaveBeenCalledWith('test-key', '42');
    expect(value()).toBe(42);
  });

  it('handles boolean values', () => {
    const [value, setValue] = usePersistentSignal('test-key', false);

    setValue(true);

    expect(localStorageMock.setItem).toHaveBeenCalledWith('test-key', 'true');
    expect(value()).toBe(true);
  });

  it('handles object values', () => {
    const [value, setValue] = usePersistentSignal('test-key', { a: 1 });

    setValue({ b: 2 });

    expect(localStorageMock.setItem).toHaveBeenCalledWith('test-key', '[object Object]');
    expect(value()).toEqual({ b: 2 });
  });
});
