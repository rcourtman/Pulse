import { Component, createSignal, Show, createEffect } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';
import { apiFetch } from '@/utils/apiClient';

interface GenerateAPITokenProps {
  currentTokenHint?: string;
}

export const GenerateAPIToken: Component<GenerateAPITokenProps> = (props) => {
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [newToken, setNewToken] = createSignal<string | null>(null);
  const [showToken, setShowToken] = createSignal(false);
  const [copied, setCopied] = createSignal(false);
  const [currentHint, setCurrentHint] = createSignal(props.currentTokenHint || '');
  const [showConfirm, setShowConfirm] = createSignal(false);
  
  // Update hint when props change
  createEffect(() => {
    if (props.currentTokenHint) {
      setCurrentHint(props.currentTokenHint);
    }
  });
  
  const generateNewToken = async () => {
    setIsGenerating(true);
    setShowConfirm(false);
    
    try {
      const response = await apiFetch('/api/security/regenerate-token', {
        method: 'POST'
      });
      
      if (!response.ok) {
        const error = await response.text();
        throw new Error(error || 'Failed to generate token');
      }
      
      const data = await response.json();
      setNewToken(data.token);
      // Update the current hint with the new token
      if (data.token && data.token.length >= 20) {
        setCurrentHint(data.token.slice(0, 8) + '...' + data.token.slice(-4));
      }
      setShowToken(true);
      showSuccess('New API token generated! Save it now - it won\'t be shown again.');
    } catch (error) {
      showError(`Failed to generate token: ${error}`);
    } finally {
      setIsGenerating(false);
    }
  };
  
  const handleCopy = async () => {
    if (!newToken()) return;
    
    const success = await copyToClipboard(newToken()!);
    if (success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } else {
      showError('Failed to copy to clipboard');
    }
  };
  
  return (
    <div class="space-y-4">
      <Show when={!showToken()}>
        <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">
            API Token Active
          </h4>
          <Show when={currentHint() && currentHint().length > 0}>
            <div class="mb-3 px-3 py-2 bg-gray-800 dark:bg-gray-950 rounded">
              <code class="text-xs text-gray-300 font-mono">
                Current token: {currentHint()}
              </code>
            </div>
          </Show>
          <p class="text-xs text-gray-600 dark:text-gray-400 mb-4">
            An API token is configured for this instance. Use it with the X-API-Token header for automation.
          </p>
          
          <button type="button"
            onClick={() => setShowConfirm(true)}
            disabled={isGenerating()}
            class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isGenerating() ? 'Generating...' : 'Generate New Token'}
          </button>
        </div>
      </Show>
      
      <Show when={showToken() && newToken()}>
        <div class="space-y-4">
          <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4">
            <h4 class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
              ✅ New API Token Generated!
            </h4>
            <p class="text-xs text-green-700 dark:text-green-300">
              Save this token now - it will never be shown again!
            </p>
          </div>
          
          <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
            <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Your New API Token</label>
            <div class="flex items-center space-x-2">
              <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                {newToken()}
              </code>
              <button type="button"
                onClick={handleCopy}
                class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
              >
                {copied() ? 'Copied!' : 'Copy'}
              </button>
            </div>
          </div>
          
          <div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3">
            <div class="flex items-start space-x-2">
              <svg class="w-4 h-4 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <div class="text-xs text-blue-700 dark:text-blue-300">
                <p class="font-semibold">Token Active Immediately!</p>
                <p class="mt-1">Your new API token is active and ready to use.</p>
                <p class="mt-1 text-blue-600 dark:text-blue-400">The old token (if any) has been invalidated.</p>
              </div>
            </div>
          </div>
          
          <button type="button"
            onClick={() => {
              setShowToken(false);
              setNewToken(null);
            }}
            class="px-4 py-2 bg-gray-600 text-white text-sm rounded-lg hover:bg-gray-700 transition-colors"
          >
            Done
          </button>
        </div>
      </Show>
      
      <Show when={showConfirm()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl p-6 max-w-md w-full mx-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">
              Generate New API Token?
            </h3>
            <p class="text-sm text-gray-600 dark:text-gray-400 mb-6">
              This will generate a new API token and <span class="font-semibold text-red-600 dark:text-red-400">immediately invalidate the current token</span>. 
              Any scripts or integrations using the old token will stop working.
            </p>
            <div class="flex gap-3 justify-end">
              <button type="button"
                onClick={() => setShowConfirm(false)}
                class="px-4 py-2 text-sm text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
              >
                Cancel
              </button>
              <button type="button"
                onClick={generateNewToken}
                class="px-4 py-2 text-sm text-white bg-red-600 rounded-lg hover:bg-red-700 transition-colors"
              >
                Generate New Token
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};