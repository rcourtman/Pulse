import { Component, createSignal, Show, onMount } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';
import { STORAGE_KEYS } from '@/constants';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { showTokenReveal } from '@/stores/tokenReveal';
import type { APITokenRecord } from '@/api/security';

export const FirstRunSetup: Component = () => {
  const [username, setUsername] = createSignal('admin');
  const [password, setPassword] = createSignal('');
  const [confirmPassword, setConfirmPassword] = createSignal('');
  const [useCustomPassword, setUseCustomPassword] = createSignal(false);
  const [generatedPassword, setGeneratedPassword] = createSignal('');
  const [, setApiToken] = createSignal('');
  const [isSettingUp, setIsSettingUp] = createSignal(false);
  const [showCredentials, setShowCredentials] = createSignal(false);
  const [savedUsername, setSavedUsername] = createSignal('');
  const [savedPassword, setSavedPassword] = createSignal('');
  const [savedToken, setSavedToken] = createSignal('');
  const [copied, setCopied] = createSignal<'password' | 'token' | null>(null);
  const [themeMode, setThemeMode] = createSignal<'system' | 'light' | 'dark'>('system');

  const applyTheme = (mode: 'system' | 'light' | 'dark') => {
    if (mode === 'light') {
      document.documentElement.classList.remove('dark');
      localStorage.setItem(STORAGE_KEYS.DARK_MODE, 'false');
    } else if (mode === 'dark') {
      document.documentElement.classList.add('dark');
      localStorage.setItem(STORAGE_KEYS.DARK_MODE, 'true');
    } else {
      // System preference
      localStorage.removeItem(STORAGE_KEYS.DARK_MODE);
      if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
        document.documentElement.classList.add('dark');
      } else {
        document.documentElement.classList.remove('dark');
      }
    }
  };

  onMount(() => {
    // Check for saved theme preference
    const savedTheme = localStorage.getItem(STORAGE_KEYS.DARK_MODE);
    if (savedTheme === 'false') {
      setThemeMode('light');
      document.documentElement.classList.remove('dark');
    } else if (savedTheme === 'true') {
      setThemeMode('dark');
      document.documentElement.classList.add('dark');
    } else {
      // No saved preference - use system preference
      setThemeMode('system');
      if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
        document.documentElement.classList.add('dark');
      } else {
        document.documentElement.classList.remove('dark');
      }
    }
  });

  const generatePassword = () => {
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!@#$%';
    let pass = '';
    for (let i = 0; i < 16; i++) {
      pass += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return pass;
  };

  const generateToken = (): string => {
    // Generate 24 bytes (48 hex chars) to avoid hash detection issue
    const array = new Uint8Array(24);
    crypto.getRandomValues(array);
    return Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');
  };

  const handleSetup = async () => {
    // Validate custom password if used
    if (useCustomPassword()) {
      if (!password()) {
        showError('Please enter a password');
        return;
      }
      if (password() !== confirmPassword()) {
        showError('Passwords do not match');
        return;
      }
      if (password().length < 8) {
        showError('Password must be at least 8 characters');
        return;
      }
    }

    setIsSettingUp(true);

    // Generate password if not custom
    const finalPassword = useCustomPassword() ? password() : generatePassword();
    if (!useCustomPassword()) {
      setGeneratedPassword(finalPassword);
    }

    // Generate API token
    const token = generateToken();
    setApiToken(token);

    try {
      const response = await fetch('/api/security/quick-setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: username(),
          password: finalPassword,
          apiToken: token,
        }),
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(error || 'Failed to setup security');
      }

      const result = await response.json();

      if (result.skipped) {
        // Shouldn't happen in first-run, but handle it
        window.location.reload();
        return;
      }

      // Save credentials for display
      setSavedUsername(username());
      setSavedPassword(useCustomPassword() ? password() : generatedPassword());
      setSavedToken(token);

      const bootstrapRecord: APITokenRecord = {
        id: 'bootstrap-token',
        name: 'Bootstrap token',
        prefix: token.slice(0, 6),
        suffix: token.slice(-4),
        createdAt: new Date().toISOString(),
      };
      showTokenReveal({
        token,
        record: bootstrapRecord,
        source: 'first-run',
        note: 'Copy this bootstrap token now. It unlocks API access for agents and automations.',
      });

      // Show credentials
      setShowCredentials(true);
      showSuccess('Security configured successfully!');
    } catch (error) {
      showError(`Failed to setup security: ${error}`);
    } finally {
      setIsSettingUp(false);
    }
  };

  const handleCopy = async (type: 'password' | 'token') => {
    const value = type === 'password' ? savedPassword() : savedToken();
    const success = await copyToClipboard(value);
    if (success) {
      setCopied(type);
      setTimeout(() => setCopied(null), 2000);
    }
  };

  const downloadCredentials = () => {
    const credentials = `Pulse Security Credentials
========================
Generated: ${new Date().toISOString()}

Web Interface Login:
-------------------
URL: ${window.location.origin}
Username: ${savedUsername()}
Password: ${savedPassword()}

API Access:
-----------
API Token: ${savedToken()}

Example API Usage:
curl -H "X-API-Token: ${savedToken()}" ${window.location.origin}/api/state

IMPORTANT: Keep these credentials secure!
`;

    const blob = new Blob([credentials], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `pulse-credentials-${Date.now()}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <div class="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800 flex items-center justify-center p-4">
      <div class="w-full max-w-2xl">
        {/* Logo/Header */}
        <div class="text-center mb-8">
          <div class="flex items-center justify-center gap-2 mb-4">
            <svg
              width="48"
              height="48"
              viewBox="0 0 256 256"
              xmlns="http://www.w3.org/2000/svg"
              class="pulse-logo"
            >
              <title>Pulse Logo</title>
              <circle class="pulse-bg fill-blue-600 dark:fill-blue-500" cx="128" cy="128" r="122" />
              <circle
                class="pulse-ring fill-none stroke-white stroke-[14] opacity-[0.92]"
                cx="128"
                cy="128"
                r="84"
              />
              <circle
                class="pulse-center fill-white dark:fill-[#dbeafe]"
                cx="128"
                cy="128"
                r="26"
              />
            </svg>
            <span class="text-4xl font-bold text-gray-800 dark:text-gray-100">Pulse</span>
          </div>
          <p class="text-gray-600 dark:text-gray-400">Let's set up your monitoring dashboard</p>
        </div>

        <div class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl overflow-hidden">
          <Show when={!showCredentials()}>
            <div class="p-8">
              <SectionHeader
                title="Initial security setup"
                size="lg"
                class="mb-6"
                titleClass="text-gray-800 dark:text-gray-100"
              />

              <div class="space-y-6">
                {/* Username */}
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Admin Username
                  </label>
                  <input
                    type="text"
                    value={username()}
                    onInput={(e) => setUsername(e.currentTarget.value)}
                    class="w-full px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    placeholder="admin"
                  />
                </div>

                {/* Password Setup */}
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Admin Password
                  </label>

                  <div class="flex gap-2 mb-3">
                    <button
                      type="button"
                      onClick={() => setUseCustomPassword(false)}
                      class={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
                        !useCustomPassword()
                          ? 'bg-blue-600 text-white'
                          : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                      }`}
                    >
                      Generate Secure Password
                    </button>
                    <button
                      type="button"
                      onClick={() => setUseCustomPassword(true)}
                      class={`flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
                        useCustomPassword()
                          ? 'bg-blue-600 text-white'
                          : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                      }`}
                    >
                      Set Custom Password
                    </button>
                  </div>

                  <Show when={useCustomPassword()}>
                    <div class="space-y-3">
                      <input
                        type="password"
                        value={password()}
                        onInput={(e) => setPassword(e.currentTarget.value)}
                        class="w-full px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        placeholder="Enter password (min 8 characters)"
                      />
                      <input
                        type="password"
                        value={confirmPassword()}
                        onInput={(e) => setConfirmPassword(e.currentTarget.value)}
                        class="w-full px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        placeholder="Confirm password"
                      />
                    </div>
                  </Show>

                  <Show when={!useCustomPassword()}>
                    <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
                      <p class="text-sm text-blue-700 dark:text-blue-300">
                        A secure 16-character password will be generated for you. Make sure to save
                        it when shown!
                      </p>
                    </div>
                  </Show>
                </div>

                {/* Theme Selection */}
                <div>
                  <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                    Theme Preference
                  </label>
                  <div class="grid grid-cols-3 gap-2">
                    <button
                      type="button"
                      onClick={() => {
                        setThemeMode('system');
                        applyTheme('system');
                      }}
                      class={`py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
                        themeMode() === 'system'
                          ? 'bg-blue-600 text-white'
                          : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                      }`}
                    >
                      System
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setThemeMode('light');
                        applyTheme('light');
                      }}
                      class={`py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
                        themeMode() === 'light'
                          ? 'bg-blue-600 text-white'
                          : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                      }`}
                    >
                      Light
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setThemeMode('dark');
                        applyTheme('dark');
                      }}
                      class={`py-2 px-4 rounded-lg text-sm font-medium transition-colors ${
                        themeMode() === 'dark'
                          ? 'bg-blue-600 text-white'
                          : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                      }`}
                    >
                      Dark
                    </button>
                  </div>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
                    {themeMode() === 'system'
                      ? 'Using your operating system theme preference'
                      : `Using ${themeMode()} mode`}
                  </p>
                </div>

                {/* Info Box */}
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 space-y-2">
                  <SectionHeader
                    title="What happens next"
                    size="sm"
                    titleClass="text-gray-800 dark:text-gray-200"
                  />
                  <ul class="text-sm text-gray-600 dark:text-gray-400 space-y-1">
                    <li class="flex items-start">
                      <span class="text-green-500 mr-2">✓</span>
                      <span>Your admin account will be created</span>
                    </li>
                    <li class="flex items-start">
                      <span class="text-green-500 mr-2">✓</span>
                      <span>An API token will be generated for automation</span>
                    </li>
                    <li class="flex items-start">
                      <span class="text-green-500 mr-2">✓</span>
                      <span>All API endpoints will be protected</span>
                    </li>
                    <li class="flex items-start">
                      <span class="text-green-500 mr-2">✓</span>
                      <span>You'll need to login to access the dashboard</span>
                    </li>
                  </ul>
                </div>

                {/* Setup Button */}
                <button
                  type="button"
                  onClick={handleSetup}
                  disabled={isSettingUp()}
                  class="w-full py-3 px-4 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-400 text-white rounded-lg font-medium transition-colors disabled:cursor-not-allowed"
                >
                  {isSettingUp() ? 'Setting up...' : 'Complete Setup'}
                </button>
              </div>
            </div>
          </Show>

          <Show when={showCredentials()}>
            <div class="p-8">
              <div class="text-center mb-6">
                <div class="w-16 h-16 bg-green-100 dark:bg-green-900/50 rounded-full flex items-center justify-center mx-auto mb-4">
                  <svg
                    class="w-8 h-8 text-green-600 dark:text-green-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                </div>
                <SectionHeader
                  title="Setup complete!"
                  size="lg"
                  class="mb-2"
                  align="center"
                  titleClass="text-gray-800 dark:text-gray-100"
                />
                <p class="text-gray-600 dark:text-gray-400">
                  Save your credentials now - they won't be shown again
                </p>
              </div>

              <div class="space-y-4">
                {/* Username */}
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                  <label class="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                    Username
                  </label>
                  <div class="font-mono text-lg text-gray-900 dark:text-gray-100">
                    {savedUsername()}
                  </div>
                </div>

                {/* Password */}
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                  <label class="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                    Password
                  </label>
                  <div class="flex items-center justify-between">
                    <code class="font-mono text-lg text-gray-900 dark:text-gray-100 break-all">
                      {savedPassword()}
                    </code>
                    <button
                      type="button"
                      onClick={() => handleCopy('password')}
                      class="ml-2 p-2 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
                      title="Copy password"
                    >
                      {copied() === 'password' ? (
                        <svg
                          class="w-5 h-5 text-green-600"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M5 13l4 4L19 7"
                          />
                        </svg>
                      ) : (
                        <svg
                          class="w-5 h-5 text-gray-600 dark:text-gray-400"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                          />
                        </svg>
                      )}
                    </button>
                  </div>
                </div>

                {/* API Token */}
                <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
                  <label class="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                    API Token (for automation)
                  </label>
                  <div class="flex items-center justify-between">
                    <code class="font-mono text-sm text-gray-900 dark:text-gray-100 break-all">
                      {savedToken()}
                    </code>
                    <button
                      type="button"
                      onClick={() => handleCopy('token')}
                      class="ml-2 p-2 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
                      title="Copy token"
                    >
                      {copied() === 'token' ? (
                        <svg
                          class="w-5 h-5 text-green-600"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M5 13l4 4L19 7"
                          />
                        </svg>
                      ) : (
                        <svg
                          class="w-5 h-5 text-gray-600 dark:text-gray-400"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                          />
                        </svg>
                      )}
                    </button>
                  </div>
                </div>

                {/* Warning */}
                <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-4">
                  <p class="text-sm font-semibold text-amber-800 dark:text-amber-200 mb-1">
                    ⚠️ Important
                  </p>
                  <p class="text-xs text-amber-700 dark:text-amber-300">
                    These credentials will never be shown again. Save them in a password manager
                    now!
                  </p>
                </div>

                {/* Action Buttons */}
                <div class="flex gap-3">
                  <button
                    type="button"
                    onClick={downloadCredentials}
                    class="flex-1 py-3 px-4 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-lg font-medium transition-colors flex items-center justify-center gap-2"
                  >
                    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                      />
                    </svg>
                    Download Credentials
                  </button>
                  <button
                    type="button"
                    onClick={() => window.location.reload()}
                    class="flex-1 py-3 px-4 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors"
                  >
                    Continue to Login
                  </button>
                </div>
              </div>
            </div>
          </Show>
        </div>
      </div>
    </div>
  );
};
