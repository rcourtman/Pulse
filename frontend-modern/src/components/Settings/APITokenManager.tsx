import { Component, For, Show, createSignal, onMount } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { showError, showSuccess } from '@/utils/toast';
import { copyToClipboard } from '@/utils/clipboard';
import { formatRelativeTime } from '@/utils/format';

interface APITokenManagerProps {
  currentTokenHint?: string;
  onTokensChanged?: () => void;
}

export const APITokenManager: Component<APITokenManagerProps> = (props) => {
  const [tokens, setTokens] = createSignal<APITokenRecord[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [newTokenValue, setNewTokenValue] = createSignal<string | null>(null);
  const [newTokenRecord, setNewTokenRecord] = createSignal<APITokenRecord | null>(null);
  const [copied, setCopied] = createSignal(false);
  const [nameInput, setNameInput] = createSignal('');

  const loadTokens = async () => {
    setLoading(true);
    try {
      const list = await SecurityAPI.listTokens();
      setTokens(list);
    } catch (err) {
      console.error('Failed to load API tokens', err);
      showError('Failed to load API tokens');
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    void loadTokens();
  });

  const handleGenerate = async () => {
    setIsGenerating(true);
    setCopied(false);
    try {
      const trimmedName = nameInput().trim() || undefined;
      const { token, record } = await SecurityAPI.createToken(trimmedName);
      setTokens((prev) => [record, ...prev]);
      setNewTokenValue(token);
      setNewTokenRecord(record);
      setNameInput('');
      showSuccess('New API token generated! Save it now – it will not be shown again.');
      props.onTokensChanged?.();

      try {
        window.localStorage.setItem('apiToken', token);
        // Fire a storage event so other listeners update immediately
        window.dispatchEvent(
          new StorageEvent('storage', { key: 'apiToken', newValue: token }),
        );
      } catch (storageErr) {
        console.warn('Unable to persist API token in localStorage', storageErr);
      }
    } catch (err) {
      console.error('Failed to generate API token', err);
      showError('Failed to generate API token');
    } finally {
      setIsGenerating(false);
    }
  };

  const handleCopy = async () => {
    const value = newTokenValue();
    if (!value) return;

    const success = await copyToClipboard(value);
    if (success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } else {
      showError('Failed to copy to clipboard');
    }
  };

  const handleDelete = async (record: APITokenRecord) => {
    const confirmed = window.confirm(
      `Revoke token "${record.name}"? Any agents or integrations using it will stop working.`,
    );
    if (!confirmed) return;

    try {
      await SecurityAPI.deleteToken(record.id);
      setTokens((prev) => prev.filter((token) => token.id !== record.id));
      showSuccess('Token revoked');
      props.onTokensChanged?.();

      const current = newTokenRecord();
      if (current && current.id === record.id) {
        setNewTokenValue(null);
        setNewTokenRecord(null);
      }
    } catch (err) {
      console.error('Failed to revoke API token', err);
      showError('Failed to revoke API token');
    }
  };

  const tokenHint = (record: APITokenRecord) => {
    if (record.prefix && record.suffix) {
      return `${record.prefix}…${record.suffix}`;
    }
    if (record.prefix) {
      return `${record.prefix}…`;
    }
    return '—';
  };

  return (
    <Card padding="none" class="overflow-hidden border border-gray-200 dark:border-gray-700" border={false}>
      <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center gap-3">
          <div class="p-2 bg-blue-100 dark:bg-blue-900/50 rounded-lg">
            <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
              />
            </svg>
          </div>
          <SectionHeader
            title="API tokens"
            description="Generate or revoke access tokens for automation and agents"
            size="sm"
            class="flex-1"
          />
        </div>
      </div>

      <div class="p-6 space-y-6">
        <div class="text-xs text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3">
          Issue a dedicated token for each host or automation. That way, if a system is compromised, you can revoke just its token without disrupting anything else.
        </div>

        <div class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300" for="api-token-name">
            Token name
          </label>
          <input
            id="api-token-name"
            type="text"
            value={nameInput()}
            onInput={(event) => setNameInput(event.currentTarget.value)}
            class="w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            type="button"
            class="inline-flex items-center justify-center px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            onClick={handleGenerate}
            disabled={isGenerating()}
          >
            {isGenerating() ? 'Generating…' : 'Generate API token'}
          </button>
        </div>

        <Show when={props.currentTokenHint && !tokens().length}>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            Current token hint: <span class="font-mono">{props.currentTokenHint}</span>
          </p>
        </Show>

        <Show when={newTokenValue()}>
          <div class="space-y-3">
            <div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4">
              <h4 class="text-sm font-semibold text-green-800 dark:text-green-200 mb-2">✅ New API token generated</h4>
              <p class="text-xs text-green-700 dark:text-green-300">
                Save this value now – it is only shown once. Update your automation or agents immediately.
              </p>
            </div>

            <div class="bg-gray-50 dark:bg-gray-900 rounded-lg p-3">
              <div class="flex items-center gap-2">
                <code class="flex-1 font-mono text-sm bg-white dark:bg-gray-800 px-3 py-2 rounded border border-gray-200 dark:border-gray-700 break-all">
                  {newTokenValue()}
                </code>
                <button
                  type="button"
                  onClick={handleCopy}
                  class="px-3 py-2 text-xs bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
                >
                  {copied() ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>
          </div>
        </Show>

        <div class="space-y-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Active tokens</h3>
          <Show
            when={!loading() && tokens().length > 0}
            fallback={
              <p class="text-sm text-gray-600 dark:text-gray-400">
                No API tokens yet. Generate one above to get started.
              </p>
            }
          >
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-gray-200 dark:border-gray-700">
                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Label</th>
                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Token hint</th>
                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Created</th>
                    <th class="text-left py-2 px-3 font-medium text-gray-600 dark:text-gray-400">Last used</th>
                    <th class="py-2 px-3 text-right font-medium text-gray-600 dark:text-gray-400">Actions</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={tokens()}>
                    {(token) => (
                      <tr>
                        <td class="py-2 px-3 text-gray-900 dark:text-gray-100">{token.name || 'Untitled token'}</td>
                        <td class="py-2 px-3 font-mono text-xs text-gray-600 dark:text-gray-400">{tokenHint(token)}</td>
                        <td class="py-2 px-3 text-gray-600 dark:text-gray-400">
                          {formatRelativeTime(new Date(token.createdAt).getTime())}
                        </td>
                        <td class="py-2 px-3 text-gray-600 dark:text-gray-400">
                          {token.lastUsedAt ? formatRelativeTime(new Date(token.lastUsedAt).getTime()) : 'Never'}
                        </td>
                        <td class="py-2 px-3 text-right">
                          <button
                            type="button"
                            onClick={() => handleDelete(token)}
                            class="inline-flex items-center px-3 py-1 text-xs font-medium text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900/30 rounded hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors"
                          >
                            Revoke
                          </button>
                        </td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </div>
      </div>
    </Card>
  );
};
