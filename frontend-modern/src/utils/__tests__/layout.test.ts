import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/settings', () => ({
  SettingsAPI: { getSystemSettings: vi.fn() },
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

import { createLayoutStore } from '@/utils/layout';
import { STORAGE_KEYS } from '@/utils/localStorage';

describe('layout store applyServerMode (#1130)', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('applies the server full-width value even when a stale local preference exists', () => {
    // Stale local preference that loadFromServer() would otherwise honor.
    localStorage.setItem(STORAGE_KEYS.FULL_WIDTH_MODE, 'default');

    const store = createLayoutStore();
    expect(store.isFullWidth()).toBe(false);

    store.applyServerMode(true);

    expect(store.isFullWidth()).toBe(true);
    expect(localStorage.getItem(STORAGE_KEYS.FULL_WIDTH_MODE)).toBe('full-width');
  });

  it('applies a server default that overrides a local full-width preference', () => {
    localStorage.setItem(STORAGE_KEYS.FULL_WIDTH_MODE, 'full-width');

    const store = createLayoutStore();
    expect(store.isFullWidth()).toBe(true);

    store.applyServerMode(false);

    expect(store.isFullWidth()).toBe(false);
  });

  it('leaves the current mode unchanged when the server value is undefined', () => {
    localStorage.setItem(STORAGE_KEYS.FULL_WIDTH_MODE, 'full-width');

    const store = createLayoutStore();
    store.applyServerMode(undefined);

    expect(store.isFullWidth()).toBe(true);
  });
});
