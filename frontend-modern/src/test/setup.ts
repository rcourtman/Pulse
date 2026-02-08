import '@testing-library/jest-dom';
import { beforeEach } from 'vitest';

type StorageValue = string | null;

const hasStorageApi = (candidate: unknown): candidate is Storage => {
  if (!candidate) return false;
  const maybeStorage = candidate as Partial<Storage>;
  return (
    typeof maybeStorage.getItem === 'function' &&
    typeof maybeStorage.setItem === 'function' &&
    typeof maybeStorage.removeItem === 'function' &&
    typeof maybeStorage.clear === 'function'
  );
};

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

const ensureLocalStorage = () => {
  // Avoid reading `globalThis.localStorage` directly. Newer Node versions
  // expose an experimental Web Storage API that emits warnings when accessed
  // without a backing file. Use a descriptor check instead to keep test output
  // clean and deterministic.
  const desc = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  const existing = desc && 'value' in desc ? (desc.value as Storage | undefined) : undefined;
  if (hasStorageApi(existing)) return;

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
beforeEach(() => {
  ensureLocalStorage();
});
