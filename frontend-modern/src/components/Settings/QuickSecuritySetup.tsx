import { Component, createSignal, Show } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';

interface SecurityCredentials {
  username: string;
  password: string;
  apiToken?: string;
}

export const QuickSecuritySetup: Component = () => {
  const [isSettingUp, setIsSettingUp] = createSignal(false);
  const [credentials, setCredentials] = createSignal<SecurityCredentials | null>(null);
  const [showCredentials, setShowCredentials] = createSignal(false);
  const [copied, setCopied] = createSignal<'username' | 'password' | 'token' | null>(null);
  const [readyToRestart, setReadyToRestart] = createSignal(false);
  const [isRestarting, setIsRestarting] = createSignal(false);
  const [useCustomPassword, setUseCustomPassword] = createSignal(false);
  const [customUsername, setCustomUsername] = createSignal('admin');
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
    const array = new Uint8Array(32);
    crypto.getRandomValues(array);
    return Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
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

  const restartPulse = async () => {
    setIsRestarting(true);
    try {
      const response = await fetch('/api/security/apply-restart', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include'  // Include cookies for CSRF token
      });

      if (!response.ok) {
        throw new Error('Failed to restart Pulse');
      }

      showSuccess('Restarting Pulse... You will be redirected to login.');
      
      // Wait for restart then redirect to login
      setTimeout(() => {
        // Just reload - the auth check in App.tsx will show the login page
        window.location.reload();
      }, 5000);
    } catch (error) {
      showError(`Failed to restart: ${error}`);
      setIsRestarting(false);
    }
  };

  const setupSecurity = async () => {
    // Validate custom password if using
    if (useCustomPassword()) {
      if (customPassword().length < 8) {
        showError('Password must be at least 8 characters');
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
        apiToken: generateToken()
      };

      // Call API to enable security
      const response = await fetch('/api/security/quick-setup', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(newCredentials),
        credentials: 'include'  // Include cookies for CSRF
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(error || 'Failed to setup security');
      }

      const result = await response.json();
      setCredentials(newCredentials);
      setShowCredentials(true);
      
      // Store the API token in sessionStorage so it can be retrieved later in this session
      if (newCredentials.apiToken) {
        sessionStorage.setItem('pulse_last_api_token', newCredentials.apiToken);
      }
      
      // Store the command if manual action needed
      if (result.command) {
        (window as any).securityCommand = result.command;
      }
      
      // Check if we can auto-restart
      if (result.readyToRestart) {
        setReadyToRestart(true);
        showSuccess('Security configured! Save your credentials, then click "Restart Pulse" to apply.');
      } else if (result.method === 'systemd' && !result.automatic) {
        showSuccess('Security configured! Run the command shown below to apply settings.');
      } else if (result.method === 'docker') {
        showSuccess('Security configured! Please restart your Docker container with the credentials shown.');
      } else {
        showSuccess('Security configured! Please restart Pulse to apply settings.');
      }
    } catch (error) {
      showError(`Failed to setup security: ${error}`);
    } finally {
      setIsSettingUp(false);
    }
  };

  const downloadCredentials = () => {
    if (!credentials()) return;
    
    const content = `Pulse Security Credentials
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
              <svg class="h-6 w-6 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            </div>
            <div class="flex-1">
              <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Quick Security Setup</h4>
              <p class="text-xs text-gray-600 dark:text-gray-400 mt-1">
                Enable authentication with one click. This will:
              </p>
              <ul class="mt-2 space-y-1 text-xs text-gray-600 dark:text-gray-400">
                <li class="flex items-center">
                  <span class="text-green-500 mr-2">✓</span>
                  Generate secure random password
                </li>
                <li class="flex items-center">
                  <span class="text-green-500 mr-2">✓</span>
                  Enable basic authentication
                </li>
                <li class="flex items-center">
                  <span class="text-green-500 mr-2">✓</span>
                  Create API token for automation
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
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                Password Setup
              </label>
              <div class="flex items-center space-x-2">
                <button
                  onClick={() => setUseCustomPassword(false)}
                  class={`px-3 py-1 text-xs rounded-lg transition-colors ${
                    !useCustomPassword() 
                      ? 'bg-blue-600 text-white' 
                      : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                  }`}
                >
                  Auto-Generate
                </button>
                <button
                  onClick={() => setUseCustomPassword(true)}
                  class={`px-3 py-1 text-xs rounded-lg transition-colors ${
                    useCustomPassword() 
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
                <div>
                  <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Username
                  </label>
                  <input
                    type="text"
                    value={customUsername()}
                    onInput={(e) => setCustomUsername(e.currentTarget.value)}
                    class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    placeholder="admin"
                  />
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Password (min 8 characters)
                  </label>
                  <input
                    type="password"
                    value={customPassword()}
                    onInput={(e) => setCustomPassword(e.currentTarget.value)}
                    class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    placeholder="Enter password"
                  />
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Confirm Password
                  </label>
                  <input
                    type="password"
                    value={confirmPassword()}
                    onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                    class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    placeholder="Confirm password"
                  />
                </div>
              </div>
            </Show>

            <Show when={!useCustomPassword()}>
              <p class="text-xs text-gray-600 dark:text-gray-400">
                A secure 16-character password will be generated for you
              </p>
            </Show>
          </div>

          <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3">
            <div class="flex">
              <svg class="h-5 w-5 text-yellow-600 dark:text-yellow-400 mr-2 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <div class="text-xs text-yellow-700 dark:text-yellow-300">
                <p class="font-semibold">Important:</p>
                <p>{useCustomPassword() 
                  ? 'Your password will be hashed before storage' 
                  : 'Credentials will be shown only once. Save them immediately!'}</p>
              </div>
            </div>
          </div>

          <button
            onClick={setupSecurity}
            disabled={isSettingUp()}
            class="w-full px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isSettingUp() ? (
              <span class="flex items-center justify-center">
                <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Setting up security...
              </span>
            ) : (
              'Enable Security Now'
            )}
          </button>
        </div>
      </Show>

      <Show when={showCredentials() && credentials()}>
        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <h4 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
              🎉 Security Enabled Successfully!
            </h4>
            <button
              onClick={downloadCredentials}
              class="px-3 py-1 text-xs bg-green-600 text-white rounded hover:bg-green-700 transition-colors"
            >
              Download Credentials
            </button>
          </div>

          <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-3">
            <p class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
              Save these credentials now - they won't be shown again!
            </p>
          </div>

          <div class="space-y-3">
            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Username</label>
              <div class="flex items-center space-x-2">
                <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700">
                  {credentials()!.username}
                </code>
                <button
                  onClick={() => handleCopy(credentials()!.username, 'username')}
                  class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                >
                  {copied() === 'username' ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>

            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Password</label>
              <div class="flex items-center space-x-2">
                <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                  {credentials()!.password}
                </code>
                <button
                  onClick={() => handleCopy(credentials()!.password, 'password')}
                  class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                >
                  {copied() === 'password' ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>

            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
              <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">API Token</label>
              <div class="flex items-center space-x-2">
                <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                  {credentials()!.apiToken}
                </code>
                <button
                  onClick={() => handleCopy(credentials()!.apiToken!, 'token')}
                  class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                >
                  {copied() === 'token' ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                This is your API token for automation. Use with X-API-Token header.
              </p>
            </div>
          </div>

          <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-3">
            <Show 
              when={readyToRestart()}
              fallback={
                <Show 
                  when={(window as any).securityCommand}
                  fallback={
                    <>
                      <p class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
                        ✅ Security configured successfully!
                      </p>
                      <p class="text-xs text-green-700 dark:text-green-300">
                        Save your credentials above. Pulse will apply the security settings.
                      </p>
                    </>
                  }
                >
                  <p class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
                    ✅ One more step to enable security:
                  </p>
                  <p class="text-xs text-green-700 dark:text-green-300 mb-2">
                    Run this command in your terminal:
                  </p>
                  <div class="bg-gray-900 text-green-400 p-2 rounded font-mono text-xs overflow-x-auto">
                    {(window as any).securityCommand}
                  </div>
                  <p class="text-xs text-green-700 dark:text-green-300 mt-2">
                    This will apply the settings and restart Pulse with security enabled.
                  </p>
                </Show>
              }
            >
              <p class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
                ✅ Security configured! Ready to apply.
              </p>
              <p class="text-xs text-green-700 dark:text-green-300 mb-3">
                Make sure you've saved your credentials above before restarting.
              </p>
              <button
                onClick={restartPulse}
                disabled={isRestarting()}
                class="w-full px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {isRestarting() ? (
                  <span class="flex items-center justify-center">
                    <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Restarting Pulse...
                  </span>
                ) : (
                  'Restart Pulse to Apply Security'
                )}
              </button>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};