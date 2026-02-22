import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useDebouncedValue } from '../useDebouncedValue';
import { createSignal } from 'solid-js';

describe('useDebouncedValue', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('returns initial value immediately', () => {
    const [value] = createSignal('initial');
    const debounced = useDebouncedValue(value, 300);

    expect(debounced()).toBe('initial');
  });

  it('updates debounced value after delay', () => {
    const [value, setValue] = createSignal('initial');
    const debounced = useDebouncedValue(value, 300);

    expect(debounced()).toBe('initial');

    setValue('updated');

    expect(debounced()).toBe('initial');

    vi.advanceTimersByTime(300);

    expect(debounced()).toBe('updated');
  });

  it('uses default delay of 300ms', () => {
    const [value, setValue] = createSignal('initial');
    const debounced = useDebouncedValue(value);

    setValue('updated');

    vi.advanceTimersByTime(299);
    expect(debounced()).toBe('initial');

    vi.advanceTimersByTime(1);
    expect(debounced()).toBe('updated');
  });

  it('resets timer on rapid changes', () => {
    const [value, setValue] = createSignal('initial');
    const debounced = useDebouncedValue(value, 300);

    setValue('first');
    vi.advanceTimersByTime(200);
    setValue('second');
    vi.advanceTimersByTime(200);
    setValue('third');

    expect(debounced()).toBe('initial');

    vi.advanceTimersByTime(100);
    expect(debounced()).toBe('initial');

    vi.advanceTimersByTime(200);
    expect(debounced()).toBe('third');
  });

  it('handles different types', () => {
    const [value, setValue] = createSignal(42);
    const debounced = useDebouncedValue(value, 100);

    expect(debounced()).toBe(42);

    setValue(100);
    vi.advanceTimersByTime(100);

    expect(debounced()).toBe(100);
  });

  it('handles objects', () => {
    const [value, setValue] = createSignal({ a: 1 });
    const debounced = useDebouncedValue(value, 100);

    expect(debounced()).toEqual({ a: 1 });

    setValue({ b: 2 });
    vi.advanceTimersByTime(100);

    expect(debounced()).toEqual({ b: 2 });
  });
});
