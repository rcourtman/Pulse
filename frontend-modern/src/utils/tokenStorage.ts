import { onCleanup } from 'solid-js';

const STORAGE_KEY = 'pulse_api_token';

let memoryToken: string | null = null;
const listeners = new Set<(value: string | null) => void>();

const getSessionStorage = (): Storage | undefined => {
  if (typeof window === 'undefined') {
    return undefined;
  }
  try {
    return window.sessionStorage;
  } catch {
    return undefined;
  }
};

const notify = (value: string | null) => {
  listeners.forEach((listener) => {
    try {
      listener(value);
    } catch {
      // Ignore listener failures to avoid breaking notification flow.
    }
  });

  if (typeof window !== 'undefined') {
    try {
      window.dispatchEvent(new StorageEvent('storage', { key: STORAGE_KEY, newValue: value ?? null }));
    } catch {
      // Some browsers disallow programmatic StorageEvent dispatch; ignore failures.
    }
  }
};

const writeValue = (value: string | null) => {
  memoryToken = value;
  const storage = getSessionStorage();
  if (!storage) {
    notify(value);
    return;
  }

  try {
    if (value === null) {
      storage.removeItem(STORAGE_KEY);
    } else {
      storage.setItem(STORAGE_KEY, value);
    }
  } catch {
    // If sessionStorage is unavailable (e.g., Safari private mode), fall back to memory token only.
  }

  notify(value);
};

export const getStoredAPIToken = (): string | null => {
  const storage = getSessionStorage();
  if (!storage) {
    return memoryToken;
  }

  try {
    const stored = storage.getItem(STORAGE_KEY);
    if (stored !== null) {
      memoryToken = stored;
    }
    return stored ?? memoryToken;
  } catch {
    return memoryToken;
  }
};

export const setStoredAPIToken = (token: string): void => {
  writeValue(token);
};

export const clearStoredAPIToken = (): void => {
  writeValue(null);
};

export const subscribeAPIToken = (listener: (token: string | null) => void): (() => void) => {
  listeners.add(listener);
  listener(getStoredAPIToken());
  return () => {
    listeners.delete(listener);
  };
};

export const useAPITokenSubscription = (listener: (token: string | null) => void): void => {
  if (typeof window === 'undefined') {
    listener(getStoredAPIToken());
    return;
  }
  const unsubscribe = subscribeAPIToken(listener);
  onCleanup(unsubscribe);
};
