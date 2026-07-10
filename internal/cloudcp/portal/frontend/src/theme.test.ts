import { beforeEach, describe, expect, it, vi } from 'vitest';

import { applyPortalTheme, effectivePortalTheme, installPortalThemeToggle } from './theme';

function stubLocalStorage() {
  var data = new Map<string, string>();
  var stub = {
    getItem: function(key: string) { return data.has(key) ? String(data.get(key)) : null; },
    setItem: function(key: string, value: string) { data.set(key, String(value)); },
    removeItem: function(key: string) { data.delete(key); },
    clear: function() { data.clear(); },
  };
  Object.defineProperty(window, 'localStorage', { value: stub, configurable: true });
}

function stubMatchMedia(prefersDark: boolean) {
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(function(query: string) {
    return {
      matches: prefersDark && query.includes('dark'),
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    };
  }));
}

describe('portal theme', function() {
  beforeEach(function() {
    document.documentElement.removeAttribute('data-theme');
    document.body.innerHTML = '<button id="portal-theme-toggle" type="button"></button>';
    vi.unstubAllGlobals();
    stubLocalStorage();
  });

  it('follows the system preference when nothing is stored', function() {
    stubMatchMedia(true);
    expect(effectivePortalTheme()).toBe('dark');
    stubMatchMedia(false);
    expect(effectivePortalTheme()).toBe('light');
  });

  it('prefers the stored override over the system preference', function() {
    stubMatchMedia(true);
    window.localStorage.setItem('pulse-portal-theme', 'light');
    expect(effectivePortalTheme()).toBe('light');
  });

  it('applies and clears the data-theme attribute', function() {
    applyPortalTheme('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    applyPortalTheme(null);
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
  });

  it('toggle switches theme, persists it, and updates the button label', function() {
    stubMatchMedia(false);
    installPortalThemeToggle();
    var button = document.getElementById('portal-theme-toggle') as HTMLButtonElement;
    expect(button.getAttribute('aria-label')).toBe('Switch to dark theme');

    button.click();
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(window.localStorage.getItem('pulse-portal-theme')).toBe('dark');
    expect(button.getAttribute('aria-label')).toBe('Switch to light theme');

    button.click();
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    expect(window.localStorage.getItem('pulse-portal-theme')).toBe('light');
    expect(button.getAttribute('aria-label')).toBe('Switch to dark theme');
  });
});
