import { createSignal, createEffect, onCleanup, Accessor } from 'solid-js';

/**
 * Creates a debounced version of a signal value
 * @param value - The signal accessor to debounce
 * @param delay - Delay in milliseconds (default: 300ms)
 * @returns Debounced signal accessor
 */
export function useDebouncedValue<T>(value: Accessor<T>, delay: number = 300): Accessor<T> {
  const [debouncedValue, setDebouncedValue] = createSignal<T>(value());

  createEffect(() => {
    const currentValue = value();
    const timer = setTimeout(() => {
      setDebouncedValue(() => currentValue);
    }, delay);

    onCleanup(() => clearTimeout(timer));
  });

  return debouncedValue;
}
