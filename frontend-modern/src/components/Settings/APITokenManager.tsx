import { Component, For, Show, createMemo, createSignal, onMount } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { showError, showSuccess } from '@/utils/toast';
import { formatRelativeTime } from '@/utils/format';
import { useWebSocket } from '@/App';
import type { DockerHost } from '@/types/api';
import { showTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';

interface APITokenManagerProps {
  currentTokenHint?: string;
  onTokensChanged?: () => void;
  refreshing?: boolean;
}

export const APITokenManager: Component<APITokenManagerProps> = (props) => {
  const { state } = useWebSocket();
  const dockerHosts = createMemo<DockerHost[]>(() => state.dockerHosts ?? []);
  const dockerTokenUsage = createMemo(() => {
    const usage = new Map<string, { count: number; hosts: string[] }>();
    for (const host of dockerHosts()) {
      const tokenId = host.tokenId;
      if (!tokenId) continue;
      const displayName = host.displayName?.trim() || host.hostname || host.id;
      const existing = usage.get(tokenId);
      if (existing) {
        usage.set(tokenId, {
          count: existing.count + 1,
          hosts: [...existing.hosts, displayName],
        });
      } else {
        usage.set(tokenId, { count: 1, hosts: [displayName] });
      }
    }
    return usage;
  });

  const [tokens, setTokens] = createSignal<APITokenRecord[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [newTokenValue, setNewTokenValue] = createSignal<string | null>(null);
  const [newTokenRecord, setNewTokenRecord] = createSignal<APITokenRecord | null>(null);
  const [nameInput, setNameInput] = createSignal('');
  const tokenRevealState = useTokenRevealState();

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
    try {
      const trimmedName = nameInput().trim() || undefined;
      const { token, record } = await SecurityAPI.createToken(trimmedName);

      setTokens((prev) => [record, ...prev]);
      setNewTokenValue(token);
      setNewTokenRecord(record);
      setNameInput('');

      showTokenReveal({
        token,
        record,
        source: 'security',
        note: 'Copy this token now. You can reopen this dialog from Security → API tokens while this page stays open.',
      });
      showSuccess('New API token generated. Copy it below while it is still visible.');
      props.onTokensChanged?.();

      try {
        window.localStorage.setItem('apiToken', token);
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

  const tokenHint = (record: APITokenRecord) => {
    if (record.prefix && record.suffix) {
      return `${record.prefix}…${record.suffix}`;
    }
    if (record.prefix) {
      return `${record.prefix}…`;
    }
    return '—';
  };

  const tokenNameForDialog = (record: APITokenRecord) => {
    if (record.name?.trim()) return record.name.trim();
    if (record.prefix && record.suffix) return `${record.prefix}…${record.suffix}`;
    if (record.prefix) return `${record.prefix}…`;
    return 'unnamed token';
  };

  const handleDelete = async (record: APITokenRecord) => {
    const usage = dockerTokenUsage().get(record.id);
    const displayName = tokenNameForDialog(record);

    let message = `Revoke token "${displayName}"? Any agents or integrations using it will stop working.`;

    if (usage) {
      const hostListPreview = usage.hosts.slice(0, 5).join(', ');
      const extraCount = usage.hosts.length - 5;
      const hostSummary =
        extraCount > 0 ? `${hostListPreview}, +${extraCount} more` : hostListPreview;
      const hostCountLabel =
        usage.count === 1 ? 'a Docker host' : `${usage.count} Docker hosts`;
      message = `Token "${displayName}" is currently used by ${hostCountLabel}.\nHosts: ${hostSummary}\n\nRevoking it will cause those agents to stop reporting until you update them with a new token.\n\nContinue?`;
    }

    const confirmed = window.confirm(message);
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

  const isRevealActiveForCurrentToken = () => {
    const active = tokenRevealState();
    if (!active) return false;
    return newTokenValue() !== null && active.token === newTokenValue();
  };

  const reopenTokenDialog = () => {
    const token = newTokenValue();
    const record = newTokenRecord();
    if (!token || !record) return;
    showTokenReveal({
      token,
      record,
      source: 'security',
      note: 'Copy this token now. Close the dialog once you have stored it safely.',
    });
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
        <Show when={props.refreshing}>
          <div class="flex items-center gap-2 rounded-md bg-blue-50 dark:bg-blue-900/30 border border-blue-200 dark:border-blue-800 px-3 py-2 text-xs text-blue-800 dark:text-blue-200">
            <svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke-width="4" stroke="currentColor" />
              <path class="opacity-75" d="M4 12a8 8 0 018-8" stroke-width="4" stroke-linecap="round" stroke="currentColor" />
            </svg>
            <span>Refreshing security status…</span>
          </div>
        </Show>

        {/* Generated token reminder - only show when dialog is not already visible */}
        <Show when={newTokenValue() && !isRevealActiveForCurrentToken()}>
          <div class="space-y-3 rounded-lg border border-green-300 dark:border-green-700 bg-green-50 dark:bg-green-900/30 p-4">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 rounded-full bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300 p-2">
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <div class="space-y-2">
                <h3 class="text-base font-semibold text-green-900 dark:text-green-100">
                  Token ready to copy
                </h3>
                <p class="text-sm text-green-800 dark:text-green-200 leading-snug">
                  We keep the full token inside a secure dialog. Reopen it if you still need to copy the value before navigating away.
                </p>
                <Show when={newTokenRecord()}>
                  <p class="text-xs text-green-900/80 dark:text-green-200/80">
                    Label{' '}
                    <span class="font-semibold">{newTokenRecord()?.name || 'Untitled token'}</span>
                    <Show when={newTokenRecord()?.prefix || newTokenRecord()?.suffix}>
                      {' '}· Hint{' '}
                      <code class="rounded bg-green-100 dark:bg-green-900/50 px-1.5 py-0.5 font-mono text-[11px] text-green-800 dark:text-green-200">
                        {tokenHint(newTokenRecord()!)}
                      </code>
                    </Show>
                  </p>
                </Show>
              </div>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={reopenTokenDialog}
                class="inline-flex items-center gap-1 rounded-md bg-green-600 hover:bg-green-700 text-white text-sm font-semibold px-4 py-2 transition-colors shadow-sm"
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7h4m0 0v4m0-4l-6 6-4-4-6 6" />
                </svg>
                Show token dialog
              </button>
              <button
                type="button"
                onClick={() => {
                  setNewTokenValue(null);
                  setNewTokenRecord(null);
                }}
                class="inline-flex items-center rounded-md border border-green-400 dark:border-green-700 px-4 py-2 text-sm font-medium text-green-800 dark:text-green-200 hover:bg-green-100 dark:hover:bg-green-900/40 transition-colors"
              >
                Dismiss reminder
              </button>
            </div>
          </div>
        </Show>

        <div class="text-xs text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-3">
          Issue a dedicated token for each host or automation. That way, if a system is compromised, you can revoke just its token without disrupting anything else.
        </div>

        <div class="space-y-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Generate new token</h3>
          <div class="flex gap-2">
            <input
              id="api-token-name"
              type="text"
              value={nameInput()}
              onInput={(event) => setNameInput(event.currentTarget.value)}
              placeholder="e.g., docker-host-1"
              class="flex-1 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <button
              type="button"
              class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
              onClick={handleGenerate}
              disabled={isGenerating()}
            >
              {isGenerating() ? 'Generating…' : 'Generate'}
            </button>
          </div>
        </div>

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
                    {(token) => {
                      const usage = dockerTokenUsage().get(token.id);
                      const hostTitle = usage ? usage.hosts.join(', ') : undefined;
                      const hostPreview = usage ? usage.hosts.slice(0, 2).join(', ') : '';
                      const extraCount = usage ? usage.hosts.length - 2 : 0;
                      const hostSummary =
                        usage && usage.count === 1
                          ? usage.hosts[0]
                          : usage
                              ? `${hostPreview}${extraCount > 0 ? `, +${extraCount} more` : ''}`
                              : '';
                      const hostCountLabel =
                        usage && usage.count === 1 ? 'host' : usage ? 'hosts' : '';

                      return (
                        <tr>
                          <td class="py-2 px-3 text-gray-900 dark:text-gray-100">
                            <div class="flex items-center gap-2">
                              <span>{token.name || 'Untitled token'}</span>
                              <Show when={usage}>
                                <span class="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
                                  Docker
                                </span>
                              </Show>
                            </div>
                            <Show when={usage}>
                              <div
                                class="mt-1 text-xs text-blue-700 dark:text-blue-300"
                                title={hostTitle}
                              >
                                Used by Docker {hostCountLabel}: {hostSummary}
                              </div>
                            </Show>
                          </td>
                          <td class="py-2 px-3 font-mono text-xs text-gray-600 dark:text-gray-400">
                            {tokenHint(token)}
                          </td>
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
                      );
                    }}
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
