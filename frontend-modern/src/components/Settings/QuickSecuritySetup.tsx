import { Component, createSignal, Show } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';
import { clearAuth as clearApiClientAuth } from '@/utils/apiClient';
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
}

export const QuickSecuritySetup: Component<QuickSecuritySetupProps> = (props) => {
  const mode = props.mode ?? 'initial';
  const isRotation = mode === 'rotate';
  const [isSettingUp, setIsSettingUp] = createSignal(false);
  const [credentials, setCredentials] = createSignal<SecurityCredentials | null>(null);
  const [showCredentials, setShowCredentials] = createSignal(false);
  const [copied, setCopied] = createSignal<'username' | 'password' | 'token' | null>(null);
  const [useCustomPassword, setUseCustomPassword] = createSignal(false);
  const [customUsername, setCustomUsername] = createSignal(props.defaultUsername ?? 'admin');
  const [customPassword, setCustomPassword] = createSignal('');
  const [confirmPassword, setConfirmPassword] = createSignal('');

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
    const success = await copyToClipboard(text);
    if (success) {
      setCopied(type);
      setTimeout(() => setCopied(null), 2000);
    } else {
      showError('Failed to copy to clipboard');
    }
  };

  const setupSecurity = async () => {
    // Validate custom password if using
    if (useCustomPassword()) {
      if (customPassword().length < 12) {
        showError('Password must be at least 12 characters');
        return;
      }
      if (customPassword() !== confirmPassword()) {
        showError('Passwords do not match');
        return;
      }
    }

    setIsSettingUp(true);


    try {
      // Generate or use custom credentials
      const newCredentials: SecurityCredentials = {
        username: customUsername(),
        password: useCustomPassword() ? customPassword() : generatePassword(),
        apiToken: generateToken(),
      };

      // Get CSRF token from cookie for authenticated requests (rotation mode)
      const csrfToken = document.cookie
        .split('; ')
        .find((row) => row.startsWith('pulse_csrf='))
        ?.split('=')[1];

      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      if (csrfToken) {
        headers['X-CSRF-Token'] = csrfToken;
      }

      // Call API to enable security
      const response = await fetch('/api/security/quick-setup', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          ...newCredentials,
          force: isRotation,
        }),
        credentials: 'include', // Include cookies for CSRF
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(error || 'Failed to setup security');
      }

      // Parse response to check if setup was skipped
      const result = await response.json();

      if (result.skipped) {
        // Security was already configured, don't show credentials
        showError(
          result.message ||
          'Security is already configured. Please remove existing security first if you want to reconfigure.',
        );
        if (props.onConfigured) {
          props.onConfigured();
        }
        return;
      }

      // Response is successful and security was newly configured
      setCredentials(newCredentials);
      setShowCredentials(true);

      clearApiClientAuth();

      // Show success message
      showSuccess(
        isRotation
          ? 'Admin credentials generated. Save them before continuing.'
          : 'Security configured. Save your credentials before continuing.',
      );

      // DON'T notify parent yet - wait until user dismisses credentials
    } catch (error) {
      showError(`Failed to setup security: ${error}`);
    } finally {
      setIsSettingUp(false);
    }
  };

  const downloadCredentials = () => {
    if (!credentials()) return;

    const content = `Pulse Admin Credentials ${isRotation ? '(Rotated)' : ''}
Generated: ${new Date().toISOString()}

Basic Authentication:
Username: ${credentials()!.username}
Password: ${credentials()!.password}

API Token: ${credentials()!.apiToken}

Important:
- Save these credentials securely
- They will not be shown again
- Use the API token for export/import operations
- Basic auth is required to access the web interface
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
    <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6">
      <Show when={!showCredentials()}>
        <div class="space-y-4">
          <div class="flex items-start space-x-3">
            <div class="flex-shrink-0">
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
                titleClass="text-gray-900 dark:text-gray-100"
                descriptionClass="!text-xs text-gray-600 dark:text-gray-400"
              />
              <ul class="mt-2 space-y-1 text-xs text-gray-600 dark:text-gray-400">
                <li class="flex items-center">
                  <span class="text-green-500 mr-2">✓</span>
                  {isRotation
                    ? 'Generate a new secure password'
                    : 'Generate secure random password'}
                </li>
                <li class="flex items-center">
                  <span class="text-green-500 mr-2">✓</span>
                  {isRotation ? 'Replace the stored admin password' : 'Enable basic authentication'}
                </li>
                <li class="flex items-center">
                  <span class="text-green-500 mr-2">✓</span>
                  {isRotation
                    ? 'Create a new API token for automation'
                    : 'Create API token for automation'}
                </li>
                <li class="flex items-center">
                  <span class="text-green-500 mr-2">✓</span>
                  Enable audit logging
                </li>
              </ul>
            </div>
          </div>

          <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3 space-y-3">
            <div class="flex items-center justify-between">
              <label class={labelClass()}>Password Setup</label>
              <div class="flex items-center space-x-2">
                <button
                  type="button"
                  onClick={() => setUseCustomPassword(false)}
                  class={`px-3 py-1 text-xs rounded-lg transition-colors ${!useCustomPassword()
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                    }`}
                >
                  Auto-Generate
                </button>
                <button
                  type="button"
                  onClick={() => setUseCustomPassword(true)}
                  class={`px-3 py-1 text-xs rounded-lg transition-colors ${useCustomPassword()
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                    }`}
                >
                  Custom
                </button>
              </div>
            </div>

            <Show when={useCustomPassword()}>
              <div class="space-y-2">
                <div class={formField}>
                  <label class={labelClass()}>Username</label>
                  <input
                    type="text"
                    value={customUsername()}
                    onInput={(e) => setCustomUsername(e.currentTarget.value)}
                    class={controlClass()}
                    placeholder="admin"
                  />
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Password (min 12 characters)</label>
                  <input
                    type="password"
                    value={customPassword()}
                    onInput={(e) => setCustomPassword(e.currentTarget.value)}
                    class={controlClass()}
                    placeholder="Enter password"
                  />
                </div>
                <div class={formField}>
                  <label class={labelClass()}>Confirm password</label>
                  <input
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
          </div>

          <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3">
            <div class="flex">
              <svg
                class="h-5 w-5 text-yellow-600 dark:text-yellow-400 mr-2 flex-shrink-0"
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
                    Existing sessions will be logged out once Pulse restarts with the new
                    credentials.
                  </p>
                </Show>
              </div>
            </div>
          </div>

          <button
            type="button"
            onClick={setupSecurity}
            disabled={isSettingUp()}
            class="w-full px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
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
      </Show>

      <Show when={showCredentials() && credentials()}>
        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <SectionHeader
              title={isRotation ? 'Admin credentials generated' : 'Security enabled successfully'}
              size="md"
              class="flex-1"
              titleClass="text-gray-900 dark:text-gray-100"
            />
            <button
              type="button"
              onClick={downloadCredentials}
              class="px-3 py-1 text-xs bg-green-600 text-white rounded hover:bg-green-700 transition-colors"
            >
              Download credentials
            </button>
          </div>

          <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-3">
            <p class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
              Save these credentials now - they won't be shown again!
            </p>
          </div>

          <div class="space-y-3">
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
              <label class={labelClass('text-xs')}>Username</label>
              <div class="mt-1 flex items-center gap-2">
                <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700">
                  {credentials()!.username}
                </code>
                <button
                  type="button"
                  onClick={() => handleCopy(credentials()!.username, 'username')}
                  class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                >
                  {copied() === 'username' ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>

            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
              <label class={labelClass('text-xs')}>Password</label>
              <div class="mt-1 flex items-center gap-2">
                <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                  {credentials()!.password}
                </code>
                <button
                  type="button"
                  onClick={() => handleCopy(credentials()!.password, 'password')}
                  class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                >
                  {copied() === 'password' ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>

            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
              <label class={labelClass('text-xs')}>API token</label>
              <div class="mt-1 flex items-center gap-2">
                <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                  {credentials()!.apiToken}
                </code>
                <button
                  type="button"
                  onClick={() => handleCopy(credentials()!.apiToken!, 'token')}
                  class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
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

          <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-3">
            <p class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
              Credentials saved
            </p>
            <p class="text-xs text-green-700 dark:text-green-300">
              The service needs to be restarted for security settings to take effect.
            </p>
            <p class="text-xs text-green-600 dark:text-green-400 mt-2 italic">
              Save your credentials above - they won't be shown again.
            </p>
          </div>

          <div class="flex justify-end">
            <button
              type="button"
              onClick={() => {
                setShowCredentials(false);
                // Now notify parent that configuration is complete
                if (props.onConfigured) {
                  props.onConfigured();
                }
                // Reload the page to trigger login screen
                window.location.reload();
              }}
              class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 transition-colors"
            >
              Done - I've Saved My Credentials
            </button>
          </div>
        </div>
      </Show>
    </div>
  );
};
