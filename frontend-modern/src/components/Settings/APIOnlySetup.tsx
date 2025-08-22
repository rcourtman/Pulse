import { Component, createSignal, Show } from 'solid-js';
import { showSuccess, showError } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';

interface APIOnlySetupProps {
  onTokenGenerated?: () => void;
}

export const APIOnlySetup: Component<APIOnlySetupProps> = (props) => {
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [token, setToken] = createSignal<string | null>(null);
  const [showToken, setShowToken] = createSignal(false);
  const [copied, setCopied] = createSignal(false);
  
  const generateToken = async () => {
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
      setToken(data.token);
      setShowToken(true);
      showSuccess('API token generated! Save it now - it won\'t be shown again.');
    } catch (error) {
      showError(`Failed to generate token: ${error}`);
    } finally {
      setIsGenerating(false);
    }
  };
  
  const handleCopy = async () => {
    if (!token()) return;
    
    const success = await copyToClipboard(token()!);
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
        <div class="space-y-4">
          <p class="text-sm text-gray-700 dark:text-gray-300">
            Generate an API token for programmatic access. Use it for:
          </p>
          <ul class="list-disc list-inside text-xs text-gray-600 dark:text-gray-400 space-y-1">
            <li>Automation scripts and CI/CD pipelines</li>
            <li>Monitoring integrations</li>
            <li>Third-party applications</li>
          </ul>
          
          <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-3">
            <p class="text-xs text-amber-700 dark:text-amber-300">
              <strong>Note:</strong> Without password authentication enabled, 
              the UI will remain publicly accessible.
            </p>
          </div>
          
          <button type="button"
            onClick={generateToken}
            disabled={isGenerating()}
            class="px-4 py-2 bg-green-600 text-white text-sm rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {isGenerating() ? 'Generating...' : 'Generate API Token'}
          </button>
        </div>
      </Show>
      
      <Show when={showToken() && token()}>
        <div class="space-y-4">
          <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4">
            <h4 class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">
              ✅ API Token Generated!
            </h4>
            <p class="text-xs text-green-700 dark:text-green-300">
              Save this token now - it will never be shown again!
            </p>
          </div>
          
          <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
            <label class="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">Your API Token</label>
            <div class="flex items-center space-x-2">
              <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                {token()}
              </code>
              <button type="button"
                onClick={handleCopy}
                class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
              >
                {copied() ? 'Copied!' : 'Copy'}
              </button>
            </div>
          </div>
          
          
          <button type="button"
            onClick={() => {
              setShowToken(false);
              setToken(null);
              if (props.onTokenGenerated) {
                props.onTokenGenerated();
              }
            }}
            class="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 transition-colors"
          >
            Done - I've Saved My Token
          </button>
        </div>
      </Show>
    </div>
  );
};