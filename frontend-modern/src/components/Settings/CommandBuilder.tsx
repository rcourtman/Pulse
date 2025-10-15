import { Component, createSignal, Show, createMemo, createEffect } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { showTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';

interface CommandBuilderProps {
  command: string;
  placeholder: string;
  storedToken?: string | null;
  currentTokenHint?: string; // Masked token preview (e.g., "abc12***...xyz89")
  onTokenChange?: (token: string) => void;
  onTokenGenerated?: (token: string, record: APITokenRecord) => void;
  requiresToken: boolean;
  hasExistingToken?: boolean;
  canManageTokens?: boolean;
}

export const CommandBuilder: Component<CommandBuilderProps> = (props) => {
  const [tokenInput, setTokenInput] = createSignal('');
  const [showToken, setShowToken] = createSignal(false);
  const [isValidating, setIsValidating] = createSignal(false);
  const [validationResult, setValidationResult] = createSignal<'valid' | 'invalid' | null>(null);
  const [copied, setCopied] = createSignal(false);

  // Token generation/revocation state
  const [showGenerateModal, setShowGenerateModal] = createSignal(false);
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [tokenLabel, setTokenLabel] = createSignal('Docker agent token');
  const [latestGeneratedToken, setLatestGeneratedToken] = createSignal<string | null>(null);
  const [latestGeneratedRecord, setLatestGeneratedRecord] = createSignal<APITokenRecord | null>(null);
  const tokenRevealState = useTokenRevealState();

  const defaultTokenLabel = () => `Docker agent token ${new Date().toISOString().slice(0, 10)}`;
  const canManageTokens = () => props.canManageTokens !== false;
  const openGenerateModal = () => {
    if (!canManageTokens()) return;
    setTokenLabel(defaultTokenLabel());
    setShowGenerateModal(true);
  };

  const isRevealActiveForLatestToken = () => {
    const token = latestGeneratedToken();
    const active = tokenRevealState();
    if (!token || !active) return false;
    return active.token === token;
  };

  const reopenLatestTokenDialog = () => {
    const token = latestGeneratedToken();
    const record = latestGeneratedRecord();
    if (!token || !record) return;
    showTokenReveal({
      token,
      record,
      source: 'docker-command',
      note: 'Copy this token now. Close the dialog once you have stored it securely.',
    });
  };

  // Initialize with stored token if available
  createEffect(() => {
    if (props.storedToken && tokenInput() === '') {
      setTokenInput(props.storedToken);
      // Don't auto-validate stored tokens to avoid unnecessary API calls
    }
  });

  // Computed command with token substitution
  const finalCommand = createMemo(() => {
    const token = tokenInput().trim();
    if (!props.requiresToken) {
      return props.command;
    }
    if (token) {
      return props.command.replace(props.placeholder, token);
    }
    return props.command;
  });

  // Check if placeholder is still present
  const hasPlaceholder = createMemo(() => {
    return props.requiresToken && finalCommand().includes(props.placeholder);
  });

  // Determine state (warning, success, or neutral)
  const commandState = createMemo<'warning' | 'success' | 'neutral'>(() => {
    if (!props.requiresToken) return 'success'; // No token needed, always ready
    if (hasPlaceholder()) return 'warning'; // Placeholder still present
    if (validationResult() === 'valid') return 'success'; // Token validated
    if (tokenInput().trim().length > 0) return 'neutral'; // Token entered but not validated
    return 'warning'; // No token entered
  });

  // Validate token via API
  const validateToken = async () => {
    const token = tokenInput().trim();
    if (!token || !props.requiresToken) return;

    setIsValidating(true);
    setValidationResult(null);

    try {
      const response = await apiFetch('/api/security/validate-token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
      });

      // Handle different HTTP status codes appropriately
      if (!response.ok) {
        if (response.status === 401) {
          window.showToast('error', 'Authentication required - please log in');
          return;
        }
        if (response.status === 403) {
          window.showToast('error', 'Permission denied - admin access required');
          return;
        }
        if (response.status === 429) {
          window.showToast('error', 'Rate limit exceeded - please try again later');
          return;
        }
        // Other server errors
        window.showToast('error', `Server error (${response.status}) - please try again`);
        return;
      }

      const data = await response.json();
      setValidationResult(data.valid ? 'valid' : 'invalid');

      if (data.valid) {
        window.showToast('success', 'Token is valid ✓');
      } else {
        window.showToast('error', 'Token is invalid');
      }
    } catch (error) {
      // Network or parsing error
      console.error('Token validation failed:', error);
      window.showToast('error', 'Network error - failed to validate token');
    } finally {
      setIsValidating(false);
    }
  };

  // Copy command to clipboard
  const copyCommand = async () => {
    // Warn if placeholder is still present
    if (hasPlaceholder()) {
      window.showToast(
        'warning',
        `Please replace ${props.placeholder} with your API token before running the command`,
      );
      return;
    }

    const command = finalCommand();
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(command);
      } else {
        // Fallback
        const textarea = document.createElement('textarea');
        textarea.value = command;
        textarea.style.position = 'fixed';
        textarea.style.left = '-999999px';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }

      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      window.showToast('success', 'Command copied to clipboard');
    } catch (error) {
      console.error('Failed to copy:', error);
      window.showToast('error', 'Failed to copy to clipboard');
    }
  };

  // Handle token input change
  const handleTokenChange = (value: string) => {
    setTokenInput(value);
    setValidationResult(null); // Clear validation when token changes
    if (props.onTokenChange) {
      props.onTokenChange(value);
    }
  };

  // Generate new token
  const generateNewToken = async () => {
    if (!canManageTokens()) return;
    if (isGenerating()) return;
    setIsGenerating(true);

    try {
      const desiredName = tokenLabel().trim() || undefined;
      const { token: newToken, record } = await SecurityAPI.createToken(desiredName);

      setShowGenerateModal(false);
      setLatestGeneratedToken(newToken);
      setLatestGeneratedRecord(record);

      reopenLatestTokenDialog();
      // Auto-populate the command builder
     setTokenInput(newToken);
     if (props.onTokenGenerated) {
       props.onTokenGenerated(newToken, record);
     }

      if (typeof window !== 'undefined') {
        try {
          window.localStorage.setItem('apiToken', newToken);
          window.dispatchEvent(new StorageEvent('storage', { key: 'apiToken', newValue: newToken }));
        } catch (storageErr) {
          console.warn('Unable to persist API token in localStorage', storageErr);
        }
      }

      window.showToast('success', 'New API token generated. Copy it from the dialog while it is visible.');
    } catch (error) {
      console.error('Token generation failed:', error);
      window.showToast('error', error instanceof Error ? error.message : 'Failed to generate token');
    } finally {
      setIsGenerating(false);
    }
  };

  // Use existing token
  const useExistingToken = () => {
    if (props.storedToken) {
      setTokenInput(props.storedToken);
      window.showToast('success', 'Using stored token');
    }
  };

  // Copy existing token to clipboard
  const copyExistingToken = async () => {
    if (!props.storedToken) return;

    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(props.storedToken);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = props.storedToken;
        textarea.style.position = 'fixed';
        textarea.style.left = '-999999px';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }

      window.showToast('success', 'Token copied to clipboard');
    } catch (error) {
      console.error('Failed to copy token:', error);
      window.showToast('error', 'Failed to copy token');
    }
  };

  // Visual styling based on state
  const borderColor = createMemo(() => {
    switch (commandState()) {
      case 'warning':
        return 'border-yellow-500 dark:border-yellow-600';
      case 'success':
        return 'border-green-500 dark:border-green-600';
      default:
        return 'border-gray-300 dark:border-gray-600';
    }
  });

  const bgColor = createMemo(() => {
    switch (commandState()) {
      case 'warning':
        return 'bg-yellow-50 dark:bg-yellow-900/10';
      case 'success':
        return 'bg-green-50 dark:bg-green-900/10';
      default:
        return 'bg-gray-50 dark:bg-gray-900';
    }
  });

  const indicatorIcon = createMemo(() => {
    switch (commandState()) {
      case 'warning':
        return (
          <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        );
      case 'success':
        return (
          <svg class="w-5 h-5 text-green-600 dark:text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        );
      default:
        return null;
    }
  });

  return (
    <div class="space-y-3">
      {/* Token Status Section */}
      <Show when={props.requiresToken}>
        <Show
          when={props.hasExistingToken}
          fallback={
            /* No token exists - offer to generate */
            <div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-3">
              <div class="flex items-start justify-between gap-3">
                <div class="flex-1">
                  <div class="flex items-center gap-2 mb-1">
                    <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                    <span class="text-sm font-semibold text-yellow-800 dark:text-yellow-300">No API token configured</span>
                  </div>
                  <p class="text-xs text-yellow-700 dark:text-yellow-400">Generate a token to secure your Docker agents.</p>
                </div>
                <Show when={canManageTokens()}>
                  <button
                    type="button"
                    onClick={openGenerateModal}
                    disabled={isGenerating()}
                    class="px-3 py-1.5 text-xs font-medium text-white bg-blue-600 rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors whitespace-nowrap"
                  >
                    {isGenerating() ? 'Generating...' : 'Generate API Token'}
                  </button>
                </Show>
              </div>
              <Show when={!canManageTokens()}>
                <p class="mt-3 text-xs text-yellow-700 dark:text-yellow-400">
                  Sign in with an administrator account to create tokens from the browser.
                </p>
              </Show>
            </div>
          }
        >
          {/* Token exists - show status and actions */}
          <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-3">
            <div class="flex items-start justify-between gap-3">
              <div class="flex-1">
                <div class="flex items-center gap-2 mb-1">
                  <svg class="w-5 h-5 text-green-600 dark:text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <span class="text-sm font-semibold text-green-800 dark:text-green-300">API token configured</span>
                </div>
                <Show when={props.currentTokenHint}>
                  <code class="text-xs font-mono text-green-700 dark:text-green-400 bg-green-100 dark:bg-green-900/40 px-2 py-0.5 rounded">
                    {props.currentTokenHint}
                  </code>
                </Show>
              </div>
              <div class="flex gap-2">
                <button
                  type="button"
                  onClick={useExistingToken}
                  disabled={!props.storedToken}
                  class="px-3 py-1.5 text-xs font-medium text-green-700 dark:text-green-300 bg-green-100 dark:bg-green-900/40 rounded hover:bg-green-200 dark:hover:bg-green-900/60 disabled:opacity-50 disabled:cursor-not-allowed transition-colors whitespace-nowrap"
                  title={props.storedToken ? "Fill the command with your existing token" : "Token not saved in browser - use the token input below instead"}
                >
                  Use This Token
                </button>
                <button
                  type="button"
                  onClick={copyExistingToken}
                  disabled={!props.storedToken}
                  class="px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 rounded hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                  title={props.storedToken ? "Copy token to clipboard" : "Token not saved in browser - generate a new one if needed"}
                >
                  Copy
                </button>
                <Show when={canManageTokens()}>
                  <button
                    type="button"
                    onClick={openGenerateModal}
                    disabled={isGenerating()}
                    class="px-3 py-1.5 text-xs font-medium text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/40 rounded hover:bg-blue-200 dark:hover:bg-blue-900/60 disabled:opacity-50 disabled:cursor-not-allowed transition-colors whitespace-nowrap"
                    title="Generate another token for a new host or automation workflow"
                  >
                    Generate Token
                  </button>
                </Show>
              </div>
            </div>
            <p class="mt-2 text-xs text-gray-600 dark:text-gray-400">
              Manage or revoke tokens from the table above whenever a credential is no longer needed.
            </p>
          </div>
        </Show>
      </Show>

      {/* Security explanation */}
      <Show when={props.requiresToken}>
        <div class="flex items-start gap-2 text-xs text-gray-600 dark:text-gray-400 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded p-2">
          <svg class="w-4 h-4 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <div>
            <span class="font-medium text-blue-800 dark:text-blue-300">Security Note:</span>{' '}
            <span class="text-blue-700 dark:text-blue-400">
              Tokens are not auto-inserted to prevent accidental exposure. Paste your token below to build the command.
            </span>
          </div>
        </div>
      </Show>

      {/* Token input field */}
      <Show when={props.requiresToken}>
        <div class="space-y-2">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
            API Token
            <Show when={props.storedToken}>
              <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
                (prefilled from saved token)
              </span>
            </Show>
          </label>
          <div class="flex gap-2">
            <div class="flex-1 relative">
              <input
                type={showToken() ? 'text' : 'password'}
                value={tokenInput()}
                onInput={(e) => handleTokenChange(e.currentTarget.value)}
                placeholder="Paste your API token here"
                class="w-full px-3 py-2 pr-10 text-sm font-mono border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              />
              <button
                type="button"
                onClick={() => setShowToken(!showToken())}
                class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                title={showToken() ? 'Hide token' : 'Show token'}
              >
                <Show
                  when={showToken()}
                  fallback={
                    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                    </svg>
                  }
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                  </svg>
                </Show>
              </button>
            </div>
            <button
              type="button"
              onClick={validateToken}
              disabled={isValidating() || !tokenInput().trim()}
              class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="Test if this token is valid"
            >
              {isValidating() ? 'Testing...' : 'Test Token'}
            </button>
          </div>
          {/* Validation result */}
          <Show when={validationResult()}>
            <div class={`text-xs ${validationResult() === 'valid' ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}`}>
              {validationResult() === 'valid' ? '✓ Token is valid' : '✗ Token is invalid'}
            </div>
          </Show>
        </div>
      </Show>

      {/* Command preview */}
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">Command Preview</span>
            {indicatorIcon()}
          </div>
          <button
            type="button"
            onClick={copyCommand}
            disabled={hasPlaceholder()}
            class={`px-3 py-1 text-xs font-medium rounded transition-colors ${
              hasPlaceholder()
                ? 'bg-gray-300 dark:bg-gray-700 text-gray-500 dark:text-gray-400 cursor-not-allowed'
                : 'bg-blue-600 text-white hover:bg-blue-700'
            }`}
            title={hasPlaceholder() ? `Replace ${props.placeholder} with your token first` : 'Copy to clipboard'}
          >
            {copied() ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <div class={`relative rounded border-2 ${borderColor()} ${bgColor()} p-3 overflow-x-auto transition-colors`}>
          <code class="text-sm text-gray-900 dark:text-gray-100 font-mono break-all">
            {finalCommand()}
          </code>
        </div>
      </div>

      {/* Status message */}
      <Show when={hasPlaceholder()}>
        <div class="text-xs text-yellow-700 dark:text-yellow-400 flex items-center gap-2">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span>Paste your API token above to enable the copy button</span>
        </div>
      </Show>
      <Show when={!hasPlaceholder() && commandState() === 'success'}>
        <div class="text-xs text-green-700 dark:text-green-400 flex items-center gap-2">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>Ready to copy! Your command is complete.</span>
        </div>
      </Show>

      {/* Generate Token Confirmation Modal */}
      <Show when={showGenerateModal()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full p-6">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-3">Generate API Token</h3>
            <div class="space-y-3 mb-6">
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Create a dedicated token for this host or automation workflow. Tokens remain active until you revoke them from the API tokens list.
              </p>
              <div class="space-y-2">
                <label class="text-xs font-medium text-gray-600 dark:text-gray-400" for="command-builder-token-name">
                  Token name (optional)
                </label>
                <input
                  id="command-builder-token-name"
                  type="text"
                  value={tokenLabel()}
                  onInput={(event) => setTokenLabel(event.currentTarget.value)}
                  class="w-full rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder={defaultTokenLabel()}
                />
              </div>
              <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded p-3">
                <p class="text-xs text-blue-800 dark:text-blue-300 font-medium">
                  Tip: Issue one token per host so you can revoke compromised credentials without affecting other agents.
                </p>
              </div>
            </div>
            <div class="flex gap-3 justify-end">
              <button
                type="button"
                onClick={() => setShowGenerateModal(false)}
                class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={generateNewToken}
                disabled={isGenerating()}
                class="px-4 py-2 text-sm text-white bg-blue-600 rounded hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isGenerating() ? 'Generating…' : 'Generate Token'}
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* New Token Display Modal */}
      <Show
        when={latestGeneratedToken() && !isRevealActiveForLatestToken()}
      >
        <div class="mt-4 space-y-3 rounded-lg border border-blue-200 dark:border-blue-700 bg-blue-50 dark:bg-blue-900/20 p-4">
          <div class="flex items-start gap-2">
            <div class="flex-shrink-0 text-blue-600 dark:text-blue-300">
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
            </div>
            <div class="space-y-1">
              <p class="text-sm font-semibold text-blue-800 dark:text-blue-200">
                New token ready
              </p>
              <p class="text-xs text-blue-700 dark:text-blue-300 leading-snug">
                Reopen the secure dialog if you still need to copy the value before you leave this page. The command above already includes it.
              </p>
              <Show when={latestGeneratedRecord()}>
                <div class="text-xs text-blue-700/80 dark:text-blue-300/90">
                  Label{' '}
                  <span class="font-semibold">
                    {latestGeneratedRecord()?.name || 'Untitled token'}
                  </span>
                  <Show when={latestGeneratedRecord()?.prefix || latestGeneratedRecord()?.suffix}>
                    {' '}· Hint{' '}
                    <code class="rounded bg-blue-100 dark:bg-blue-900/40 px-1.5 py-0.5 font-mono text-[11px] text-blue-700 dark:text-blue-200">
                      {latestGeneratedRecord()?.prefix}…{latestGeneratedRecord()?.suffix}
                    </code>
                  </Show>
                </div>
              </Show>
            </div>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={reopenLatestTokenDialog}
              class="inline-flex items-center gap-1 rounded-md bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold px-3 py-2 transition-colors"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
              Show token dialog
            </button>
            <button
              type="button"
              onClick={() => {
                setLatestGeneratedToken(null);
                setLatestGeneratedRecord(null);
              }}
              class="inline-flex items-center rounded-md border border-blue-300 dark:border-blue-700 px-3 py-2 text-xs font-medium text-blue-800 dark:text-blue-200 hover:bg-blue-100 dark:hover:bg-blue-900/40 transition-colors"
            >
              Dismiss reminder
            </button>
          </div>
        </div>
      </Show>
    </div>
  );
};
