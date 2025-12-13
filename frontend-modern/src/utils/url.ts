const FALLBACK_BASE_URL = 'http://localhost:7655';

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

export function getPulseHostname(): string {
  const origin = getPulseOriginUrl();
  return origin?.hostname || 'localhost';
}

export function getPulsePort(): string {
  const origin = getPulseOriginUrl();
  if (!origin) {
    return '7655';
  }
  if (origin.port) {
    return origin.port;
  }
  return origin.protocol === 'https:' ? '443' : '80';
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
      if (stored) {
        const parsed = JSON.parse(stored);
        if (parsed?.type === 'token' && parsed.value) {
          url += `?token=${encodeURIComponent(parsed.value)}`;
        }
      }
    }
  } catch {
    // Ignore storage errors
  }

  return url;
}
