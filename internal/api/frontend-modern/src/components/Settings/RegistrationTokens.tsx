import { Component, createSignal, onMount, For, Show } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';

interface Token {
  token: string;
  expires: string;
  maxUses: number;
  usedCount: number;
  description?: string;
}

interface GenerateTokenRequest {
  validityMinutes: number;
  maxUses: number;
  allowedTypes: string[];
  description: string;
}

const RegistrationTokens: Component = () => {
  const [tokens, setTokens] = createSignal<Token[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [showGenerator, setShowGenerator] = createSignal(false);
  const [newToken, setNewToken] = createSignal<Token | null>(null);
  const [error, setError] = createSignal<string | null>(null);
  const [copiedToken, setCopiedToken] = createSignal<string | null>(null);
  
  const [validityOption, setValidityOption] = createSignal('15');
  const [maxUses, setMaxUses] = createSignal('1');
  const [allowedTypesPVE, setAllowedTypesPVE] = createSignal(true);
  const [allowedTypesPBS, setAllowedTypesPBS] = createSignal(true);
  const [description, setDescription] = createSignal('');

  onMount(() => {
    fetchTokens();
  });

  const fetchTokens = async () => {
    setLoading(true);
    try {
      const apiToken = localStorage.getItem('apiToken');
      const headers: HeadersInit = {
        'Content-Type': 'application/json'
      };
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }

      const response = await fetch('/api/tokens/list', {
        headers
      });
      
      if (!response.ok) {
        throw new Error('Failed to fetch tokens');
      }
      
      const data = await response.json();
      setTokens(data || []);
    } catch (err) {
      console.error('Failed to load registration tokens:', err);
      // Keep setError here since we're not showing a toast
      setError('Failed to load registration tokens');
    } finally {
      setLoading(false);
    }
  };

  const generateToken = async () => {
    setError(null);
    try {
      const apiToken = localStorage.getItem('apiToken');
      const headers: HeadersInit = {
        'Content-Type': 'application/json'
      };
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }

      const validityMinutes = parseInt(validityOption());
      const allowedTypes: string[] = [];
      if (allowedTypesPVE()) allowedTypes.push('pve');
      if (allowedTypesPBS()) allowedTypes.push('pbs');
      
      const request: GenerateTokenRequest = {
        validityMinutes,
        maxUses: parseInt(maxUses()),
        allowedTypes,
        description: description()
      };

      const response = await fetch('/api/tokens/generate', {
        method: 'POST',
        headers,
        body: JSON.stringify(request)
      });
      
      if (!response.ok) {
        throw new Error('Failed to generate token');
      }
      
      const token = await response.json();
      setNewToken(token);
      setShowGenerator(false);
      fetchTokens();
      showSuccess('Registration token generated successfully');
      
      // Reset form
      setValidityOption('15');
      setMaxUses('1');
      setAllowedTypesPVE(true);
      setAllowedTypesPBS(true);
      setDescription('');
    } catch (err) {
      showError('Failed to generate token');
      console.error(err);
    }
  };

  const revokeToken = async (token: string) => {
    if (!confirm('Are you sure you want to revoke this token?')) {
      return;
    }
    
    try {
      const apiToken = localStorage.getItem('apiToken');
      const headers: HeadersInit = {
        'Content-Type': 'application/json'
      };
      if (apiToken) {
        headers['X-API-Token'] = apiToken;
      }

      const response = await fetch(`/api/tokens/revoke?token=${encodeURIComponent(token)}`, {
        method: 'DELETE',
        headers
      });
      
      if (!response.ok) {
        throw new Error('Failed to revoke token');
      }
      
      fetchTokens();
      showSuccess('Token revoked successfully');
    } catch (err) {
      showError('Failed to revoke token');
      console.error(err);
    }
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedToken(text);
      setTimeout(() => setCopiedToken(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  const formatExpiry = (expires: string) => {
    const expiryDate = new Date(expires);
    const now = new Date();
    const diff = expiryDate.getTime() - now.getTime();
    
    if (diff < 0) {
      return 'Expired';
    }
    
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    
    if (days > 0) {
      return `${days}d ${hours % 24}h`;
    } else if (hours > 0) {
      return `${hours}h ${minutes % 60}m`;
    } else {
      return `${minutes}m`;
    }
  };

  return (
    <div class="space-y-6">
      <div class="flex justify-between items-center">
        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">
          Registration Tokens
        </h3>
        <button
          onClick={() => setShowGenerator(true)}
          class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
        >
          <svg class="h-4 w-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Generate Token
        </button>
      </div>

      <Show when={error()}>
        <div class="rounded-md bg-red-50 dark:bg-red-900/20 p-4">
          <div class="flex">
            <svg class="h-5 w-5 text-red-400 dark:text-red-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <div class="ml-3">
              <p class="text-sm text-red-800 dark:text-red-300">{error()}</p>
            </div>
          </div>
        </div>
      </Show>

      <Show when={newToken()}>
        <div class="rounded-md bg-green-50 dark:bg-green-900/20 p-4 border border-green-200 dark:border-green-800">
          <div class="flex items-start">
            <svg class="h-5 w-5 text-green-400 dark:text-green-300 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
            </svg>
            <div class="ml-3 flex-1">
              <h4 class="text-sm font-medium text-green-800 dark:text-green-300">
                New Token Generated
              </h4>
              <div class="mt-2 space-y-2">
                <div class="flex items-center space-x-2">
                  <code class="flex-1 text-sm bg-white dark:bg-gray-800 px-2 py-1 rounded border border-green-300 dark:border-green-700">
                    {newToken()!.token}
                  </code>
                  <button
                    onClick={() => copyToClipboard(newToken()!.token)}
                    class="p-1 hover:bg-green-100 dark:hover:bg-green-800 rounded"
                  >
                    <svg class="h-4 w-4 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                    </svg>
                  </button>
                </div>
                <p class="text-xs text-green-700 dark:text-green-400">
                  Save this token securely. It won't be shown again.
                </p>
              </div>
            </div>
          </div>
        </div>
      </Show>

      <Show when={showGenerator()}>
        <div class="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 space-y-4">
          <h4 class="font-medium text-gray-900 dark:text-gray-100">Generate Registration Token</h4>
          
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Validity
              </label>
              <select
                value={validityOption()}
                onChange={(e) => setValidityOption(e.currentTarget.value)}
                class="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              >
                <option value="15">15 minutes</option>
                <option value="60">1 hour</option>
                <option value="1440">24 hours</option>
              </select>
            </div>
            
            <div>
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
                Max Uses
              </label>
              <input
                type="number"
                min="1"
                max="100"
                value={maxUses()}
                onInput={(e) => setMaxUses(e.currentTarget.value)}
                class="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
          </div>
          
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Allowed Node Types
            </label>
            <div class="mt-2 space-x-4">
              <label class="inline-flex items-center">
                <input
                  type="checkbox"
                  checked={allowedTypesPVE()}
                  onChange={(e) => setAllowedTypesPVE(e.currentTarget.checked)}
                  class="rounded border-gray-300 text-blue-600 shadow-sm focus:border-blue-500 focus:ring-blue-500"
                />
                <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Proxmox VE</span>
              </label>
              <label class="inline-flex items-center">
                <input
                  type="checkbox"
                  checked={allowedTypesPBS()}
                  onChange={(e) => setAllowedTypesPBS(e.currentTarget.checked)}
                  class="rounded border-gray-300 text-blue-600 shadow-sm focus:border-blue-500 focus:ring-blue-500"
                />
                <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">PBS</span>
              </label>
            </div>
          </div>
          
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              Description (optional)
            </label>
            <input
              type="text"
              value={description()}
              onInput={(e) => setDescription(e.currentTarget.value)}
              placeholder="e.g., Production cluster setup"
              class="mt-1 block w-full rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>
          
          <div class="flex justify-end space-x-2">
            <button
              onClick={() => setShowGenerator(false)}
              class="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
            >
              Cancel
            </button>
            <button
              onClick={generateToken}
              disabled={!allowedTypesPVE() && !allowedTypesPBS()}
              class="px-3 py-2 border border-transparent rounded-md text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Generate
            </button>
          </div>
        </div>
      </Show>

      <Show
        when={!loading()}
        fallback={
          <div class="text-center py-4">
            <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
          </div>
        }
      >
        <Show
          when={tokens().length > 0}
          fallback={
            <div class="text-center py-8 text-gray-500 dark:text-gray-400">
              <svg class="h-12 w-12 mx-auto mb-2 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
              </svg>
              <p>No registration tokens</p>
              <p class="text-sm mt-1">Generate a token to allow secure node registration</p>
            </div>
          }
        >
          <div class="space-y-2">
            <For each={tokens()}>
              {(token) => (
                <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
                  <div class="flex items-start justify-between">
                    <div class="flex-1">
                      <div class="flex items-center space-x-2">
                        <code class="text-sm font-mono bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                          {token.token}
                        </code>
                        <button
                          onClick={() => copyToClipboard(token.token)}
                          class="p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                        >
                          <Show
                            when={copiedToken() === token.token}
                            fallback={
                              <svg class="h-3 w-3 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                              </svg>
                            }
                          >
                            <span class="text-xs text-green-600 dark:text-green-400">Copied!</span>
                          </Show>
                        </button>
                      </div>
                      <Show when={token.description}>
                        <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                          {token.description}
                        </p>
                      </Show>
                      <div class="flex items-center space-x-4 mt-2 text-xs text-gray-500 dark:text-gray-400">
                        <span>Expires: {formatExpiry(token.expires)}</span>
                        <span>Uses: {token.usedCount}/{token.maxUses === 0 ? '∞' : token.maxUses}</span>
                      </div>
                    </div>
                    <button
                      onClick={() => revokeToken(token.token)}
                      class="ml-4 p-1 hover:bg-red-100 dark:hover:bg-red-900/20 rounded text-red-600 dark:text-red-400"
                    >
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>
      </Show>
    </div>
  );
};

export default RegistrationTokens;