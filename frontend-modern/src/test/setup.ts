import '@testing-library/jest-dom';

type StorageValue = string | null;

const ensureLocalStorage = () => {
  // Avoid reading `globalThis.localStorage` directly. Newer Node versions
  // expose an experimental Web Storage API that emits warnings when accessed
  // without a backing file. Use a descriptor check instead to keep test output
  // clean and deterministic.
  const desc = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  const existing = desc && 'value' in desc ? (desc.value as Storage | undefined) : undefined;
  const hasApi =
    existing &&
    typeof existing.getItem === 'function' &&
    typeof existing.setItem === 'function' &&
    typeof existing.removeItem === 'function' &&
    typeof existing.clear === 'function';

  if (hasApi) return;

  class InMemoryStorage {
    private store: Record<string, string> = {};

    get length() {
      return Object.keys(this.store).length;
    }

    key(index: number): StorageValue {
      return Object.keys(this.store)[index] ?? null;
    }

    getItem(key: string): StorageValue {
      return Object.prototype.hasOwnProperty.call(this.store, key) ? this.store[key] : null;
    }

    setItem(key: string, value: string) {
      this.store[key] = String(value);
    }

    removeItem(key: string) {
      delete this.store[key];
    }

    clear() {
      this.store = {};
    }
  }

  Object.defineProperty(globalThis, 'Storage', {
    value: InMemoryStorage,
    writable: true,
    configurable: true,
  });

  const storage = new InMemoryStorage();
  Object.defineProperty(globalThis, 'localStorage', {
    value: storage,
    writable: true,
    configurable: true,
  });

  if (typeof window !== 'undefined') {
    Object.defineProperty(window, 'localStorage', {
      value: storage,
      writable: true,
      configurable: true,
    });
  }
};

ensureLocalStorage();
