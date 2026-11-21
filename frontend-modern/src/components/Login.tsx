import {
  batch,
  Component,
  createEffect,
  createSignal,
  Show,
  For,
  onMount,
  onCleanup,
  lazy,
  Suspense,
  untrack,
} from 'solid-js';
import { logger } from '@/utils/logger';
import { PulseBrandMark } from '@/components/Brand/PulseBrandMark';
import { apiClient, apiFetchJSON } from '@/utils/apiClient';
import { SESSION_STORAGE_KEYS, STORAGE_KEYS } from '@/utils/localStorage';
import { TROUBLESHOOTING_DOC_URL } from '@/utils/docsLinks';
import Globe from 'lucide-solid/icons/globe';
import Key from 'lucide-solid/icons/key';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { readSSOReturnError, SSOReturnError } from '@/components/SSOReturnError';

const SetupWizard = lazy(() => import('./SetupWizard').then((m) => ({ default: m.SetupWizard })));

export const SIGN_IN_CHECK_SLOW_DELAY_MS = 10_000;

interface LoginProps {
  onLogin: () => void;
  hasAuth?: boolean; // Compatibility hint; full discovery still decides setup and sign-in options.
  securityStatus?: SecurityStatus; // Full security status from App.tsx to avoid redundant API call
}

import type { SecurityStatus, SSOProviderInfo } from '@/types/config';

// The public demo's credentials. They are shown on the login page, so there
// is nothing to protect by making the visitor type them.
const DEMO_CREDENTIALS = { username: 'demo', password: 'demo' } as const;

function getBrowserStorage(kind: 'localStorage' | 'sessionStorage'): Storage | undefined {
  if (typeof window === 'undefined') return undefined;
  try {
    return window[kind];
  } catch {
    return undefined;
  }
}

function readRememberedUsername(): string {
  const storage = getBrowserStorage('localStorage');
  if (!storage) return '';
  try {
    return (storage.getItem(STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME) || '').trim();
  } catch {
    return '';
  }
}

function persistRememberedUsername(username: string, remember: boolean): void {
  const storage = getBrowserStorage('localStorage');
  if (!storage) return;
  try {
    if (remember) {
      storage.setItem(STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME, username);
    } else {
      storage.removeItem(STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME);
    }
  } catch {
    // Ignore storage failures (private browsing, quota, managed browsers).
  }
}

export const Login: Component<LoginProps> = (props) => {
  const [username, setUsername] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [rememberMe, setRememberMe] = createSignal(false);
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  // Only an explicit server observation can select setup. A boolean hint from
  // bootstrap cannot supply the SSO/local-login policy or turn a failed read
  // into an unconfigured server.
  const hasAuthDiscovery = (value: unknown): value is SecurityStatus =>
    value !== null &&
    typeof value === 'object' &&
    typeof (value as SecurityStatus).hasAuthentication === 'boolean';
  const initialStatus = hasAuthDiscovery(props.securityStatus) ? props.securityStatus : null;
  const [authStatus, setAuthStatus] = createSignal<SecurityStatus | null>(initialStatus);
  const [loadingAuth, setLoadingAuth] = createSignal(!initialStatus);
  const [authReadFailed, setAuthReadFailed] = createSignal(false);
  const [authRestart, setAuthRestart] = createSignal<'hidden' | 'waiting' | 'ready'>('hidden');
  let authRead: { controller: AbortController; slowTimer: number } | undefined;
  let active = true;
  let accessEntry: HTMLDivElement | undefined;
  let authReadNotice: HTMLDivElement | undefined;
  let authReadHelp: HTMLAnchorElement | undefined;
  let loginHelp: HTMLAnchorElement | undefined;

  const retireAuthRead = () => {
    const previous = authRead;
    if (!previous) return;
    authRead = undefined;
    window.clearTimeout(previous.slowTimer);
    previous.controller.abort();
  };

  onCleanup(() => {
    active = false;
    retireAuthRead();
  });

  const readAuthStatus = async () => {
    if (!active || (authRead && authRestart() !== 'ready')) return;
    if (authRead) {
      // Keep the focused control mounted, but make it single-flight again.
      setAuthRestart('waiting');
      retireAuthRead();
    }
    const controller = new AbortController();
    const read = {
      controller,
      slowTimer: window.setTimeout(() => {
        if (authRead === read) setAuthRestart('ready');
      }, SIGN_IN_CHECK_SLOW_DELAY_MS),
    };
    authRead = read;
    setLoadingAuth(true);
    let result: SecurityStatus | null = null;
    try {
      const data = await apiFetchJSON<unknown>('/api/security/status', {
        signal: controller.signal,
        retry: false,
      });
      if (hasAuthDiscovery(data)) result = data;
    } catch {
      // Never expose proxy content, raw response errors or guessed auth state.
    }

    // Aborting cannot retract a reply that has already been delivered.
    if (authRead !== read) return;
    window.clearTimeout(read.slowTimer);
    authRead = undefined;
    const focused = document.activeElement;
    const returnFocus = result && authReadNotice?.contains(focused);
    const wasReadingHelp = focused === authReadHelp;
    batch(() => {
      setAuthStatus(result);
      setAuthReadFailed(result === null);
      setLoadingAuth(false);
      setAuthRestart('hidden');
    });
    if (returnFocus) {
      queueMicrotask(() => {
        // Discovery removes the notice. Return owned focus without stealing it
        // from a user who already moved on, and keep Help selected if it exists.
        if (!active || document.activeElement !== document.body) return;
        const target = wasReadingHelp && loginHelp?.isConnected ? loginHelp : accessEntry;
        target?.focus();
      });
    }
  };
  const [oidcLoading] = createSignal(false);
  const [oidcError, setOidcError] = createSignal('');
  const [oidcMessage, setOidcMessage] = createSignal('');
  const [demoAutoLogin, setDemoAutoLogin] = createSignal(false);

  const ssoProviders = () => authStatus()?.ssoProviders || [];
  const demoModeEnabled = () =>
    authStatus()?.presentationPolicy?.demoMode === true ||
    authStatus()?.sessionCapabilities?.demoMode === true;

  onMount(async () => {
    const rememberedUsername = readRememberedUsername();
    if (rememberedUsername) {
      setUsername(rememberedUsername);
      setRememberMe(true);
    }

    const params = new URLSearchParams(window.location.search);

    // Retain the return message without using it to select sign-in options.
    const oidcStatus = params.get('oidc');
    const samlStatus = params.get('saml');
    const returnError = readSSOReturnError(params);
    if (returnError) {
      setOidcError(returnError);
      setError('');
    }
    if (oidcStatus === 'success' || samlStatus === 'success') {
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

    // The full bootstrap observation avoids a redundant read. Failed or
    // incomplete bootstrap discovery takes the same guarded read path here.
    if (!initialStatus) await readAuthStatus();
  });

  // Auto-redirect to OIDC is intentionally disabled to prevent redirect loops
  // when both password and OIDC are configured. Users must manually click OIDC button.

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    if (loading()) return;
    setError('');

    // Read values directly from the form DOM to handle password manager and
    // browser form-state restores that do not trigger Solid input/change events.
    const form = e.currentTarget as HTMLFormElement;
    const usernameInput = form.querySelector('#username') as HTMLInputElement;
    const passwordInput = form.querySelector('#password') as HTMLInputElement;
    const rememberInput = form.querySelector('#remember-me') as HTMLInputElement;
    const usernameValue = usernameInput?.value || username();
    const passwordValue = passwordInput?.value || password();
    const rememberLogin = rememberInput?.checked ?? rememberMe();
    setRememberMe(rememberLogin);

    // Validate that we have credentials before attempting login
    if (!usernameValue || !passwordValue) {
      setError('Please enter both username and password');
      return;
    }

    await submitCredentials(usernameValue, passwordValue, rememberLogin);
  };

  const submitCredentials = async (
    usernameValue: string,
    passwordValue: string,
    rememberLogin: boolean,
  ) => {
    // Demo startup depends on discovery, not on the request it starts.
    if (untrack(loading)) return;
    setLoading(true);
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
          rememberMe: rememberLogin,
        }),
        skipAuth: true,
      });

      const data = await response.json();

      if (response.ok && data.success) {
        // Credentials are valid; persist username for convenience and rely on session cookie
        try {
          getBrowserStorage('sessionStorage')?.setItem(STORAGE_KEYS.AUTH_USER, usernameValue);
        } catch (_err) {
          // Ignore storage failures (private browsing, etc.)
        }
        persistRememberedUsername(usernameValue, rememberLogin);
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
        if (!rememberLogin) setUsername('');
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
        if (!rememberLogin) setUsername('');
        setPassword('');
      } else {
        setError(data.message || 'Server error. Please try again.');
      }
    } catch (_err) {
      setError('Failed to connect to server');
    } finally {
      setLoading(false);
    }
  };

  // Demo mode: the public demo is read-only and its credentials are printed on
  // this page anyway, so sign the visitor in instead of making them type
  // demo/demo. Once per browser tab, and never straight after a sign-out.
  createEffect(() => {
    if (loadingAuth() || authReadFailed() || !demoModeEnabled() || showFirstRunSetup()) return;
    const params = new URLSearchParams(window.location.search);
    if (params.has('oidc') || params.has('saml')) return;
    const storage = getBrowserStorage('sessionStorage');
    if (storage?.getItem(SESSION_STORAGE_KEYS.DEMO_AUTO_LOGIN)) return;
    try {
      storage?.setItem(SESSION_STORAGE_KEYS.DEMO_AUTO_LOGIN, 'attempted');
    } catch (_err) {
      // If the marker cannot be stored the sign-in still runs once for this render.
    }
    setDemoAutoLogin(true);
    void submitCredentials(DEMO_CREDENTIALS.username, DEMO_CREDENTIALS.password, false).finally(
      () => setDemoAutoLogin(false),
    );
  });

  // Debug logging
  logger.debug('[Login] Render', {
    loadingAuth: loadingAuth(),
    authStatus: authStatus(),
  });

  const showFirstRunSetup = () => authStatus()?.hasAuthentication === false;

  const shouldShowLocalLogin = () => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('show_local') === 'true') return true;
    return !authStatus()?.hideLocalLogin;
  };

  return (
    <div ref={accessEntry} role="region" aria-label="Pulse access" tabIndex={-1}>
      <Show
        when={!loadingAuth() && !authReadFailed()}
        fallback={
          <div class="min-h-screen flex items-center justify-center px-4 py-8 text-base-content">
            <div
              ref={authReadNotice}
              class="w-full max-w-md rounded-md border border-border bg-surface p-6 shadow-xs sm:p-8"
            >
              <h1 class="flex items-center gap-2 text-lg font-semibold">
                <Show when={!authReadFailed()}>
                  <LoadingSpinner size="button" tone="info" />
                </Show>
                {authReadFailed()
                  ? 'Could not check sign-in options'
                  : 'Checking authentication...'}
              </h1>
              <p role="status" aria-atomic="true" class="mt-3 text-sm leading-6 text-muted">
                {authRestart() === 'ready'
                  ? 'The sign-in check is taking longer than expected. You can keep waiting or restart it.'
                  : loadingAuth()
                    ? authReadFailed() || authRestart() === 'waiting'
                      ? 'Checking authentication again…'
                      : 'Checking which sign-in options Pulse offers.'
                    : 'Pulse could not read its authentication status. This does not mean authentication is unconfigured.'}
              </p>
              <SSOReturnError message={oidcError()} duringCheck />
              <Show when={authReadFailed() || authRestart() !== 'hidden'}>
                <Show when={authReadFailed()}>
                  <p class="mt-3 text-sm leading-6 text-muted">
                    Retry the check before changing authentication settings. This check does not
                    change your settings or start setup.
                  </p>
                </Show>
                <button
                  type="button"
                  aria-disabled={loadingAuth() && authRestart() !== 'ready'}
                  onClick={() => void readAuthStatus()}
                  class="mt-5 min-h-11 w-full rounded-md bg-blue-600 px-4 py-3 text-sm font-medium text-white hover:bg-blue-700 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 aria-disabled:opacity-50"
                >
                  {authRestart() === 'hidden' ? 'Retry sign-in check' : 'Restart sign-in check'}
                </button>
                <a
                  ref={authReadHelp}
                  href={TROUBLESHOOTING_DOC_URL}
                  class="mt-3 inline-flex min-h-11 items-center rounded-sm px-1 text-sm font-medium text-blue-700 hover:underline focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:text-blue-300"
                >
                  Can’t sign in?
                </a>
              </Show>
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
                oidcLoading,
                oidcError,
                oidcMessage,
                demoModeEnabled: demoModeEnabled(),
                demoAutoLogin,
                showLocalLogin: shouldShowLocalLogin(),
                ssoProviders: ssoProviders(),
              }}
              helpRef={(element) => (loginHelp = element)}
            />
          }
        >
          <Suspense
            fallback={
              <div class="min-h-screen flex items-center justify-center">
                <div class="text-center">
                  <LoadingSpinner size="lg" tone="info" label="Loading setup" class="mb-4" />
                  <p class="text-muted">Loading setup...</p>
                </div>
              </div>
            }
          >
            <SetupWizard
              onComplete={(nextPath) => {
                const target = nextPath || window.location.pathname;
                window.location.assign(target);
              }}
            />
          </Suspense>
        </Show>
      </Show>
    </div>
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
  oidcLoading: () => boolean;
  oidcError: () => string;
  oidcMessage: () => string;
  demoModeEnabled: boolean;
  demoAutoLogin: () => boolean;
  showLocalLogin: boolean;
  ssoProviders: SSOProviderInfo[];
  helpRef: (element: HTMLAnchorElement) => void;
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
    oidcLoading,
    oidcError,
    oidcMessage,
    demoModeEnabled,
    demoAutoLogin,
    showLocalLogin,
    ssoProviders,
  } = props;
  const hasSSOProviders = ssoProviders.length > 0;
  const providerReturn = () => hasSSOProviders && Boolean(oidcError());
  const compactReturn = () => Boolean(oidcError()) && (showLocalLogin || hasSSOProviders);

  return (
    <div
      class="min-h-screen flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8"
      classList={{ 'max-sm:pt-6': compactReturn(), 'max-sm:py-6': hasSSOProviders }}
    >
      <div class="max-w-md w-full space-y-8" classList={{ 'max-sm:space-y-4': providerReturn() }}>
        {/* Demo Credentials Banner */}
        <Show when={demoModeEnabled}>
          <div class="bg-surface rounded-md p-4 shadow-xs border border-blue-200 dark:border-blue-800 animate-fade-in">
            <div class="flex items-center gap-3">
              <div class="shrink-0 w-10 h-10 rounded-full bg-blue-600 flex items-center justify-center">
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
                <div class="text-sm text-muted" role="status">
                  {demoAutoLogin()
                    ? 'Signing you in to the demo…'
                    : !showLocalLogin && error()
                      ? 'Automatic demo sign-in didn’t complete.'
                      : ''}
                </div>
                <Show when={!demoAutoLogin()}>
                  <div class="text-sm text-muted">
                    {showLocalLogin
                      ? 'Login with'
                      : 'The local sign-in form is hidden. If your administrator offers local sign-in, use'}{' '}
                    <code class="bg-blue-100 dark:bg-blue-900/25 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 rounded-sm font-mono text-xs">
                      demo
                    </code>{' '}
                    /{' '}
                    <code class="bg-blue-100 dark:bg-blue-900/25 text-blue-700 dark:text-blue-300 px-1.5 py-0.5 rounded-sm font-mono text-xs">
                      demo
                    </code>
                  </div>
                </Show>
              </div>
            </div>
          </div>
        </Show>

        <div class="animate-fade-in">
          <div class="flex justify-center mb-8" classList={{ 'max-sm:mb-3': compactReturn() }}>
            <div class="relative group">
              <PulseBrandMark
                class={`relative w-24 h-24${compactReturn() ? ' max-sm:size-14' : ''}`}
              />
            </div>
          </div>

          <h2
            class="mt-6 text-center text-4xl font-extrabold text-blue-600 animate-fade-in delay-100 pb-1"
            classList={{ 'max-sm:mt-3': providerReturn() }}
          >
            Welcome to Pulse
          </h2>

          <Show when={showLocalLogin}>
            <p class="mt-3 text-center text-sm text-muted animate-fade-in delay-200">
              Enter your credentials to continue
            </p>
          </Show>
        </div>
        <form
          class="mt-8 space-y-6 bg-surface rounded-md p-8 shadow-xs border border-border animate-slide-up"
          classList={{ 'max-sm:mt-4 max-sm:p-4': providerReturn() }}
          onSubmit={handleSubmit}
        >
          {/* Return errors describe the attempt even when no providers are now listed. */}
          <Show when={hasSSOProviders || oidcError()}>
            <div class="space-y-3">
              {/* Read why the last attempt failed before choosing where to try next. */}
              <SSOReturnError message={oidcError()} />
              <For each={ssoProviders}>
                {(provider) => (
                  <button
                    type="button"
                    class={`w-full min-h-11 inline-flex items-center justify-center px-2 py-2.5 rounded-md border border-blue-500 text-blue-600 hover:bg-blue-50 transition sm:px-4 sm:py-3 dark:border-blue-400 dark:text-blue-200 dark:hover:bg-blue-900/25 ${oidcLoading() ? 'opacity-75 cursor-wait' : ''}`}
                    disabled={oidcLoading()}
                    onClick={() => {
                      window.location.href = provider.loginUrl;
                    }}
                  >
                    <span class="inline-flex min-w-0 items-center gap-2">
                      <Show
                        when={provider.iconUrl}
                        fallback={
                          provider.type === 'saml' ? (
                            <Key class="size-5 shrink-0" aria-hidden="true" />
                          ) : (
                            <Globe class="size-5 shrink-0" aria-hidden="true" />
                          )
                        }
                      >
                        <img src={provider.iconUrl} alt="" class="size-5 shrink-0" />
                      </Show>
                      <span class="min-w-0 wrap-anywhere">
                        Continue with {provider.displayName || provider.name}
                      </span>
                    </span>
                  </button>
                )}
              </For>
              <Show when={hasSSOProviders && oidcMessage()}>
                <div class="rounded-md bg-green-50 dark:bg-green-900/25 border border-green-200 dark:border-green-700 px-3 py-2 text-sm text-green-600 dark:text-green-300">
                  {oidcMessage()}
                </div>
              </Show>
              <Show when={hasSSOProviders && showLocalLogin}>
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
          <Show when={!showLocalLogin && !hasSSOProviders && !demoModeEnabled}>
            <div class="space-y-2 text-sm">
              <h3 class="font-semibold text-base-content">No sign-in options on this page</h3>
              <p class="text-muted">
                The local sign-in form is hidden and Pulse didn’t list a single sign-on provider.
                Ask your Pulse administrator how to sign in.
              </p>
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
                  class="appearance-none relative block w-full pl-10 pr-3 py-3 border border-border placeholder-gray-500 rounded-md focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all sm:text-sm"
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
                  class="appearance-none relative block w-full pl-10 pr-3 py-3 border border-border placeholder-gray-500 rounded-md focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all sm:text-sm"
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
                  class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-border rounded-sm cursor-pointer"
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
                role="alert"
                aria-live="assertive"
                class={`rounded-md p-4 ${
                  error().includes('locked')
                    ? 'bg-orange-50 dark:bg-orange-900'
                    : 'bg-red-50 dark:bg-red-900/25'
                }`}
              >
                <div class="flex">
                  <div class="shrink-0">
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
                aria-disabled={loading()}
                onClick={(event) => {
                  // Keep focus, but skip native validation on a busy activation.
                  if (loading()) event.preventDefault();
                }}
                class="group relative w-full flex justify-center py-3 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-hidden focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 aria-disabled:bg-surface-hover aria-disabled:hover:bg-surface-hover aria-disabled:text-base-content aria-disabled:cursor-wait transition-colors"
              >
                <Show when={loading()}>
                  <LoadingSpinner size="button" class="-ml-1 mr-3" />
                </Show>
                <Show when={loading()} fallback="Sign in to Pulse">
                  Authenticating...
                </Show>
              </button>
              <span role="status" class="sr-only">
                {loading() ? 'Sign-in in progress.' : ''}
              </span>
            </div>
          </Show>
          <div class="border-t border-border pt-4 text-center">
            <a
              ref={props.helpRef}
              href={TROUBLESHOOTING_DOC_URL}
              class="inline-flex min-h-10 items-center rounded-sm px-1 text-xs font-medium text-blue-700 underline-offset-2 transition-colors hover:underline focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500/60 dark:text-blue-300 sm:min-h-9"
            >
              Can’t sign in?
            </a>
          </div>
        </form>
      </div>
    </div>
  );
};
