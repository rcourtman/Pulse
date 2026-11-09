import { Component, createSignal, Show, onCleanup } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';
import {
  apiErrorFromResponse,
  apiFetch,
  clearApiToken,
  getApiToken,
  getOrgID,
} from '@/utils/apiClient';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formField, labelClass, controlClass, formHelpText } from '@/components/shared/Form';

interface SecurityCredentials {
  username: string;
  password: string;
  apiToken?: string;
}

interface QuickSecuritySetupProps {
  onConfigured?: () => void;
  defaultUsername?: string;
  mode?: 'initial' | 'rotate';
  owner: string;
  disabled: boolean;
  onPendingChange: (pending: boolean) => void;
}

export const QuickSecuritySetup: Component<QuickSecuritySetupProps> = (props) => {
  const mode = props.mode ?? 'initial';
  const isRotation = mode === 'rotate';
  const [isSettingUp, setIsSettingUp] = createSignal(false);
  const [credentials, setCredentials] = createSignal<SecurityCredentials | null>(null);
  const [outcome, setOutcome] = createSignal<'confirmed' | 'unconfirmed' | null>(null);
  const [error, setError] = createSignal('');
  const [copied, setCopied] = createSignal<'username' | 'password' | 'token' | null>(null);
  const [useCustomPassword, setUseCustomPassword] = createSignal(false);
  const [customUsername, setCustomUsername] = createSignal(props.defaultUsername ?? 'admin');
  const [customPassword, setCustomPassword] = createSignal('');
  const [confirmPassword, setConfirmPassword] = createSignal('');
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;
  const owner = props.owner;
  const org = getOrgID() || 'default';
  let credentialAtAdmission = getApiToken();
  let controller: AbortController | undefined;
  let attempted: SecurityCredentials | null = null;
  let disposed = false;
  const isCurrent = () =>
    !disposed &&
    props.owner === owner &&
    (getOrgID() || 'default') === org &&
    getApiToken() === credentialAtAdmission;
  const controlsDisabled = () => props.disabled || isSettingUp();

  onCleanup(() => {
    disposed = true;
    controller?.abort();
    controller = undefined;
    attempted = null;
    credentialAtAdmission = null;
    setCredentials(null);
    setCustomPassword('');
    setConfirmPassword('');
    props.onPendingChange(false);
    if (copiedTimer) clearTimeout(copiedTimer);
  });

  const generatePassword = (length: number = 16): string => {
    // Avoid special chars that could cause issues with URLs or shell commands
    const charset = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-';
    let password = '';
    const array = new Uint8Array(length);
    crypto.getRandomValues(array);
    for (let i = 0; i < length; i++) {
      password += charset[array[i] % charset.length];
    }
    return password;
  };

  const generateToken = (): string => {
    // Generate 24 bytes (48 hex chars) to avoid hash detection issue with 64-char tokens
    const array = new Uint8Array(24);
    crypto.getRandomValues(array);
    return Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');
  };

  const handleCopy = async (text: string, type: 'username' | 'password' | 'token') => {
    const issued = credentials();
    if (!isCurrent() || props.disabled || !issued) return;
    const success = await copyToClipboard(text);
    if (!isCurrent() || props.disabled || credentials() !== issued) return;
    if (success) {
      setCopied(type);
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => {
        if (isCurrent() && credentials() === issued) setCopied(null);
      }, 2000);
    } else {
      showError('Failed to copy to clipboard');
    }
  };

  const setupSecurity = async () => {
    if (!isCurrent() || controlsDisabled() || credentials()) return;
    setError('');
    if (useCustomPassword()) {
      if (customPassword().length < 12) {
        setError('Password must be at least 12 characters');
        return;
      }
      if (customPassword() !== confirmPassword()) {
        setError('Passwords do not match');
        return;
      }
    }

    const request = new AbortController();
    controller = request;
    attempted = {
      username: customUsername(),
      password: useCustomPassword() ? customPassword() : generatePassword(),
      apiToken: generateToken(),
    };
    setIsSettingUp(true);
    props.onPendingChange(true);
    try {
      const response = await apiFetch('/api/security/quick-setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        expectedOrgID: org,
        signal: request.signal,
        retry: false,
        body: JSON.stringify({ ...attempted, force: isRotation }),
      });
      if (!isCurrent()) return;
      if (!response.ok) {
        if (response.status >= 500) throw new Error('Unconfirmed credential change');
        const refusal = await apiErrorFromResponse(response, 'Credential change was refused');
        if (isCurrent()) setError(refusal.message);
        return;
      }
      const result: unknown = await response.json();
      if (!isCurrent()) return;
      if (
        !result ||
        typeof result !== 'object' ||
        !('success' in result) ||
        result.success !== true
      )
        throw new Error('Unconfirmed credential change');
      if ('skipped' in result && result.skipped === true) {
        showError('Security is already configured. No credentials were changed.');
        props.onConfigured?.();
        return;
      }

      setCredentials(attempted);
      setOutcome('confirmed');
      setCustomPassword('');
      setConfirmPassword('');
      // The server has replaced the old API tokens and established a session.
      // Drop only this reader's obsolete token, not its selected organisation.
      clearApiToken();
      credentialAtAdmission = null;
      showSuccess('Credentials are active. Save them before continuing.');
    } catch {
      if (!isCurrent()) return;
      // The password may already have been saved even if token persistence or
      // response delivery failed. Keep the attempted values for this reader;
      // neither claim success nor generate replacements with a blind retry.
      setCredentials(attempted);
      setOutcome('unconfirmed');
      setCustomPassword('');
      setConfirmPassword('');
    } finally {
      if (controller === request) {
        controller = undefined;
        attempted = null;
        setIsSettingUp(false);
        props.onPendingChange(false);
      }
    }
  };

  const downloadCredentials = () => {
    const saved = credentials();
    if (!isCurrent() || props.disabled || !saved) return;

    const content = `Pulse Admin Credentials ${isRotation ? '(Rotated)' : ''}
Generated: ${new Date().toISOString()}
Status: ${outcome() === 'confirmed' ? 'Confirmed active' : 'Unconfirmed - check sign-in and token before relying on them'}

Basic Authentication:
Username: ${saved.username}
Password: ${saved.password}

API Token: ${saved.apiToken}

Important:
- Save these credentials securely
- They will not be shown again
- Use the API token for export/import operations
- Use the administrator username/password for local sign-in
`;

    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `pulse-credentials-${Date.now()}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div class="bg-surface rounded-md border border-border p-6">
      <Show when={!credentials()}>
        <div class="space-y-4">
          <div class="flex items-start space-x-3">
            <div class="shrink-0">
              <svg
                class="h-6 w-6 text-blue-600 dark:text-blue-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 10V3L4 14h7v7l9-11h-7z"
                />
              </svg>
            </div>
            <div class="flex-1">
              <SectionHeader
                title={isRotation ? 'Generate new admin credentials' : 'Quick security setup'}
                description={
                  isRotation
                    ? 'Create a fresh password and API token. This will:'
                    : 'Enable authentication with one click. This will:'
                }
                size="sm"
                titleClass="text-base-content"
                descriptionClass="text-xs! text-muted"
              />
              <ul class="mt-2 space-y-1 text-xs text-muted">
                <li class="flex items-center">
                  <span class="text-emerald-400 mr-2">✓</span>
                  {isRotation
                    ? 'Generate a new secure password'
                    : 'Generate secure random password'}
                </li>
                <li class="flex items-center">
                  <span class="text-emerald-400 mr-2">✓</span>
                  {isRotation ? 'Replace the stored admin password' : 'Enable basic authentication'}
                </li>
                <li class="flex items-center">
                  <span class="text-emerald-400 mr-2">✓</span>
                  {isRotation
                    ? 'Replace all API tokens (full access)'
                    : 'Replace any API tokens (full access)'}
                </li>
                <li class="flex items-center">
                  <span class="text-emerald-400 mr-2">✓</span>
                  Enable audit logging
                </li>
              </ul>
            </div>
          </div>

          <fieldset disabled={controlsDisabled()} class="min-w-0 bg-page rounded-md p-3 space-y-3">
            <div class="flex items-center justify-between">
              <span class={labelClass()}>Password Setup</span>
              <div class="flex items-center space-x-2">
                <button
                  type="button"
                  onClick={() => setUseCustomPassword(false)}
                  class={`px-3 py-1 text-xs rounded-md transition-colors ${
                    !useCustomPassword()
                      ? 'bg-blue-600 text-white'
                      : 'bg-surface-hover text-base-content hover:bg-surface-hover'
                  }`}
                >
                  Auto-Generate
                </button>
                <button
                  type="button"
                  onClick={() => setUseCustomPassword(true)}
                  class={`px-3 py-1 text-xs rounded-md transition-colors ${
                    useCustomPassword()
                      ? 'bg-blue-600 text-white'
                      : 'bg-surface-hover text-base-content hover:bg-surface-hover'
                  }`}
                >
                  Custom
                </button>
              </div>
            </div>

            <Show when={useCustomPassword()}>
              <div class="space-y-2">
                <div class={formField}>
                  <label for="quick-security-username" class={labelClass()}>
                    Username
                  </label>
                  <input
                    id="quick-security-username"
                    type="text"
                    value={customUsername()}
                    onInput={(e) => setCustomUsername(e.currentTarget.value)}
                    class={controlClass()}
                    placeholder="admin"
                  />
                </div>
                <div class={formField}>
                  <label for="quick-security-password" class={labelClass()}>
                    Password (min 12 characters)
                  </label>
                  <input
                    id="quick-security-password"
                    type="password"
                    value={customPassword()}
                    onInput={(e) => setCustomPassword(e.currentTarget.value)}
                    class={controlClass()}
                    placeholder="Enter password"
                  />
                </div>
                <div class={formField}>
                  <label for="quick-security-confirm-password" class={labelClass()}>
                    Confirm password
                  </label>
                  <input
                    id="quick-security-confirm-password"
                    type="password"
                    value={confirmPassword()}
                    onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                    class={controlClass()}
                    placeholder="Confirm password"
                  />
                </div>
              </div>
            </Show>

            <Show when={!useCustomPassword()}>
              <p class={formHelpText}>A secure 16-character password will be generated for you</p>
            </Show>
          </fieldset>

          <div class="bg-yellow-50 dark:bg-yellow-900/25 border border-yellow-200 dark:border-yellow-800 rounded-md p-3">
            <div class="flex">
              <svg
                class="h-5 w-5 text-yellow-600 dark:text-yellow-400 mr-2 shrink-0"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <div class="text-xs text-yellow-700 dark:text-yellow-300">
                <p class="font-semibold">Important:</p>
                <p>
                  {useCustomPassword()
                    ? 'Your password will be hashed before storage'
                    : 'Credentials will be shown only once. Save them immediately!'}
                </p>
                <Show when={isRotation}>
                  <p class="mt-1">
                    Active immediately. Update agents and automation with the new token.
                  </p>
                </Show>
              </div>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <Show when={error()}>
              <p role="alert" class="min-w-0 flex-1 text-xs text-red-600 dark:text-red-400">
                {error()}
              </p>
            </Show>
            <button
              type="button"
              onClick={setupSecurity}
              disabled={controlsDisabled()}
              classList={{ 'w-full': !error(), 'shrink-0': !!error() }}
              class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {isSettingUp() ? (
                <span class="flex items-center justify-center">
                  <svg
                    class="animate-spin -ml-1 mr-2 h-4 w-4 text-white"
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
                  {isRotation ? 'Rotating credentials...' : 'Setting up security...'}
                </span>
              ) : isRotation ? (
                'Rotate credentials'
              ) : (
                'Enable Security Now'
              )}
            </button>
          </div>
        </div>
      </Show>

      <Show when={credentials()}>
        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <SectionHeader
              title={
                outcome() === 'unconfirmed'
                  ? 'Credential change unconfirmed'
                  : isRotation
                    ? 'Admin credentials generated'
                    : 'Security enabled successfully'
              }
              size="md"
              class="flex-1"
              titleClass="text-base-content"
            />
            <button
              type="button"
              onClick={downloadCredentials}
              disabled={props.disabled}
              class="px-3 py-1 text-xs bg-green-600 text-white rounded-sm hover:bg-green-700 transition-colors"
            >
              Download credentials
            </button>
          </div>

          <InlineNotice tone={outcome() === 'unconfirmed' ? 'warning' : 'success'} role="status">
            <p class="font-semibold">Save these credentials now - they won't be shown again!</p>
          </InlineNotice>

          <div class="space-y-3">
            <div class="bg-page rounded-md p-3">
              <span class={labelClass('text-xs')}>Username</span>
              <div class="mt-1 flex items-center gap-2">
                <code class="flex-1 font-mono text-sm bg-surface px-3 py-2 rounded-sm border border-border">
                  {credentials()!.username}
                </code>
                <button
                  type="button"
                  onClick={() => handleCopy(credentials()!.username, 'username')}
                  disabled={props.disabled}
                  class="px-3 py-2 text-xs bg-slate-600 text-white rounded-sm hover:bg-slate-700 transition-colors"
                >
                  {copied() === 'username' ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>

            <div class="bg-page rounded-md p-3">
              <span class={labelClass('text-xs')}>Password</span>
              <div class="mt-1 flex items-center gap-2">
                <code class="flex-1 font-mono text-sm bg-surface px-3 py-2 rounded-sm border border-border break-all">
                  {credentials()!.password}
                </code>
                <button
                  type="button"
                  onClick={() => handleCopy(credentials()!.password, 'password')}
                  disabled={props.disabled}
                  class="px-3 py-2 text-xs bg-slate-600 text-white rounded-sm hover:bg-slate-700 transition-colors"
                >
                  {copied() === 'password' ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>

            <div class="bg-page rounded-md p-3">
              <span class={labelClass('text-xs')}>API token</span>
              <div class="mt-1 flex items-center gap-2">
                <code class="flex-1 font-mono text-sm bg-surface px-3 py-2 rounded-sm border border-border break-all">
                  {credentials()!.apiToken}
                </code>
                <button
                  type="button"
                  onClick={() => handleCopy(credentials()!.apiToken!, 'token')}
                  disabled={props.disabled}
                  class="px-3 py-2 text-xs bg-slate-600 text-white rounded-sm hover:bg-slate-700 transition-colors"
                >
                  {copied() === 'token' ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <p class={formHelpText + ' mt-2'}>
                Use this token with the X-API-Token header for automation.
              </p>
              <p class="mt-1 text-xs font-semibold text-red-600 dark:text-red-400">
                This token is only shown once. Save it now.
              </p>
            </div>
          </div>

          <InlineNotice tone={outcome() === 'unconfirmed' ? 'warning' : 'success'}>
            <Show
              when={outcome() === 'unconfirmed'}
              fallback={
                <>
                  <p class="font-semibold">Credentials are active now</p>
                  <p class="text-xs">
                    No restart is needed. Use the new token for agents and automation.
                  </p>
                </>
              }
            >
              <p class="font-semibold">The password or token may have changed.</p>
              <p class="text-xs">
                Check sign-in in a separate window and verify the token before relying on these
                credentials. Do not repeat the change until you have checked the result.
              </p>
            </Show>
          </InlineNotice>

          <div class="flex justify-end">
            <button
              type="button"
              onClick={() => {
                if (!isCurrent() || props.disabled) return;
                const confirmed = outcome() === 'confirmed';
                setCredentials(null);
                props.onConfigured?.();
                if (confirmed) window.location.reload();
              }}
              disabled={props.disabled}
              class="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 transition-colors"
            >
              {outcome() === 'unconfirmed'
                ? "Close - I've Saved These Credentials"
                : "Done - I've Saved My Credentials"}
            </button>
          </div>
        </div>
      </Show>
    </div>
  );
};
