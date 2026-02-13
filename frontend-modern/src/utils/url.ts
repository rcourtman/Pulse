const FALLBACK_BASE_URL = 'http://localhost:7655';
const KIOSK_MODE_KEY = 'pulse_kiosk_mode';
const CONTROL_CHAR_PATTERN = /[\u0000-\u001F\u007F]/;
const MAX_API_TOKEN_LENGTH = 8 * 1024;
const MAX_AUTH_STORAGE_CHARS = 16 * 1024;

// Reactive kiosk mode tracking - listeners are notified when kiosk mode changes
type KioskListener = (enabled: boolean) => void;
const kioskListeners = new Set<KioskListener>();

function notifyKioskListeners(enabled: boolean): void {
  kioskListeners.forEach(listener => listener(enabled));
}

/**
 * Subscribe to kiosk mode changes.
 * Returns an unsubscribe function.
 */
export function subscribeToKioskMode(listener: KioskListener): () => void {
  kioskListeners.add(listener);
  return () => kioskListeners.delete(listener);
}

/**
 * Check and persist kiosk mode from URL parameters.
 * When ?kiosk=1 or ?kiosk=true is in the URL, this stores the preference
 * in sessionStorage so it persists across navigation and refreshes.
 * Call this on app initialization.
 */
export function initKioskMode(): void {
  if (typeof window === 'undefined') return;

  try {
    const params = new URLSearchParams(window.location.search);
    const kioskParam = params.get('kiosk');

    if (kioskParam === '1' || kioskParam === 'true') {
      window.sessionStorage.setItem(KIOSK_MODE_KEY, 'true');
    }
    // Note: We don't clear kiosk mode if param is absent or false
    // This allows navigating within the app while preserving kiosk mode
    // User must explicitly clear session or pass kiosk=false to exit
    if (kioskParam === '0' || kioskParam === 'false') {
      window.sessionStorage.removeItem(KIOSK_MODE_KEY);
    }
  } catch {
    // Ignore storage errors
  }
}

/**
 * Check if the app is in kiosk mode.
 * Returns true if kiosk mode is stored in session or present in current URL.
 */
export function isKioskMode(): boolean {
  if (typeof window === 'undefined') return false;

  try {
    // Check sessionStorage first (persists across navigation)
    const stored = window.sessionStorage.getItem(KIOSK_MODE_KEY);
    if (stored === 'true') return true;
    if (stored === 'false') return false; // Explicitly disabled

    // Fall back to URL check (for initial page load)
    const params = new URLSearchParams(window.location.search);
    const kioskParam = params.get('kiosk');
    return kioskParam === '1' || kioskParam === 'true';
  } catch {
    return false;
  }
}

/**
 * Get the explicit kiosk mode preference from storage.
 * Returns true/false if set, or null if unset.
 */
export function getKioskModePreference(): boolean | null {
  if (typeof window === 'undefined') return null;
  try {
    const stored = window.sessionStorage.getItem(KIOSK_MODE_KEY);
    if (stored === 'true') return true;
    if (stored === 'false') return false;
    return null;
  } catch {
    return null;
  }
}

/**
 * Set kiosk mode on or off.
 * Updates sessionStorage and notifies reactive listeners.
 */
export function setKioskMode(enabled: boolean): void {
  if (typeof window === 'undefined') return;

  try {
    if (enabled) {
      window.sessionStorage.setItem(KIOSK_MODE_KEY, 'true');
    } else {
      window.sessionStorage.setItem(KIOSK_MODE_KEY, 'false');
    }
    notifyKioskListeners(enabled);
  } catch {
    // Ignore storage errors
  }
}

export function getPulseBaseUrl(): string {
  if (typeof window === 'undefined' || !window.location) {
    return FALLBACK_BASE_URL;
  }

  const { origin, protocol, hostname, port } = window.location;

  if (origin && origin !== 'null') {
    return origin;
  }

  if (!protocol || !hostname) {
    return FALLBACK_BASE_URL;
  }

  const base = `${protocol}//${hostname}`;
  return port ? `${base}:${port}` : base;
}

function getPulseOriginUrl(): URL | null {
  try {
    return new URL(getPulseBaseUrl());
  } catch {
    return null;
  }
}

function getStoredSessionAuthToken(storage: Storage): string | null {
  const stored = storage.getItem('pulse_auth');
  if (!stored) {
    return null;
  }

  try {
    const parsed = JSON.parse(stored) as unknown;
    if (!parsed || typeof parsed !== 'object') {
      return null;
    }
    const auth = parsed as { type?: unknown; value?: unknown };
    if (auth.type !== 'token' || typeof auth.value !== 'string' || auth.value === '') {
      return null;
    }
    return auth.value;
  } catch {
    return null;
  }
}

export function getPulseHostname(): string {
  const origin = getPulseOriginUrl();
  return origin?.hostname || 'localhost';
}

export function isPulseHttps(): boolean {
  const origin = getPulseOriginUrl();
  return origin?.protocol === 'https:';
}

export function getPulseWebSocketUrl(path = '/ws'): string {
  const origin = getPulseOriginUrl();
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;

  if (!origin) {
    return `ws://localhost${normalizedPath}`;
  }

  const protocol = origin.protocol === 'https:' ? 'wss:' : 'ws:';
  let url = `${protocol}//${origin.host}${normalizedPath}`;

  // Add API token as query parameter if available (WebSocket doesn't support headers)
  // Import dynamically to avoid circular dependencies
  try {
    const storage = typeof window !== 'undefined' ? window.sessionStorage : null;
    if (storage) {
      const stored = storage.getItem('pulse_auth');
      if (stored && stored.length <= MAX_AUTH_STORAGE_CHARS) {
        const parsed = JSON.parse(stored);
        const token = parsed?.type === 'token' ? sanitizeApiToken(parsed.value) : null;
        if (token) {
          const separator = url.includes('?') ? '&' : '?';
          url += `${separator}token=${encodeURIComponent(token)}`;
        }
      }
    }
  } catch {
    // Ignore storage errors
  }

  return url;
}
