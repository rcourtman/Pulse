import fs from 'node:fs';
import { beforeAll, beforeEach, expect } from 'vitest';
import {
  needsJestDomForSource,
  needsNonEnglishLocalesForSource,
} from './vitest.projects';

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

const readTestSource = (testPath: string | undefined): string | null => {
  if (!testPath) return null;
  try {
    return fs.readFileSync(testPath, 'utf8');
  } catch {
    return null;
  }
};

ensureLocalStorage();

const initialTestPath = expect.getState().testPath;
const initialTestSource = readTestSource(initialTestPath);

// Load jest-dom matchers only for DOM-backed tests that use them (or fall back
// to loading when the current test file cannot be determined).
if (
  typeof window !== 'undefined' &&
  (initialTestPath === undefined ||
    initialTestSource === null ||
    needsJestDomForSource(initialTestPath, initialTestSource))
) {
  await import('@testing-library/jest-dom');
}

beforeAll(async () => {
  const currentTestPath = expect.getState().testPath ?? initialTestPath;
  const source =
    currentTestPath === initialTestPath ? initialTestSource : readTestSource(currentTestPath);
  if (source !== null && !needsNonEnglishLocalesForSource(source)) {
    return;
  }
  const { FIRST_LOCALIZATION_LOCALES, loadLocaleCatalog } = await import('@/i18n');
  await Promise.all(FIRST_LOCALIZATION_LOCALES.map((locale) => loadLocaleCatalog(locale)));
});

beforeEach(() => {
  ensureLocalStorage();
});
