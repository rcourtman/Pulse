import { Component, createSignal, Show } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';

export const GenerateAPIToken: Component = () => {
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [newToken, setNewToken] = createSignal<string | null>(null);
  const [showToken, setShowToken] = createSignal(false);
  const [copied, setCopied] = createSignal(false);
  const [deploymentType, setDeploymentType] = createSignal<string>('');
  
  const generateNewToken = async () => {
    if (!confirm('Generate a new API token? The old token will stop working immediately.')) {
      return;
    }
    
    setIsGenerating(true);
    
    try {
      const response = await fetch('/api/security/regenerate-token', {
        method: 'POST',
        credentials: 'include'
      });
      
      if (!response.ok) {
        const error = await response.text();
        throw new Error(error || 'Failed to generate token');
      }
      
      const data = await response.json();
      setNewToken(data.token);
      setDeploymentType(data.deploymentType);
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
  
  const getRestartInstructions = () => {
    switch(deploymentType()) {
      case 'docker':
        return 'Restart your Docker container to activate the new token.';
      case 'proxmoxve':
        return 'Restart Pulse from the ProxmoxVE host to activate the new token.';
      case 'systemd':
        return 'Run: sudo systemctl restart pulse';
      default:
        return 'Restart the Pulse service to activate the new token.';
    }
  };
  
  return (
    <div class="space-y-4">
      <Show when={!showToken()}>
        <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">
            API Token Active
          </h4>
          <p class="text-xs text-gray-600 dark:text-gray-400 mb-4">
            An API token is configured for this instance. Use it with the X-API-Token header for automation.
          </p>
          
          <button
            onClick={generateNewToken}
            disabled={isGenerating()}
            class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isGenerating() ? 'Generating...' : 'Generate New Token'}
          </button>
        </div>
        
        <div class="text-xs text-gray-500 dark:text-gray-400">
          <p class="font-medium mb-1">Using the API Token:</p>
          <code class="block bg-gray-900 dark:bg-gray-950 text-gray-100 p-2 rounded text-xs">
            curl -H "X-API-Token: YOUR_TOKEN" http://pulse:7655/api/...
          </code>
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
              <button
                onClick={handleCopy}
                class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
              >
                {copied() ? 'Copied!' : 'Copy'}
              </button>
            </div>
          </div>
          
          <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
            <div class="flex items-start space-x-2">
              <svg class="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <div class="text-xs text-amber-700 dark:text-amber-300">
                <p class="font-semibold">Restart Required</p>
                <p class="mt-1">{getRestartInstructions()}</p>
                <p class="mt-1 text-amber-600 dark:text-amber-400">The old token has been invalidated and will no longer work.</p>
              </div>
            </div>
          </div>
          
          <button
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
    </div>
  );
};