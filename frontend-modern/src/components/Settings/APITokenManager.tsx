import { Component, For, Show, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { showError, showSuccess } from '@/utils/toast';
import { formatRelativeTime } from '@/utils/format';
import { useWebSocket } from '@/App';
import type { DockerHost } from '@/types/api';
import { showTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';
import {
  API_SCOPE_LABELS,
  API_SCOPE_OPTIONS,
  DOCKER_MANAGE_SCOPE,
  DOCKER_REPORT_SCOPE,
  HOST_AGENT_SCOPE,
  SETTINGS_READ_SCOPE,
  SETTINGS_WRITE_SCOPE,
} from '@/constants/apiScopes';

interface APITokenManagerProps {
  currentTokenHint?: string;
  onTokensChanged?: () => void;
  refreshing?: boolean;
}

const SCOPES_DOC_URL =
  'https://github.com/rcourtman/Pulse/blob/main/docs/CONFIGURATION.md#token-scopes';
const WILDCARD_SCOPE = '*';

export const APITokenManager: Component<APITokenManagerProps> = (props) => {
  const { state } = useWebSocket();
  const dockerHosts = createMemo<DockerHost[]>(() => state.dockerHosts ?? []);
  const dockerTokenUsage = createMemo(() => {
    const usage = new Map<string, { count: number; hosts: string[] }>();
    for (const host of dockerHosts()) {
      const tokenId = host.tokenId;
      if (!tokenId) continue;
      const displayName = host.displayName?.trim() || host.hostname || host.id;
      const previous = usage.get(tokenId);
      if (previous) {
        usage.set(tokenId, {
          count: previous.count + 1,
          hosts: [...previous.hosts, displayName],
        });
      } else {
        usage.set(tokenId, { count: 1, hosts: [displayName] });
      }
    }
    return usage;
  });

  const [tokens, setTokens] = createSignal<APITokenRecord[]>([]);
  const sortedTokens = createMemo(() =>
    [...tokens()].sort(
      (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
    ),
  );
  const totalTokens = createMemo(() => sortedTokens().length);
  const wildcardCount = createMemo(() =>
    sortedTokens().filter((token) => {
      const scopes = token.scopes;
      return !scopes || scopes.length === 0 || scopes.includes('*');
    }).length,
  );
  const scopedTokenCount = createMemo(() => totalTokens() - wildcardCount());
  const hasWildcardTokens = createMemo(() => wildcardCount() > 0);
  const mostRecentLabel = createMemo(() => {
    const first = sortedTokens()[0];
    if (!first) return '—';
    const timestamp = new Date(first.createdAt).getTime();
    return Number.isFinite(timestamp) ? formatRelativeTime(timestamp) : '—';
  });

  const [loading, setLoading] = createSignal(true);
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [newTokenValue, setNewTokenValue] = createSignal<string | null>(null);
  const [newTokenRecord, setNewTokenRecord] = createSignal<APITokenRecord | null>(null);
  const [nameInput, setNameInput] = createSignal('');
  const tokenRevealState = useTokenRevealState();
  const [selectedScopes, setSelectedScopes] = createSignal<string[]>([]);

  type ScopeGroup = (typeof API_SCOPE_OPTIONS)[number]['group'];
  type ScopeOption = (typeof API_SCOPE_OPTIONS)[number];
  const scopeGroupOrder: ScopeGroup[] = ['Monitoring', 'Agents', 'Settings'];
  const scopeGroups = createMemo<[ScopeGroup, ScopeOption[]][]>(() => {
    const grouped: Record<ScopeGroup, ScopeOption[]> = {
      Monitoring: [],
      Agents: [],
      Settings: [],
    };
    for (const option of API_SCOPE_OPTIONS) {
      grouped[option.group].push(option);
    }
    return scopeGroupOrder
      .map((group) => [group, grouped[group]] as [ScopeGroup, ScopeOption[]])
      .filter(([, options]) => options.length > 0);
  });

  const isFullAccessSelected = () =>
    selectedScopes().length === 0 || selectedScopes().includes(WILDCARD_SCOPE);

  const scopePresets: { label: string; scopes: string[]; description: string }[] = [
    {
      label: 'Host agent',
      scopes: [HOST_AGENT_SCOPE],
      description: 'Allow pulse-host-agent to submit OS, CPU, and disk metrics.',
  },
  {
    label: 'Docker report',
    scopes: [DOCKER_REPORT_SCOPE],
    description: 'Permits Docker agents to stream host and container telemetry only.',
  },
  {
    label: 'Docker manage',
    scopes: [DOCKER_REPORT_SCOPE, DOCKER_MANAGE_SCOPE],
    description: 'Extends Docker reporting with lifecycle actions (restart, stop, etc.).',
  },
  {
    label: 'Settings read',
    scopes: [SETTINGS_READ_SCOPE],
    description: 'Read configuration snapshots and diagnostics without modifying anything.',
  },
  {
    label: 'Settings admin',
    scopes: [SETTINGS_READ_SCOPE, SETTINGS_WRITE_SCOPE],
    description: 'Full settings read/write – equivalent to automation with admin privileges.',
    },
  ];

  const presetMatchesSelection = (presetScopes: string[]) => {
    const selection = [...selectedScopes()]
      .filter((scope) => scope !== WILDCARD_SCOPE)
      .sort();
    const target = [...presetScopes].sort();
    if (target.length === 0) {
      return isFullAccessSelected();
    }
    if (selection.length !== target.length) {
      return false;
    }
    return target.every((scope) => selection.includes(scope));
  };

  const presetButtonBase =
    'flex w-full items-start justify-between gap-3 rounded-md border px-3 py-2 text-left text-sm transition-colors';
  const presetButtonActive =
    'border-blue-400 ring-1 ring-blue-300 bg-blue-50/70 dark:border-blue-500 dark:ring-blue-400/40 dark:bg-blue-900/20';
  const presetButtonInactive =
    'border-gray-300 bg-white hover:border-blue-400 dark:border-gray-600 dark:bg-gray-900/60 dark:hover:border-blue-500';
  const selectedScopeChips = createMemo(() =>
    selectedScopes()
      .filter((scope) => scope !== WILDCARD_SCOPE)
      .map((scope) => ({
        value: scope,
        label: API_SCOPE_LABELS[scope] ?? scope,
      }))
      .sort((a, b) => a.label.localeCompare(b.label)),
  );
  const [advancedScopesOpen, setAdvancedScopesOpen] = createSignal(false);

  const applyScopePreset = (scopes: string[]) => {
    const unique = Array.from(new Set(scopes)).filter(Boolean);
    setSelectedScopes(unique);
  };
  const clearScopes = () => setSelectedScopes([]);

  let createSectionRef: HTMLDivElement | undefined;
  const [createHighlight, setCreateHighlight] = createSignal(false);
  let highlightTimer: number | undefined;
  const focusCreateSection = () => {
    if (!createSectionRef) return;
    createSectionRef.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setCreateHighlight(true);
    window.clearTimeout(highlightTimer);
    highlightTimer = window.setTimeout(() => setCreateHighlight(false), 1600);
  };
  onCleanup(() => {
    if (highlightTimer) window.clearTimeout(highlightTimer);
  });

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
      const scopeSelection = [...selectedScopes()].sort();
      const scopePayload = scopeSelection.length > 0 ? scopeSelection : undefined;
      const { token, record } = await SecurityAPI.createToken(trimmedName, scopePayload);

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
    return 'untitled token';
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

    if (!window.confirm(message)) return;

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
    <div class="space-y-6">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div class="space-y-1">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">API tokens</h3>
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Authenticate host agents, Docker integrations, and automation pipelines with scoped access.
          </p>
        </div>
        <button
          type="button"
          onClick={focusCreateSection}
          class="inline-flex items-center gap-2 self-start rounded-md border border-blue-200 bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1"
        >
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Generate token
        </button>
      </div>

      <Show when={props.refreshing}>
        <div class="flex items-center gap-2 rounded-md border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/30 px-3 py-2 text-xs text-blue-800 dark:text-blue-200">
          <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke-width="4" stroke="currentColor" />
            <path class="opacity-75" d="M4 12a8 8 0 018-8" stroke-width="4" stroke-linecap="round" stroke="currentColor" />
          </svg>
          <span>Refreshing security status…</span>
        </div>
      </Show>

      <div class="space-y-6">
        <Card padding="lg" class="space-y-6">
          <div class="space-y-2">
            <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">Active tokens</h4>
            <p class="text-sm text-gray-600 dark:text-gray-400">
              Rotate tokens regularly and scope them to the minimum access required.
            </p>
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900/40 p-3">
              <p class="text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Active tokens
              </p>
              <p class="text-xl font-semibold text-gray-900 dark:text-gray-100">{totalTokens()}</p>
            </div>
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900/40 p-3">
              <p class="text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Scoped tokens
              </p>
              <p class="text-xl font-semibold text-gray-900 dark:text-gray-100">{scopedTokenCount()}</p>
            </div>
            <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900/40 p-3">
              <p class="text-[11px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {hasWildcardTokens() ? 'Full access tokens' : 'Last generated'}
              </p>
              <p class="text-xl font-semibold text-gray-900 dark:text-gray-100">
                {hasWildcardTokens()
                  ? wildcardCount()
                  : totalTokens() > 0
                    ? mostRecentLabel()
                    : '—'}
              </p>
            </div>
          </div>

          <Show
            when={!loading() && totalTokens() > 0}
            fallback={
              <div class="rounded-lg border border-dashed border-gray-300 dark:border-gray-700/70 bg-white/60 dark:bg-gray-900/30 p-5 text-sm text-gray-600 dark:text-gray-400">
                <p class="mb-3">
                  No tokens yet. Generate one to authenticate agents and automation.
                </p>
                <button
                  type="button"
                  onClick={focusCreateSection}
                  class="inline-flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-1.5 text-xs font-semibold text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200"
                >
                  Generate token
                </button>
              </div>
            }
          >
            <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-sm">
                <thead class="bg-gray-50 dark:bg-gray-900/60">
                  <tr>
                    <th class="py-2 px-3 text-left font-medium text-gray-600 dark:text-gray-400">Label</th>
                    <th class="py-2 px-3 text-left font-medium text-gray-600 dark:text-gray-400">Token hint</th>
                    <th class="py-2 px-3 text-left font-medium text-gray-600 dark:text-gray-400">Scopes</th>
                    <th class="py-2 px-3 text-left font-medium text-gray-600 dark:text-gray-400">Created</th>
                    <th class="py-2 px-3 text-left font-medium text-gray-600 dark:text-gray-400">Last used</th>
                    <th class="py-2 px-3 text-right font-medium text-gray-600 dark:text-gray-400">Actions</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                  <For each={sortedTokens()}>
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
                      const rawScopes = token.scopes && token.scopes.length > 0 ? token.scopes : ['*'];
                      const scopeBadges = rawScopes.includes('*')
                        ? [{ value: '*', label: 'Full access' }]
                        : rawScopes.map((scope) => ({
                            value: scope,
                            label: API_SCOPE_LABELS[scope] ?? scope,
                          }));
                      const rowIsWildcard = scopeBadges.some((scope) => scope.value === '*');

                      return (
                        <tr
                          class={`transition-colors ${rowIsWildcard ? 'bg-amber-50/60 dark:bg-amber-900/15' : 'bg-white dark:bg-gray-900/20'} hover:bg-gray-50 dark:hover:bg-gray-800/60`}
                        >
                          <td class="py-3 px-3 text-gray-900 dark:text-gray-100">
                            <div class="flex items-center gap-2">
                              <span class="font-medium">{token.name || 'Untitled token'}</span>
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
                          <td class="py-3 px-3 font-mono text-xs text-gray-600 dark:text-gray-400">
                            {tokenHint(token)}
                          </td>
                          <td class="py-3 px-3">
                            <div class="flex flex-wrap gap-1">
                              <For each={scopeBadges}>
                                {(scope) => {
                                  const isWildcard = scope.value === '*';
                                  const badgeClass = isWildcard
                                    ? 'inline-flex items-center rounded-full bg-amber-100 dark:bg-amber-900/40 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:text-amber-200'
                                    : 'inline-flex items-center rounded-full bg-gray-100 dark:bg-gray-800 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:text-gray-200';
                                  return (
                                    <span class={badgeClass} title={scope.value}>
                                      {scope.label}
                                    </span>
                                  );
                                }}
                              </For>
                            </div>
                          </td>
                          <td class="py-3 px-3 text-gray-600 dark:text-gray-400">
                            {formatRelativeTime(new Date(token.createdAt).getTime())}
                          </td>
                          <td class="py-3 px-3 text-gray-600 dark:text-gray-400">
                            {token.lastUsedAt ? formatRelativeTime(new Date(token.lastUsedAt).getTime()) : 'Never'}
                          </td>
                          <td class="py-3 px-3 text-right">
                            <button
                              type="button"
                              onClick={() => handleDelete(token)}
                              class="inline-flex items-center rounded-md bg-red-50 px-3 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:bg-red-900/30 dark:text-red-300 dark:hover:bg-red-900/50"
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
        </Card>

        <Show when={newTokenValue() && !isRevealActiveForCurrentToken()}>
          <Card padding="lg" tone="success" class="space-y-3 border border-green-300 dark:border-green-700">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 rounded-full bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300 p-2">
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <div class="space-y-2">
                <h3 class="text-base font-semibold text-green-900 dark:text-green-100">
                  Token ready to copy
                </h3>
                <p class="text-sm leading-snug text-green-800 dark:text-green-200">
                  Tokens are only shown once. Copy it now or store it securely before you leave this page.
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
                class="inline-flex items-center gap-2 rounded-md bg-green-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-green-700"
              >
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor">
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
                class="inline-flex items-center rounded-md border border-green-500 px-4 py-2 text-sm font-medium text-green-800 transition-colors hover:bg-green-100 dark:border-green-600 dark:text-green-200 dark:hover:bg-green-900/40"
              >
                Dismiss
              </button>
            </div>
          </Card>
        </Show>

        <Card
          padding="lg"
          class={`space-y-6 transition-shadow duration-300 ${createHighlight() ? 'ring-2 ring-blue-500/60 dark:ring-blue-400/60 shadow-lg' : ''}`}
          ref={(el: HTMLDivElement) => {
            createSectionRef = el;
          }}
        >
          <div class="space-y-6">
            <div class="space-y-1">
              <h4 class="text-base font-semibold text-gray-900 dark:text-gray-100">Generate new token</h4>
              <p class="text-sm text-gray-600 dark:text-gray-400">
                Tokens are only displayed once. Follow the steps below to create a scoped credential.
              </p>
            </div>

            <ol class="space-y-6 text-sm text-gray-700 dark:text-gray-300">
              <li class="flex gap-3">
                <div class="mt-1 flex h-6 w-6 items-center justify-center rounded-full bg-blue-100 text-xs font-semibold uppercase text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                  1
                </div>
                <div class="flex-1 space-y-2">
                  <p class="font-medium text-gray-900 dark:text-gray-100">Name the token</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    Use something descriptive so you can identify the integration later.
                  </p>
                  <input
                    id="api-token-name"
                    type="text"
                    value={nameInput()}
                    onInput={(event) => setNameInput(event.currentTarget.value)}
                    placeholder="e.g., docker-host-1"
                    class="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  />
                </div>
              </li>

              <li class="flex gap-3">
                <div class="mt-1 flex h-6 w-6 items-center justify-center rounded-full bg-blue-100 text-xs font-semibold uppercase text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                  2
                </div>
                <div class="flex-1 space-y-2">
                  <p class="font-medium text-gray-900 dark:text-gray-100">Set a baseline scope</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    Start with a preset that matches the integration, or choose full access if you plan to trim later.
                  </p>
                  <div class="space-y-2">
                    <button
                      type="button"
                      class={`${presetButtonBase} ${isFullAccessSelected() ? presetButtonActive : presetButtonInactive}`}
                      onClick={clearScopes}
                    >
                      <div>
                        <p class="font-semibold text-gray-900 dark:text-gray-100">Full access</p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">Legacy wildcard token – grants every permission.</p>
                      </div>
                      <span class={`text-xs font-semibold uppercase ${isFullAccessSelected() ? 'text-blue-600 dark:text-blue-300' : 'text-gray-500 dark:text-gray-400'}`}>
                        {isFullAccessSelected() ? 'Selected' : 'Reset'}
                      </span>
                    </button>
                    <For each={scopePresets}>
                      {(preset) => (
                        <button
                          type="button"
                          class={`${presetButtonBase} ${presetMatchesSelection(preset.scopes) ? presetButtonActive : presetButtonInactive}`}
                          onClick={() => applyScopePreset(preset.scopes)}
                        >
                          <div>
                            <p class="font-semibold text-gray-900 dark:text-gray-100">{preset.label}</p>
                            <p class="text-xs text-gray-500 dark:text-gray-400">{preset.description}</p>
                          </div>
                          <span class={`text-xs font-semibold uppercase ${presetMatchesSelection(preset.scopes) ? 'text-blue-600 dark:text-blue-300' : 'text-blue-500 dark:text-blue-300/80'}`}>
                            {presetMatchesSelection(preset.scopes) ? 'Selected' : 'Apply'}
                          </span>
                        </button>
                      )}
                    </For>
                  </div>
                  <div class="rounded-md border border-dashed border-gray-300 bg-white px-3 py-2 text-xs text-gray-600 dark:border-gray-600 dark:bg-gray-900/50 dark:text-gray-300">
                    <span class="block text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                      Current selection
                    </span>
                    <Show
                      when={!isFullAccessSelected() && selectedScopeChips().length > 0}
                      fallback={<span class="block pt-1 text-xs text-gray-700 dark:text-gray-200">Full access (wildcard)</span>}
                    >
                      <div class="pt-1 flex flex-wrap gap-1.5">
                        <For each={selectedScopeChips()}>
                          {(chip) => (
                            <span class="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                              {chip.label}
                            </span>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                </div>
              </li>

              <li class="flex gap-3">
                <div class="mt-1 flex h-6 w-6 items-center justify-center rounded-full bg-blue-100 text-xs font-semibold uppercase text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                  3
                </div>
                <div class="flex-1 space-y-3">
                  <div>
                    <p class="font-medium text-gray-900 dark:text-gray-100">Fine-tune permissions</p>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      Toggle advanced scopes if the integration needs additional access beyond the preset.
                    </p>
                  </div>
                  <div class="space-y-2">
                    <Show when={!isFullAccessSelected() && selectedScopeChips().length > 0}>
                      <div class="flex flex-wrap gap-1.5">
                        <For each={selectedScopeChips()}>
                          {(chip) => (
                            <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                              {chip.value}
                            </span>
                          )}
                        </For>
                      </div>
                    </Show>
                    <button
                      type="button"
                      onClick={() => setAdvancedScopesOpen((open) => !open)}
                      class={`inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-wide transition-colors ${advancedScopesOpen() ? 'text-blue-600 dark:text-blue-300' : 'text-gray-600 hover:text-blue-600 dark:text-gray-300 dark:hover:text-blue-300'}`}
                    >
                      <svg
                        class={`h-3 w-3 transition-transform ${advancedScopesOpen() ? 'rotate-180' : ''}`}
                        viewBox="0 0 12 12"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.8"
                      >
                        <path d="M3 4.5L6 7.5L9 4.5" stroke-linecap="round" stroke-linejoin="round" />
                      </svg>
                      {advancedScopesOpen() ? 'Hide advanced scopes' : 'Add or remove individual scopes'}
                    </button>
                  </div>
                  <Show when={advancedScopesOpen()}>
                    <div class="space-y-4">
                      <For each={scopeGroups()}>
                        {([group, options]) => (
                          <div class="space-y-2">
                            <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                              {group}
                            </p>
                            <div class="grid gap-2 sm:grid-cols-2">
                              <For each={options}>
                                {(option) => {
                                  const inputId = `scope-${option.value.replace(/[:]/g, '-')}`;
                                  const checked = () => selectedScopes().includes(option.value);
                                  return (
                                    <label
                                      for={inputId}
                                      class={`flex items-start gap-3 rounded-lg border px-3 py-2 text-sm transition-colors ${checked() ? 'border-blue-400 bg-blue-50/70 ring-1 ring-blue-300 dark:border-blue-500 dark:bg-blue-900/30 dark:ring-blue-400/40' : 'border-gray-200 bg-white hover:border-blue-400 dark:border-gray-600 dark:bg-gray-900/50 dark:hover:border-blue-500'}`}
                                    >
                                      <input
                                        id={inputId}
                                        type="checkbox"
                                        class="mt-1 h-4 w-4 flex-shrink-0 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600"
                                        checked={checked()}
                                        onInput={(event) => {
                                          const isChecked = event.currentTarget.checked;
                                          setSelectedScopes((prev) => {
                                            if (isChecked) {
                                              if (prev.includes(option.value)) return prev;
                                              return [...prev, option.value];
                                            }
                                            return prev.filter((value) => value !== option.value);
                                          });
                                        }}
                                      />
                                      <div class="space-y-1">
                                        <p class="font-medium text-gray-900 dark:text-gray-100">
                                          {option.label}
                                        </p>
                                        <Show when={option.description}>
                                          <p class="text-xs leading-snug text-gray-500 dark:text-gray-400">
                                            {option.description}
                                          </p>
                                        </Show>
                                        <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                                          {option.value}
                                        </span>
                                      </div>
                                    </label>
                                  );
                                }}
                              </For>
                            </div>
                          </div>
                        )}
                      </For>
                    </div>
                  </Show>
                </div>
              </li>
            </ol>
          </div>

          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <button
              type="button"
              class="inline-flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-4 py-2 text-sm font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
              onClick={handleGenerate}
              disabled={isGenerating()}
            >
              {isGenerating() ? (
                <>
                  <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke-width="4" stroke="currentColor" />
                    <path class="opacity-75" d="M4 12a8 8 0 018-8" stroke-width="4" stroke-linecap="round" stroke="currentColor" />
                  </svg>
                  Generating…
                </>
              ) : (
                <>
                  <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                  </svg>
                  Generate token
                </>
              )}
            </button>
            <Show when={newTokenValue() && !isRevealActiveForCurrentToken()}>
              <button
                type="button"
                onClick={reopenTokenDialog}
                class="inline-flex items-center gap-2 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-800 dark:bg-gray-700 dark:hover:bg-gray-600"
              >
                View last token
              </button>
            </Show>
          </div>
        </Card>

        <Show when={!loading() && hasWildcardTokens()}>
          <Card padding="lg" tone="warning" class="space-y-3 border border-amber-300 dark:border-amber-700">
            <h4 class="text-sm font-semibold text-amber-900 dark:text-amber-200">
              Full access tokens detected
            </h4>
            <p class="text-xs text-amber-900/90 dark:text-amber-100/90">
              Edit existing tokens to assign scopes, or generate replacements with the presets above so compromised credentials can’t control everything.
            </p>
            <button
              type="button"
              onClick={focusCreateSection}
              class="inline-flex w-fit items-center gap-2 rounded-md border border-amber-300 bg-amber-100 px-3 py-1.5 text-xs font-semibold text-amber-800 transition-colors hover:bg-amber-200 dark:border-amber-600 dark:bg-amber-900/30 dark:text-amber-100"
            >
              Review scopes
            </button>
          </Card>
        </Show>

        <Card padding="lg" tone="muted" class="space-y-3">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">Good practices</h4>
          <ul class="space-y-2 text-sm text-gray-600 dark:text-gray-300">
            <li class="flex gap-3">
              <span class="mt-1 h-1.5 w-1.5 rounded-full bg-blue-500 dark:bg-blue-400" />
              <span>Issue separate tokens for Docker agents, host agents, and automation pipelines so you can revoke them independently.</span>
            </li>
            <li class="flex gap-3">
              <span class="mt-1 h-1.5 w-1.5 rounded-full bg-blue-500 dark:bg-blue-400" />
              <span>Rotate tokens on a schedule and remove ones that haven’t been used recently.</span>
            </li>
            <li class="flex gap-3">
              <span class="mt-1 h-1.5 w-1.5 rounded-full bg-blue-500 dark:bg-blue-400" />
              <span>
                View the{' '}
                <a
                  class="font-medium text-blue-600 underline decoration-transparent transition-colors hover:decoration-blue-500 dark:text-blue-300"
                  href={SCOPES_DOC_URL}
                  target="_blank"
                  rel="noreferrer"
                >
                  scoped token guide
                </a>{' '}
                for the full list of available permissions.
              </span>
            </li>
          </ul>
        </Card>
      </div>
    </div>
  );
};

export default APITokenManager;
