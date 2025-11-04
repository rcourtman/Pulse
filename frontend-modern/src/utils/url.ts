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
  return `${protocol}//${origin.host}${normalizedPath}`;
}
