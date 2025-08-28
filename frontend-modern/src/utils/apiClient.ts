// Centralized API client with authentication support
// This replaces the three separate auth utilities (api.ts, auth.ts, authInterceptor.ts)

interface FetchOptions extends Omit<RequestInit, 'headers'> {
  headers?: Record<string, string>;
  skipAuth?: boolean;
}

class ApiClient {
  private authHeader: string | null = null;
  private apiToken: string | null = null;
  private csrfToken: string | null = null;

  constructor() {
    // Check session storage for existing auth on page load
    this.loadStoredAuth();
    // Load CSRF token from cookie
    this.loadCSRFToken();
  }

  private loadCSRFToken() {
    // Read CSRF token from cookie
    const cookies = document.cookie.split(';');
    for (const cookie of cookies) {
      const [name, value] = cookie.trim().split('=');
      if (name === 'pulse_csrf') {
        this.csrfToken = decodeURIComponent(value);
        break;
      }
    }
  }

  private loadStoredAuth() {
    try {
      // Try to load from session storage (survives page refresh but not tab close)
      const stored = sessionStorage.getItem('pulse_auth');
      if (stored) {
        const { type, value } = JSON.parse(stored);
        if (type === 'basic') {
          this.authHeader = value;
        } else if (type === 'token') {
          this.apiToken = value;
        }
      }
    } catch (e) {
      // Invalid stored auth, ignore
    }
  }

  // Set basic auth credentials
  setBasicAuth(username: string, password: string) {
    const encoded = btoa(`${username}:${password}`);
    this.authHeader = `Basic ${encoded}`;
    
    // Store in session storage
    sessionStorage.setItem('pulse_auth', JSON.stringify({
      type: 'basic',
      value: this.authHeader
    }));
  }

  // Set API token
  setApiToken(token: string) {
    this.apiToken = token;
    
    // Store in session storage
    sessionStorage.setItem('pulse_auth', JSON.stringify({
      type: 'token',
      value: token
    }));
  }

  // Clear all authentication
  clearAuth() {
    this.authHeader = null;
    this.apiToken = null;
    sessionStorage.removeItem('pulse_auth');
  }

  // Check if we have any auth configured
  hasAuth(): boolean {
    return !!(this.authHeader || this.apiToken);
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
      if (this.authHeader) {
        finalHeaders['Authorization'] = this.authHeader;
      }
      if (this.apiToken) {
        finalHeaders['X-API-Token'] = this.apiToken;
      }
    }

    // Add CSRF token for state-changing requests
    const method = (fetchOptions.method || 'GET').toUpperCase();
    if (this.csrfToken && method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
      finalHeaders['X-CSRF-Token'] = this.csrfToken;
    }

    // Always include credentials for cookies (WebSocket session support)
    const finalOptions: RequestInit = {
      ...fetchOptions,
      headers: finalHeaders,
      credentials: 'include' // Important for session cookies
    };

    const response = await fetch(url, finalOptions);

    // If we get a 401, our auth might be invalid
    if (response.status === 401 && this.hasAuth()) {
      // Could trigger a re-login flow here
      console.warn('Authentication failed - credentials may be incorrect');
      // Don't clear auth automatically - let the user retry
    }

    // Handle CSRF token failures
    if (response.status === 403) {
      const text = await response.clone().text();
      if (text.includes('CSRF')) {
        // Try to reload CSRF token from cookie and retry
        this.loadCSRFToken();
        if (this.csrfToken) {
          finalHeaders['X-CSRF-Token'] = this.csrfToken;
          const retryResponse = await fetch(url, {
            ...fetchOptions,
            headers: finalHeaders,
            credentials: 'include'
          });
          return retryResponse;
        }
      }
    }

    // Handle rate limiting with automatic retry
    if (response.status === 429) {
      const retryAfter = response.headers.get('Retry-After');
      const waitTime = retryAfter ? parseInt(retryAfter) * 1000 : 2000; // Default 2 seconds
      
      console.warn(`Rate limit hit, retrying after ${waitTime}ms`);
      
      // Wait and retry once
      await new Promise(resolve => setTimeout(resolve, waitTime));
      
      const retryResponse = await fetch(url, {
        ...fetchOptions,
        headers: finalHeaders,
        credentials: 'include'
      });
      
      return retryResponse;
    }

    return response;
  }

  // Convenience method for JSON requests
  async fetchJSON(url: string, options: FetchOptions = {}): Promise<any> {
    const response = await this.fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers
      }
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`API request failed: ${response.status} ${text}`);
    }

    const text = await response.text();
    if (!text) return null;
    
    try {
      return JSON.parse(text);
    } catch (e) {
      console.error('Failed to parse JSON response:', text);
      throw new Error('Invalid JSON response from server');
    }
  }

  // Check if authentication is required
  async checkAuthRequired(): Promise<boolean> {
    try {
      // Try to access a protected endpoint without auth
      const response = await fetch('/api/state', { 
        method: 'GET',
        credentials: 'omit' // Don't send cookies or auth
      });
      
      // If we get 401, auth is required
      if (response.status === 401) {
        return true;
      }
      
      // If we get 200, no auth required
      return false;
    } catch (e) {
      // Network error - try the security status endpoint
      try {
        const response = await fetch('/api/security/status');
        const data = await response.json();
        return data.hasAuthentication || data.requiresAuth || false;
      } catch (err) {
        // Can't determine, assume no auth
        return false;
      }
    }
  }
}

// Create singleton instance
export const apiClient = new ApiClient();

// Export convenience functions
export const apiFetch = (url: string, options?: FetchOptions) => apiClient.fetch(url, options);
export const apiFetchJSON = (url: string, options?: FetchOptions) => apiClient.fetchJSON(url, options);
export const setBasicAuth = (username: string, password: string) => apiClient.setBasicAuth(username, password);
export const setApiToken = (token: string) => apiClient.setApiToken(token);
export const clearAuth = () => apiClient.clearAuth();
export const hasAuth = () => apiClient.hasAuth();
export const checkAuthRequired = () => apiClient.checkAuthRequired();