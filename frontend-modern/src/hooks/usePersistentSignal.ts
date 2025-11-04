import { Accessor, Setter, createEffect, createSignal } from 'solid-js';
import { logger } from '@/utils/logger';

export type PersistentSignalOptions<T> = {
  /**
   * Custom serialization function. Defaults to `String(value)`.
   */
  serialize?: (value: T) => string;
  /**
   * Custom deserialization function. Defaults to casting the stored string.
   */
  deserialize?: (value: string) => T;
  /**
   * Optional equality comparison passed to Solid's `createSignal`.
   */
  equals?: (prev: T, next: T) => boolean;
  /**
   * Alternate storage implementation (defaults to `window.localStorage`).
   */
  storage?: Storage;
};

/**
 * Creates a Solid signal that persists its value to localStorage (or a custom storage).
 * The signal reads the initial value synchronously from storage when available.
 */
export function usePersistentSignal<T>(
  key: string,
  defaultValue: T,
  options: PersistentSignalOptions<T> = {},
): [Accessor<T>, Setter<T>] {
  const storage =
    options.storage ?? (typeof window !== 'undefined' ? window.localStorage : undefined);
  const serialize = options.serialize ?? ((value: T) => String(value));
  const deserialize = options.deserialize ?? ((value: string) => value as unknown as T);

  const initialValue = (() => {
    if (!storage) {
      return defaultValue;
    }

    try {
      const raw = storage.getItem(key);
      if (raw === null) {
        return defaultValue;
      }
      return deserialize(raw);
    } catch (err) {
      logger.warn(`[usePersistentSignal] Failed to read "${key}" from storage`, err);
      return defaultValue;
    }
  })();

  const signalOptions = options.equals ? { equals: options.equals } : undefined;
  const [value, setValue] = createSignal<T>(initialValue, signalOptions);

  createEffect(() => {
    if (!storage) {
      return;
    }

    const current = value();
    try {
      if (current === undefined || current === null) {
        storage.removeItem(key);
      } else {
        storage.setItem(key, serialize(current));
      }
    } catch (err) {
      logger.warn(`[usePersistentSignal] Failed to persist "${key}"`, err);
    }
  });

  return [value, setValue];
}
