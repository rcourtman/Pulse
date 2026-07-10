const THEME_STORAGE_KEY = 'pulse-portal-theme';

export type PortalTheme = 'light' | 'dark';

function storedTheme(): PortalTheme | null {
  try {
    var value = window.localStorage.getItem(THEME_STORAGE_KEY);
    return value === 'dark' || value === 'light' ? value : null;
  } catch {
    return null;
  }
}

function systemPrefersDark(): boolean {
  return typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function effectivePortalTheme(): PortalTheme {
  var stored = storedTheme();
  if (stored) return stored;
  return systemPrefersDark() ? 'dark' : 'light';
}

export function applyPortalTheme(theme: PortalTheme | null): void {
  var root = document.documentElement;
  if (theme === 'dark' || theme === 'light') {
    root.setAttribute('data-theme', theme);
  } else {
    root.removeAttribute('data-theme');
  }
}

// Sun/moon glyphs; the icon shows the theme the click switches TO.
var SUN_ICON =
  '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">' +
  '<circle cx="8" cy="8" r="3.25"/>' +
  '<path d="M8 1.5v1.5M8 13v1.5M1.5 8H3M13 8h1.5M3.4 3.4l1.06 1.06M11.54 11.54l1.06 1.06M12.6 3.4l-1.06 1.06M4.46 11.54L3.4 12.6"/>' +
  '</svg>';
var MOON_ICON =
  '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true">' +
  '<path d="M13.5 9.5A5.75 5.75 0 0 1 6.5 2.5a5.75 5.75 0 1 0 7 7Z"/>' +
  '</svg>';

function renderToggle(button: HTMLElement): void {
  var current = effectivePortalTheme();
  button.innerHTML = current === 'dark' ? SUN_ICON : MOON_ICON;
  button.setAttribute(
    'aria-label',
    current === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'
  );
}

export function installPortalThemeToggle(): void {
  var button = document.getElementById('portal-theme-toggle');
  if (!button) return;
  renderToggle(button);
  button.addEventListener('click', function() {
    var next: PortalTheme = effectivePortalTheme() === 'dark' ? 'light' : 'dark';
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, next);
    } catch {
      // Private-mode storage failures still get the in-page switch.
    }
    applyPortalTheme(next);
    renderToggle(button);
  });
  if (typeof window.matchMedia === 'function') {
    var media = window.matchMedia('(prefers-color-scheme: dark)');
    if (typeof media.addEventListener === 'function') {
      media.addEventListener('change', function() {
        if (!storedTheme()) renderToggle(button);
      });
    }
  }
}
