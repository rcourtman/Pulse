import { Component, createSignal, Show } from 'solid-js';
import { showError } from '@/utils/toast';

export const CurrentAPIToken: Component = () => {
  const [lastGeneratedToken, setLastGeneratedToken] = createSignal<string | null>(null);
  const [showToken, setShowToken] = createSignal(false);
  const [copied, setCopied] = createSignal(false);
  
  // Store the last generated token in sessionStorage so it persists during the session
  const sessionKey = 'pulse_last_api_token';
  
  // Check if we have a token from this session
  const storedToken = sessionStorage.getItem(sessionKey);
  if (storedToken) {
    setLastGeneratedToken(storedToken);
  }

  const copyToClipboard = async () => {
    if (!lastGeneratedToken()) return;
    
    try {
      await navigator.clipboard.writeText(lastGeneratedToken()!);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      showError('Failed to copy to clipboard');
    }
  };

  return (
    <div class="space-y-4">
      <Show when={lastGeneratedToken()}>
        {/* If we have the token from this session, show it */}
        <div class="space-y-3">
          <Show when={!showToken()}>
            <div class="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-900 rounded-lg">
              <div>
                <p class="text-sm text-gray-700 dark:text-gray-300">
                  Your API token from this session
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  Click reveal to show your token
                </p>
              </div>
              <button
                onClick={() => setShowToken(true)}
                class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 transition-colors"
              >
                Reveal Token
              </button>
            </div>
          </Show>

          <Show when={showToken()}>
            <div class="space-y-3">
              <div class="flex items-center justify-between">
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  API Token
                </label>
                <button
                  onClick={() => setShowToken(false)}
                  class="text-xs text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                >
                  Hide
                </button>
              </div>
              
              <div class="flex items-center space-x-2">
                <code class="flex-1 font-mono text-sm bg-gray-50 dark:bg-gray-900 px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-700 break-all">
                  {lastGeneratedToken()}
                </code>
                <button
                  onClick={copyToClipboard}
                  class="px-3 py-2 text-sm bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors"
                >
                  {copied() ? '✓ Copied' : 'Copy'}
                </button>
              </div>
              
              <div class="text-xs text-gray-500 dark:text-gray-400 space-y-1">
                <p>Use this token with the X-API-Token header:</p>
                <code class="block bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded text-xs">
                  curl -H "X-API-Token: {lastGeneratedToken()}" http://your-pulse-url/api/health
                </code>
              </div>
            </div>
          </Show>
          
          <div class="p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg">
            <p class="text-xs text-amber-700 dark:text-amber-300">
              <strong>Note:</strong> This token is only available during your current session. 
              If you lose it after logging out, you'll need to reset your security settings.
            </p>
          </div>
        </div>
      </Show>

      <Show when={!lastGeneratedToken()}>
        {/* No token in session storage */}
        <div class="p-4 bg-gray-50 dark:bg-gray-900 rounded-lg space-y-3">
          <div>
            <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              API Token Configured
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              An API token was configured during security setup.
            </p>
          </div>
          
          <div class="p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg">
            <p class="text-xs text-amber-700 dark:text-amber-300">
              <strong>Important:</strong> For security reasons, the API token cannot be retrieved after initial setup. 
              It is stored as a one-way hash (SHA3-256) and the original value cannot be recovered.
            </p>
          </div>
          
          <div class="text-xs text-gray-600 dark:text-gray-400 space-y-2">
            <p><strong>Lost your token?</strong> You have two options:</p>
            <ol class="list-decimal list-inside space-y-1 ml-2">
              <li>If you saved it during setup, use that copy</li>
              <li>Reset security settings and create a new token</li>
            </ol>
          </div>
        </div>
      </Show>
    </div>
  );
};