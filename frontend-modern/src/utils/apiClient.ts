// Centralized API client with authentication support
// This replaces the three separate auth utilities (api.ts, auth.ts, authInterceptor.ts)

import { logger } from '@/utils/logger';
import { STORAGE_KEYS } from '@/utils/localStorage';

const AUTH_STORAGE_KEY = STORAGE_KEYS.AUTH;

const getSessionStorage = (): Storage | undefined => {
  if (typeof window === 'undefined') {
    return undefined;
  }
  try {
    return window.sessionStorage;
  } catch {
    return undefined;
  }
};

interface FetchOptions extends Omit<RequestInit, 'headers'> {
  headers?: Record<string, string>;
  skipAuth?: boolean;
}

class ApiClient {
  private apiToken: string | null = null;
  private csrfToken: string | null = null;

  constructor() {
    // Check session storage for existing auth on page load
    this.loadStoredAuth();
    // Load CSRF token from cookie
    this.loadCSRFToken();
  }

  private persistToken(token: string) {
    const storage = getSessionStorage();
    if (!storage) return;

    try {
      storage.setItem(
        STORAGE_KEYS.AUTH,
        JSON.stringify({
          type: 'token',
          value: token,
        }),
      );
    } catch {
      // Ignore storage quota errors
    }
  }

  private removeStoredToken() {
    const storage = getSessionStorage();
    if (!storage) return;

    try {
      storage.removeItem(AUTH_STORAGE_KEY);
      storage.removeItem(STORAGE_KEYS.LEGACY_TOKEN);
    } catch {
      // Ignore storage quota errors
    }
  }

  private loadCSRFToken(): string | null {
    // Read CSRF token from cookie
    const cookies = document.cookie.split(';');
    for (const cookie of cookies) {
      const [name, ...rest] = cookie.trim().split('=');
      if (name !== 'pulse_csrf') continue;
      const value = rest.join('=');
      this.csrfToken = decodeURIComponent(value || '');
      return this.csrfToken;
    }
    this.csrfToken = null;
    return null;
  }

  private loadStoredAuth() {
    try {
      // First, check for token in URL query parameter (for kiosk/dashboard mode)
      // This allows visiting ?token=xxx to auto-authenticate without cookies
      if (typeof window !== 'undefined' && window.location?.search) {
        const params = new URLSearchParams(window.location.search);
        const urlToken = params.get('token');
        if (urlToken) {
          this.apiToken = urlToken;
          this.persistToken(urlToken);
          // Clean the token from URL for security (don't expose in browser history)
          params.delete('token');
          const newQuery = params.toString();
          const newUrl = `${window.location.pathname}${newQuery ? `?${newQuery}` : ''}`;
          window.history.replaceState({}, document.title, newUrl);
          return;
        }
      }

      const storage = getSessionStorage();
      if (!storage) return;

      const stored = storage.getItem(AUTH_STORAGE_KEY);
      if (stored) {
        const { type, value } = JSON.parse(stored);
        if (type === 'token') {
          this.apiToken = value;
        }
        return;
      }

      // Legacy storage key used before apiClient refactor
      const legacyToken = storage.getItem(STORAGE_KEYS.LEGACY_TOKEN);
      if (legacyToken) {
        this.apiToken = legacyToken;
        this.persistToken(legacyToken);
        storage.removeItem(STORAGE_KEYS.LEGACY_TOKEN);
      }
    } catch (_err) {
      // Invalid stored auth, ignore
    }
  }

  // Set API token
  setApiToken(token: string) {
    this.apiToken = token;

    this.persistToken(token);
  }

  getApiToken(): string | null {
    if (this.apiToken) {
      return this.apiToken;
    }

    const storage = getSessionStorage();
    if (!storage) {
      return null;
    }

    try {
      const stored = storage.getItem(AUTH_STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored);
        if (parsed?.type === 'token' && typeof parsed.value === 'string') {
          this.apiToken = parsed.value;
          return parsed.value;
        }
      }

      const legacyToken = storage.getItem(STORAGE_KEYS.LEGACY_TOKEN);
      if (legacyToken) {
        this.apiToken = legacyToken;
        this.persistToken(legacyToken);
        storage.removeItem(STORAGE_KEYS.LEGACY_TOKEN);
        return legacyToken;
      }
    } catch {
      // Ignore parsing/storage errors
    }

    return null;
  }

  // Clear all authentication
  clearAuth() {
    this.apiToken = null;
    this.removeStoredToken();

    const storage = getSessionStorage();
    if (!storage) return;
    try {
      storage.removeItem('pulse_auth_user');
    } catch {
      // Ignore storage quota errors
    }
  }

  clearApiToken() {
    this.apiToken = null;
    this.removeStoredToken();
  }

  // Check if we have any auth configured
  hasAuth(): boolean {
    if (this.apiToken) {
      return true;
    }

    if (typeof document !== 'undefined') {
      const cookies = document.cookie.split(';');
      for (const cookie of cookies) {
        const [name] = cookie.trim().split('=');
        if (name === 'pulse_session') {
          return true;
        }
      }
    }

    return false;
  }

  // Ensure CSRF token is available by making a GET request if needed
  // The backend issues CSRF cookies on GET requests to /api/* endpoints
  private async ensureCSRFToken(): Promise<string | null> {
    try {
      // Make a simple GET request to trigger CSRF cookie issuance
      const response = await fetch('/api/health', {
        method: 'GET',
        credentials: 'include',
      });

      // The response should have set the pulse_csrf cookie
      if (response.ok) {
        // Small delay to ensure cookie is set
        await new Promise(resolve => setTimeout(resolve, 10));
        return this.loadCSRFToken();
      }
    } catch (err) {
      logger.warn('Failed to fetch CSRF token', err);
    }
    return null;
  }

  // Main fetch wrapper that adds authentication
  async fetch(url: string, options: FetchOptions = {}): Promise<Response> {
    const { skipAuth = false, headers = {}, ...fetchOptions } = options;

    // Build headers object
    const finalHeaders: Record<string, string> = { ...headers };

    // Always add headers to prevent browser auth popup for API calls
    if (url.startsWith('/api/')) {
      finalHeaders['X-Requested-With'] = 'XMLHttpRequest';
      if (!finalHeaders['Accept']) {
        finalHeaders['Accept'] = 'application/json';
      }
    }

    // Add authentication if available and not skipped
    if (!skipAuth) {
      if (this.apiToken) {
        finalHeaders['X-API-Token'] = this.apiToken;
      }
    }

    // Add CSRF token for state-changing requests
    const method = (fetchOptions.method || 'GET').toUpperCase();
    if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
      // Try to get CSRF token, or fetch one if missing
      let token = this.loadCSRFToken();
      if (!token) {
        // No CSRF token available - try to get one by making a GET request
        token = await this.ensureCSRFToken();
      }
      if (token) {
        finalHeaders['X-CSRF-Token'] = token;
      }
    }

    // Always include credentials for cookies (WebSocket session support)
    const finalOptions: RequestInit = {
      ...fetchOptions,
      headers: finalHeaders,
      credentials: 'include', // Important for session cookies
    };

    const response = await fetch(url, finalOptions);

    // If we get a 401 on an API call (not during initial auth check), redirect to login
    // Skip redirect for specific auth-check endpoints and background data fetching to avoid loops
    const skipRedirectUrls = ['/api/security/status', '/api/state', '/api/settings/ai', '/api/charts', '/api/resources'];
    const shouldSkipRedirect = skipRedirectUrls.some(path => url.includes(path));
    if (response.status === 401 && !shouldSkipRedirect) {
      logger.warn('Authentication expired - redirecting to login');
      // Clear auth and redirect to login
      if (typeof window !== 'undefined') {
        this.clearAuth();
        localStorage.setItem('just_logged_out', 'true');
        window.location.href = '/';
      }
      return response;
    }

    // Handle CSRF token failures - the 403 response should have set a new CSRF cookie
    if (response.status === 403) {
      // First try the response header (backend sends new token in X-CSRF-Token header)
      let refreshedToken = response.headers.get('X-CSRF-Token');

      // If not in header, reload from cookie (backend also sets pulse_csrf cookie on 403)
      if (!refreshedToken) {
        // Force reload from cookie - the 403 response just set it
        this.csrfToken = null;
        refreshedToken = this.loadCSRFToken();
      }

      if (refreshedToken) {
        this.csrfToken = refreshedToken;
        logger.debug(`[apiClient] Retrying ${method} ${url} with refreshed CSRF token`);
        finalHeaders['X-CSRF-Token'] = refreshedToken;
        const retryResponse = await fetch(url, {
          ...fetchOptions,
          headers: finalHeaders,
          credentials: 'include',
        });
        return retryResponse;
      }
    }

    // Handle rate limiting with automatic retry
    if (response.status === 429) {
      const retryAfter = response.headers.get('Retry-After');
      const waitTime = retryAfter ? parseInt(retryAfter) * 1000 : 2000; // Default 2 seconds

      logger.warn(`Rate limit hit, retrying after ${waitTime}ms`);

      // Wait and retry once
      await new Promise((resolve) => setTimeout(resolve, waitTime));

      const retryResponse = await fetch(url, {
        ...fetchOptions,
        headers: finalHeaders,
        credentials: 'include',
      });

      return retryResponse;
    }

    return response;
  }

  // Convenience method for JSON requests
  async fetchJSON<T = unknown>(url: string, options: FetchOptions = {}): Promise<T> {
    const response = await this.fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });

    if (!response.ok) {
      const text = await response.text();
      // Try to extract just the error message without HTTP status codes
      let errorMessage = text;

      // First try to parse as JSON (our API returns structured errors like {error, message, feature, upgrade_url})
      try {
        const jsonError = JSON.parse(text);
        if (jsonError.message) {
          errorMessage = jsonError.message;
        } else if (jsonError.error && typeof jsonError.error === 'string') {
          // Some APIs return {error: "message"} format
          errorMessage = jsonError.error;
        }
      } catch {
        // Not JSON, try other formats

        // If it looks like an HTML error page, try to extract the message
        if (text.includes('<pre>') && text.includes('</pre>')) {
          const match = text.match(/<pre>(.*?)<\/pre>/s);
          if (match) errorMessage = match[1];
        }

        // If the backend sent a plain text error, use it directly
        if (!text.includes('<') && text.length < 200) {
          errorMessage = text;
        } else if (text.length > 200) {
          // For long responses, just use a generic message
          errorMessage = `Request failed with status ${response.status}`;
        }
      }

      throw new Error(errorMessage || `Request failed with status ${response.status}`);
    }

    const text = await response.text();
    if (!text) return null as T;

    try {
      return JSON.parse(text) as T;
    } catch (_err) {
      logger.error('Failed to parse JSON response', text);
      throw new Error('Invalid JSON response from server');
    }
  }

}

// Create singleton instance
export const apiClient = new ApiClient();

// Export convenience functions
export const apiFetch = (url: string, options?: FetchOptions) => apiClient.fetch(url, options);
export const apiFetchJSON = <T = unknown>(url: string, options?: FetchOptions) =>
  apiClient.fetchJSON<T>(url, options);
export const setApiToken = (token: string) => apiClient.setApiToken(token);
export const getApiToken = () => apiClient.getApiToken();
export const clearAuth = () => apiClient.clearAuth();
export const clearApiToken = () => apiClient.clearApiToken();
export const hasAuth = () => apiClient.hasAuth();
