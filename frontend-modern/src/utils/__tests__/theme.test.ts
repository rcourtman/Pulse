import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  applyThemeClass,
  computeIsDark,
  getStoredThemePreference,
  hasStoredThemePreference,
  normalizeThemePreference,
  persistThemePreference,
} from '@/utils/theme';

describe('theme utils', () => {
  const originalMatchMedia = window.matchMedia;

  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.classList.remove('dark');

    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query.includes('dark'),
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: originalMatchMedia,
    });
    window.localStorage.clear();
    document.documentElement.classList.remove('dark');
  });

  it('reads canonical theme preference when present', () => {
    window.localStorage.setItem('pulseThemePreference', 'light');
    expect(getStoredThemePreference()).toBe('light');
  });

  it('reports whether any stored theme preference exists', () => {
    expect(hasStoredThemePreference()).toBe(false);
    window.localStorage.setItem('pulseThemePreference', 'dark');
    expect(hasStoredThemePreference()).toBe(true);
  });

  it('persists canonical preference only', () => {
    persistThemePreference('dark');
    expect(window.localStorage.getItem('pulseThemePreference')).toBe('dark');

    persistThemePreference('system');
    expect(window.localStorage.getItem('pulseThemePreference')).toBe('system');
  });

  it('normalizes unrecognized values to system', () => {
    expect(normalizeThemePreference('light')).toBe('light');
    expect(normalizeThemePreference('unknown')).toBe('system');
    expect(normalizeThemePreference(null)).toBe('system');
  });

  it('computes dark mode from preference and system fallback', () => {
    expect(computeIsDark('dark')).toBe(true);
    expect(computeIsDark('light')).toBe(false);
    expect(computeIsDark('system')).toBe(true);
  });

  it('applies root dark class directly', () => {
    applyThemeClass(true);
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    applyThemeClass(false);
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });
});
