import { Component, createSignal, Show, For, onMount, lazy, Suspense } from 'solid-js';
import { logger } from '@/utils/logger';
import { apiClient, apiFetchJSON } from '@/utils/apiClient';
import { STORAGE_KEYS } from '@/utils/localStorage';
import Globe from 'lucide-solid/icons/globe';
import Key from 'lucide-solid/icons/key';

const SetupWizard = lazy(() => import('./SetupWizard').then((m) => ({ default: m.SetupWizard })));

interface LoginProps {
  onLogin: () => void;
  hasAuth?: boolean; // If true, auth is configured (passed from App.tsx to skip redundant check)
  securityStatus?: SecurityStatus; // Full security status from App.tsx to avoid redundant API call
}

import type { SecurityStatus, SSOProviderInfo } from '@/types/config';

export const Login: Component<LoginProps> = (props) => {
  const [username, setUsername] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [rememberMe, setRememberMe] = createSignal(false);
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [authStatus, setAuthStatus] = createSignal<SecurityStatus | null>(
    props.securityStatus ?? null,
  );
  // If hasAuth is passed from App.tsx, we already know auth status - skip the loading state
  // Also skip if securityStatus is provided
  const [loadingAuth, setLoadingAuth] = createSignal(
    props.hasAuth === undefined && !props.securityStatus,
  );
  const [oidcLoading, setOidcLoading] = createSignal(false);
  const [oidcError, setOidcError] = createSignal('');
  const [oidcMessage, setOidcMessage] = createSignal('');

  const supportsOIDC = () => Boolean(authStatus()?.oidcEnabled);
  const ssoProviders = () => authStatus()?.ssoProviders || [];

  const resolveSSOError = (reason?: string | null) => {
    switch (reason) {
      case 'email_restricted':
        return 'Your account email is not permitted to access Pulse.';
      case 'domain_restricted':
        return 'Your email domain is not allowed for Pulse access.';
      case 'group_restricted':
        return 'Your account is not part of an authorized group to use Pulse.';
      case 'invalid_state':
        return 'The sign-in attempt expired. Please try again.';
      case 'exchange_failed':
        return 'We could not complete the sign-in request. Please try again shortly.';
      case 'session_failed':
        return 'Login succeeded but we could not create a session. Try again.';
      case 'invalid_id_token':
        return 'ID token verification failed. Check that OIDC_ISSUER_URL matches the issuer claim in your provider tokens (check server logs for details).';
      case 'invalid_signature_alg':
        return "The identity provider is issuing HS256 tokens. Configure it to sign ID tokens with RS256 (see your IdP's OIDC settings).";
      case 'invalid_nonce':
        return 'Security validation failed (nonce mismatch). Please try again.';
      case 'saml_validation_failed':
        return 'SAML assertion validation failed. Please try again or contact an administrator.';
      case 'provider_not_found':
        return 'SSO provider not found or not configured.';
      case 'provider_not_initialized':
        return 'SSO provider is not properly configured. Contact your administrator.';
      default:
        return 'Single sign-on failed. Please try again or contact an administrator.';
    }
  };

  onMount(async () => {
    const params = new URLSearchParams(window.location.search);

    // Handle OIDC callback
    const oidcStatus = params.get('oidc');
    if (oidcStatus === 'error') {
      const reason = params.get('oidc_error');
      setOidcError(resolveSSOError(reason));
      setError('');
    } else if (oidcStatus === 'success') {
      setOidcMessage('Signed in successfully. Loading Pulse…');
      setError('');
    }

    // Handle SAML callback
    const samlStatus = params.get('saml');
    if (samlStatus === 'error') {
      const reason = params.get('saml_error');
      setOidcError(resolveSSOError(reason));
      setError('');
    } else if (samlStatus === 'success') {
      setOidcMessage('Signed in successfully. Loading Pulse…');
      setError('');
    }

    // Clean up URL parameters
    if (oidcStatus || samlStatus) {
      params.delete('oidc');
      params.delete('oidc_error');
      params.delete('saml');
      params.delete('saml_error');
      const newQuery = params.toString();
      const newUrl = `${window.location.pathname}${newQuery ? `?${newQuery}` : ''}`;
      window.history.replaceState({}, document.title, newUrl);
    }

    // If securityStatus was passed from App.tsx, use it directly without making another API call
    // This eliminates the flicker between "Checking authentication..." and the login form
    // AND ensures hideLocalLogin, oidcEnabled, etc. are properly respected
    if (props.securityStatus) {
      logger.debug(
        '[Login] Using securityStatus from App.tsx, skipping redundant auth check',
        props.securityStatus,
      );
      setAuthStatus(props.securityStatus);
      setLoadingAuth(false);
      return;
    }

    // Legacy fallback: if only hasAuth was passed (without full securityStatus)
    if (props.hasAuth !== undefined && !props.securityStatus) {
      logger.debug('[Login] Using hasAuth from App.tsx (legacy), fetching full security status');
      // Still need to fetch full status to get hideLocalLogin, OIDC settings, etc.
    }

    logger.debug('[Login] Starting auth check...');
    try {
      const data = await apiFetchJSON<SecurityStatus>('/api/security/status');
      logger.debug('[Login] Auth status data', data);
      setAuthStatus(data);
    } catch (err: any) {
      // Check for 429
      // apiFetchJSON throws error with status attached? No, simple Error map.
      // But if needed we can parse error message if it contains "Too Many Requests"

      // Just assume no auth on error, matching previous logic mostly.
      logger.error('[Login] Failed to check auth status:', err);
      setAuthStatus({ hasAuthentication: false } as SecurityStatus);
    } finally {
      logger.debug('[Login] Auth check complete, setting loading to false');
      setLoadingAuth(false);
    }
  });

  const startOidcLogin = () => {
    if (!supportsOIDC()) return;

    setOidcError('');
    setOidcMessage('');
    setError('');
    setOidcLoading(true);

    // Navigate directly to the OIDC login endpoint using GET.
    // The server will respond with an HTTP redirect to the OIDC provider.
    // This guarantees same-window navigation in all browsers, including Arc.
    const returnTo = encodeURIComponent(`${window.location.pathname}${window.location.search}`);
    window.location.href = `/api/oidc/login?returnTo=${returnTo}`;
  };

  // Auto-redirect to OIDC is intentionally disabled to prevent redirect loops
  // when both password and OIDC are configured. Users must manually click OIDC button.

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    // Read values directly from the form DOM to handle password manager autofill
    // Password managers may fill fields without triggering input events,
    // leaving the SolidJS signals empty while the DOM has the actual values
    const form = e.currentTarget as HTMLFormElement;
    const usernameInput = form.querySelector('#username') as HTMLInputElement;
    const passwordInput = form.querySelector('#password') as HTMLInputElement;
    const usernameValue = usernameInput?.value || username();
    const passwordValue = passwordInput?.value || password();

    // Validate that we have credentials before attempting login
    if (!usernameValue || !passwordValue) {
      setError('Please enter both username and password');
      setLoading(false);
      return;
    }

    try {
      // Use the new login endpoint for better feedback
      const response = await apiClient.fetch('/api/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/json',
        },
        body: JSON.stringify({
          username: usernameValue,
          password: passwordValue,
          rememberMe: rememberMe(),
        }),
        skipAuth: true,
      });

      const data = await response.json();

      if (response.ok && data.success) {
        // Credentials are valid; persist username for convenience and rely on session cookie
        try {
          sessionStorage.setItem('pulse_auth_user', usernameValue);
        } catch (_err) {
          // Ignore storage failures (private browsing, etc.)
        }
        props.onLogin();
      } else if (response.status === 403) {
        // Account is locked
        if (data.remainingMinutes) {
          setError(
            `Account locked. Please try again in ${data.remainingMinutes} ${data.remainingMinutes === 1 ? 'minute' : 'minutes'}.`,
          );
        } else {
          setError(data.message || 'Account temporarily locked due to too many failed attempts.');
        }
        // Clear the input fields
        setUsername('');
        setPassword('');
      } else if (response.status === 429) {
        // Rate limited
        setError(data.message || 'Too many requests. Please wait a moment and try again.');
      } else if (response.status === 401) {
        // Invalid credentials with attempt information
        if (data.remaining !== undefined && data.remaining > 0) {
          setError(
            `${data.message || 'Invalid username or password.'} (${data.remaining} ${data.remaining === 1 ? 'attempt' : 'attempts'} remaining)`,
          );
        } else if (data.locked) {
          setError(data.message || 'Invalid username or password. Account is now locked.');
        } else {
          setError(data.message || 'Invalid username or password');
        }
        // Clear the input fields
        setUsername('');
        setPassword('');
      } else {
        setError(data.message || 'Server error. Please try again.');
      }
    } catch (_err) {
      // Try the old method as fallback
      try {
        const response = await apiClient.fetch('/api/state', {
          headers: {
            Authorization: `Basic ${btoa(`${usernameValue}:${passwordValue}`)}`,
            'X-Requested-With': 'XMLHttpRequest',
            Accept: 'application/json',
          },
          skipAuth: true,
        });

        if (response.ok) {
          try {
            sessionStorage.setItem('pulse_auth_user', usernameValue);
          } catch (_storageErr) {
            // Ignore storage issues
          }
          props.onLogin();
        } else if (response.status === 401) {
          setError('Invalid username or password');
          setUsername('');
          setPassword('');
        } else {
          setError('Server error. Please try again.');
        }
      } catch (_fallbackErr) {
        setError('Failed to connect to server');
      }
    } finally {
      setLoading(false);
    }
  };

  // Debug logging
  logger.debug('[Login] Render', {
    loadingAuth: loadingAuth(),
    authStatus: authStatus(),
  });

  const hasStoredSetupCredentials = () => {
    if (typeof window === 'undefined') return false;
    try {
      return Boolean(sessionStorage.getItem(STORAGE_KEYS.SETUP_CREDENTIALS));
    } catch (_err) {
      return false;
    }
  };

  const legacyDisableAuth = () => authStatus()?.deprecatedDisableAuth === true;
  const showFirstRunSetup = () =>
    authStatus()?.hasAuthentication === false || legacyDisableAuth() || hasStoredSetupCredentials();

  const shouldShowLocalLogin = () => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('show_local') === 'true') return true;
    return !authStatus()?.hideLocalLogin;
  };

  return (
    <Show
      when={!loadingAuth()}
      fallback={
        <div class="min-h-screen flex items-center justify-center bg-base">
          <div class="text-center">
            <div class="animate-spin h-12 w-12 border-4 border-blue-500 border-t-transparent rounded-full mx-auto mb-4"></div>
            <p class="text-muted">Checking authentication...</p>
          </div>
        </div>
      }
    >
      <Show
        when={showFirstRunSetup()}
        fallback={
          <LoginForm
            {...{
              username,
              setUsername,
              password,
              setPassword,
              rememberMe,
              setRememberMe,
              error,
              loading,
              handleSubmit,
              supportsOIDC,
              startOidcLogin,
              oidcLoading,
              oidcError,
              oidcMessage,
              showLocalLogin: shouldShowLocalLogin(),
              ssoProviders: ssoProviders(),
            }}
          />
        }
      >
        <Suspense
          fallback={
            <div class="min-h-screen flex items-center justify-center bg-base">
              <div class="text-center">
                <div class="animate-spin h-12 w-12 border-4 border-blue-500 border-t-transparent rounded-full mx-auto mb-4"></div>
                <p class="text-muted">Loading setup...</p>
              </div>
            </div>
          }
        >
          <SetupWizard onComplete={() => window.location.reload()} />
        </Suspense>
      </Show>
    </Show>
  );
};

// Extract login form to separate component for cleaner code
const LoginForm: Component<{
  username: () => string;
  setUsername: (v: string) => void;
  password: () => string;
  setPassword: (v: string) => void;
  rememberMe: () => boolean;
  setRememberMe: (v: boolean) => void;
  error: () => string;
  loading: () => boolean;
  handleSubmit: (e: Event) => void;
  supportsOIDC: () => boolean;
  startOidcLogin: () => void | Promise<void>;
  oidcLoading: () => boolean;
  oidcError: () => string;
  oidcMessage: () => string;
  showLocalLogin: boolean;
  ssoProviders: SSOProviderInfo[];
}> = (props) => {
  const {
    username,
    setUsername,
    password,
    setPassword,
    rememberMe,
    setRememberMe,
    error,
    loading,
    handleSubmit,
    supportsOIDC,
    startOidcLogin,
    oidcLoading,
    oidcError,
    oidcMessage,
    showLocalLogin,
    ssoProviders,
  } = props;

  // Check if we're on the demo server
  const isDemoServer = () => {
    const hostname = window.location.hostname;
    const normalized = hostname.toLowerCase();
    return normalized === 'demo' || normalized.startsWith('demo.') || normalized.includes('.demo.');
  };

  return (
    <div class="min-h-screen flex items-center justify-center bg-base py-12 px-4 sm:px-6 lg:px-8">
      <div class="max-w-md w-full space-y-8">
        {/* Demo Credentials Banner */}
        <Show when={isDemoServer()}>
          <div class="bg-surface rounded-md p-4 shadow-sm border border-blue-200 dark:border-blue-800 animate-fade-in">
            <div class="flex items-center gap-3">
              <div class="flex-shrink-0 w-10 h-10 rounded-full bg-blue-600 flex items-center justify-center">
                <svg
                  class="w-5 h-5 text-white"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
                  />
                </svg>
              </div>
              <div class="flex-1">
                <div class="font-semibold text-sm text-base-content">Demo Mode</div>
                <div class="text-sm text-muted">
                  Login with{' '}
                  <code class="bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 rounded font-mono text-xs">
                    demo
                  </code>{' '}
                  /{' '}
                  <code class="bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 rounded font-mono text-xs">
                    demo
                  </code>
                </div>
              </div>
            </div>
          </div>
        </Show>

        <div class="animate-fade-in">
          <div class="flex justify-center mb-8">
            <div class="relative group">
              <img
                src="/logo.svg"
                alt="Pulse Logo"
                class="relative w-24 h-24 rounded-md shadow-sm"
              />
            </div>
          </div>

          <h2 class="mt-6 text-center text-4xl font-extrabold text-blue-600 animate-fade-in delay-100 pb-1">
            Welcome to Pulse
          </h2>

          <Show when={showLocalLogin}>
            <p class="mt-3 text-center text-sm text-muted animate-fade-in delay-200">
              Enter your credentials to continue
            </p>
          </Show>
        </div>
        <form
          class="mt-8 space-y-6 bg-surface rounded-md p-8 shadow-sm border border-border animate-slide-up"
          onSubmit={handleSubmit}
        >
          {/* Multi-Provider SSO Section */}
          <Show when={ssoProviders.length > 0}>
            <div class="space-y-3">
              <For each={ssoProviders}>
                {(provider) => (
                  <button
                    type="button"
                    class={`w-full inline-flex items-center justify-center gap-2 px-4 py-3 rounded-md border border-blue-500 text-blue-600 hover:bg-blue-50 transition dark:border-blue-400 dark:text-blue-200 dark:hover:bg-blue-900 ${oidcLoading() ? 'opacity-75 cursor-wait' : ''}`}
                    disabled={oidcLoading()}
                    onClick={() => {
                      window.location.href = provider.loginUrl;
                    }}
                  >
                    <span class="inline-flex items-center gap-2">
                      <Show
                        when={provider.iconUrl}
                        fallback={
                          provider.type === 'saml' ? (
                            <Key class="w-5 h-5" />
                          ) : (
                            <Globe class="w-5 h-5" />
                          )
                        }
                      >
                        <img src={provider.iconUrl} alt="" class="w-5 h-5" />
                      </Show>
                      Continue with {provider.displayName || provider.name}
                    </span>
                  </button>
                )}
              </For>
              <Show when={oidcError()}>
                <div class="rounded-md bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 px-3 py-2 text-sm text-red-600 dark:text-red-300">
                  {oidcError()}
                </div>
              </Show>
              <Show when={oidcMessage()}>
                <div class="rounded-md bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-700 px-3 py-2 text-sm text-green-600 dark:text-green-300">
                  {oidcMessage()}
                </div>
              </Show>
              <Show when={showLocalLogin}>
                <div class="flex items-center gap-3 pt-2">
                  <span class="flex-1 h-px bg-surface-hover" />
                  <span class="text-xs uppercase tracking-wide text-muted">or</span>
                  <span class="flex-1 h-px bg-surface-hover" />
                </div>
                <p class="text-xs text-center text-muted">
                  Use your admin credentials to sign in below.
                </p>
              </Show>
            </div>
          </Show>
          {/* Legacy OIDC fallback (when no multi-provider SSO but legacy OIDC is enabled) */}
          <Show when={ssoProviders.length === 0 && supportsOIDC()}>
            <div class="space-y-3">
              <button
                type="button"
                class={`w-full inline-flex items-center justify-center gap-2 px-4 py-3 rounded-md border border-blue-500 text-blue-600 hover:bg-blue-50 transition dark:border-blue-400 dark:text-blue-200 dark:hover:bg-blue-900 ${oidcLoading() ? 'opacity-75 cursor-wait' : ''}`}
                disabled={oidcLoading()}
                onClick={() => startOidcLogin()}
              >
                <Show
                  when={!oidcLoading()}
                  fallback={
                    <span class="inline-flex items-center gap-2">
                      <span class="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                      Redirecting…
                    </span>
                  }
                >
                  <span class="inline-flex items-center gap-2">
                    <Globe class="w-5 h-5" />
                    Continue with Single Sign-On
                  </span>
                </Show>
              </button>
              <Show when={oidcError()}>
                <div class="rounded-md bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 px-3 py-2 text-sm text-red-600 dark:text-red-300">
                  {oidcError()}
                </div>
              </Show>
              <Show when={oidcMessage()}>
                <div class="rounded-md bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-700 px-3 py-2 text-sm text-green-600 dark:text-green-300">
                  {oidcMessage()}
                </div>
              </Show>
              <Show when={showLocalLogin}>
                <div class="flex items-center gap-3 pt-2">
                  <span class="flex-1 h-px bg-surface-hover" />
                  <span class="text-xs uppercase tracking-wide text-muted">or</span>
                  <span class="flex-1 h-px bg-surface-hover" />
                </div>
                <p class="text-xs text-center text-muted">
                  Use your admin credentials to sign in below.
                </p>
              </Show>
            </div>
          </Show>
          <Show when={showLocalLogin}>
            <div class="space-y-4">
              <div class="relative">
                <label for="username" class="sr-only">
                  Username
                </label>
                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <svg class="h-5 w-5 " fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                    />
                  </svg>
                </div>
                <input
                  id="username"
                  name="username"
                  type="text"
                  autocomplete="username"
                  required
                  class="appearance-none relative block w-full pl-10 pr-3 py-3 border border-border placeholder-gray-500 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all sm:text-sm"
                  placeholder="Username"
                  value={username()}
                  onInput={(e) => setUsername(e.currentTarget.value)}
                />
              </div>
              <div class="relative">
                <label for="password" class="sr-only">
                  Password
                </label>
                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <svg class="h-5 w-5 " fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                    />
                  </svg>
                </div>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autocomplete="current-password"
                  required
                  class="appearance-none relative block w-full pl-10 pr-3 py-3 border border-border placeholder-gray-500 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all sm:text-sm"
                  placeholder="Password"
                  value={password()}
                  onInput={(e) => setPassword(e.currentTarget.value)}
                />
              </div>
              <div class="flex items-center">
                <input
                  id="remember-me"
                  name="remember-me"
                  type="checkbox"
                  checked={rememberMe()}
                  onChange={(e) => setRememberMe(e.currentTarget.checked)}
                  class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-border rounded cursor-pointer"
                />
                <label
                  for="remember-me"
                  class="ml-2 block text-sm text-base-content cursor-pointer"
                >
                  Remember me
                </label>
              </div>
            </div>

            <Show when={error()}>
              <div
                class={`rounded-md p-4 ${
                  error().includes('locked')
                    ? 'bg-orange-50 dark:bg-orange-900'
                    : 'bg-red-50 dark:bg-red-900'
                }`}
              >
                <div class="flex">
                  <div class="flex-shrink-0">
                    <Show
                      when={error().includes('locked')}
                      fallback={
                        <svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
                          <path
                            fill-rule="evenodd"
                            d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                            clip-rule="evenodd"
                          />
                        </svg>
                      }
                    >
                      <svg class="h-5 w-5 text-orange-400" viewBox="0 0 20 20" fill="currentColor">
                        <path
                          fill-rule="evenodd"
                          d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z"
                          clip-rule="evenodd"
                        />
                      </svg>
                    </Show>
                  </div>
                  <div class="ml-3">
                    <p
                      class={`text-sm ${
                        error().includes('locked')
                          ? 'text-orange-800 dark:text-orange-200'
                          : 'text-red-800 dark:text-red-200'
                      }`}
                    >
                      {error()}
                    </p>
                    <Show when={error().includes('locked') && error().includes('minute')}>
                      <p class="text-xs mt-1 text-orange-700 dark:text-orange-300">
                        Lockouts automatically expire after the specified time. If you need
                        immediate access, contact your administrator.
                      </p>
                    </Show>
                  </div>
                </div>
              </div>
            </Show>

            <div>
              <button
                type="submit"
                disabled={loading()}
                class="group relative w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <Show when={loading()}>
                  <svg
                    class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    ></circle>
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                </Show>
                <Show when={loading()} fallback="Sign in to Pulse">
                  Authenticating...
                </Show>
              </button>
            </div>
          </Show>
        </form>
      </div>
    </div>
  );
};
