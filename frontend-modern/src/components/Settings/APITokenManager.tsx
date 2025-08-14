import { createSignal, Show, onMount } from 'solid-js';
import { SystemAPI, APITokenStatus } from '@/api/system';
import { copyToClipboard } from '@/utils/clipboard';

export function APITokenManager() {
  const [tokenStatus, setTokenStatus] = createSignal<APITokenStatus | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [showToken, setShowToken] = createSignal(false);
  const [currentToken, setCurrentToken] = createSignal<string | null>(null);
  const [error, setError] = createSignal<string | null>(null);
  const [copied, setCopied] = createSignal(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = createSignal(false);

  // Load initial status and fetch the actual token if it exists
  onMount(async () => {
    try {
      const status = await SystemAPI.getAPITokenStatus();
      setTokenStatus(status);
      
      // If there's a token, fetch it immediately
      if (status.hasToken) {
        try {
          const tokenData = await SystemAPI.getAPIToken(true);
          if (tokenData.token) {
            setCurrentToken(tokenData.token);
            setShowToken(true);
          }
        } catch (err) {
          console.error('Failed to fetch existing API token:', err);
        }
      }
    } catch (err) {
      console.error('Failed to load API token status:', err);
    }
  });

  const generateToken = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await SystemAPI.generateAPIToken();
      setTokenStatus(result);
      setCurrentToken(result.token || null);
      setShowToken(true);
    } catch (err: any) {
      setError(err.message || 'Failed to generate token');
    } finally {
      setLoading(false);
    }
  };

  const deleteToken = async () => {
    setLoading(true);
    setError(null);
    try {
      await SystemAPI.deleteAPIToken();
      setTokenStatus({ hasToken: false });
      setCurrentToken(null);
      setShowToken(false);
      setShowDeleteConfirm(false);
    } catch (err: any) {
      setError(err.message || 'Failed to delete token');
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!currentToken()) return;
    
    const success = await copyToClipboard(currentToken()!);
    if (success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } else {
      setError('Failed to copy - please select and copy manually');
    }
  };

  return (
    <div>
      
      <Show when={error()}>
        <div class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
          <p class="text-sm text-red-700 dark:text-red-300">{error()}</p>
        </div>
      </Show>

      <Show
        when={tokenStatus()?.hasToken}
        fallback={
          <div class="space-y-4">
            <p class="text-sm text-gray-600 dark:text-gray-400">
              No token configured
            </p>
            
            <button
              onClick={generateToken}
              disabled={loading()}
              class="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading() ? 'Generating...' : 'Generate Token'}
            </button>
          </div>
        }
      >
        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <p class="text-sm text-gray-600 dark:text-gray-400">
              Token active
            </p>
            <button
              onClick={() => setShowDeleteConfirm(true)}
              class="text-sm text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
            >
              Delete
            </button>
          </div>

          <Show when={currentToken() && showToken()}>
            <div class="p-4 bg-gray-50 dark:bg-gray-900/50 border border-gray-200 dark:border-gray-700 rounded-lg">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                API Token
              </p>
              <div class="relative">
                <input
                  type="text"
                  value={currentToken()!}
                  readonly
                  class="w-full px-3 py-2 pr-20 text-xs font-mono bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md"
                  onClick={(e) => e.currentTarget.select()}
                />
                <button
                  onClick={handleCopy}
                  class="absolute right-2 top-1/2 -translate-y-1/2 px-3 py-1 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                >
                  {copied() ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <div class="text-xs text-gray-600 dark:text-gray-400 mt-2 space-y-2">
                <p>Use this token for API authentication:</p>
                <code class="block bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
                  curl -H "X-API-Token: {currentToken()}" {window.location.origin}/api/health
                </code>
              </div>
            </div>
          </Show>

          <div class="flex gap-2">
            <button
              onClick={generateToken}
              disabled={loading()}
              class="px-4 py-2 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading() ? 'Generating...' : 'Regenerate Token'}
            </button>
            
            <button
              onClick={() => setShowDeleteConfirm(true)}
              disabled={loading()}
              class="px-4 py-2 bg-red-600 text-white text-sm rounded-md hover:bg-red-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Remove Token
            </button>
          </div>

        </div>
      </Show>

      {/* Delete Confirmation Modal */}
      <Show when={showDeleteConfirm()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full">
            <h3 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-4">
              Remove API Token?
            </h3>
            <p class="text-sm text-gray-600 dark:text-gray-400 mb-6">
              This will remove API authentication from your Pulse instance. All configuration endpoints 
              will be accessible without credentials.
            </p>
            <div class="flex justify-end gap-2">
              <button
                onClick={() => setShowDeleteConfirm(false)}
                class="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700"
              >
                Cancel
              </button>
              <button
                onClick={deleteToken}
                disabled={loading()}
                class="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {loading() ? 'Removing...' : 'Remove Token'}
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}