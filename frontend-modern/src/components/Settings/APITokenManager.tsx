import { Component, For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { notificationStore } from '@/stores/notifications';
import { formatRelativeTime } from '@/utils/format';
import { useWebSocket } from '@/App';
import type { DockerHost, Host } from '@/types/api';
import { getPulseBaseUrl } from '@/utils/url';
import { showTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';
import { logger } from '@/utils/logger';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import {
  API_SCOPE_LABELS,
  API_SCOPE_OPTIONS,
  DOCKER_MANAGE_SCOPE,
  DOCKER_REPORT_SCOPE,
  HOST_AGENT_SCOPE,
  MONITORING_READ_SCOPE,
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
  const { state, markDockerHostsTokenRevoked, markHostsTokenRevoked } = useWebSocket();
  const dockerHosts = createMemo<DockerHost[]>(() => state.dockerHosts ?? []);
  const hosts = createMemo<Host[]>(() => state.hosts ?? []);
  const dockerTokenUsage = createMemo(() => {
    type UsageHost = { id: string; label: string };
    const usage = new Map<string, { count: number; hosts: UsageHost[] }>();
    for (const host of dockerHosts()) {
      const tokenId = host.tokenId;
      if (!tokenId) continue;
      const label = host.displayName?.trim() || host.hostname || host.id;
      const previous = usage.get(tokenId);
      if (previous) {
        usage.set(tokenId, {
          count: previous.count + 1,
          hosts: [...previous.hosts, { id: host.id, label }],
        });
      } else {
        usage.set(tokenId, { count: 1, hosts: [{ id: host.id, label }] });
      }
    }
    return usage;
  });
  const hostTokenUsage = createMemo(() => {
    type UsageHost = { id: string; label: string };
    const usage = new Map<string, { count: number; hosts: UsageHost[] }>();
    for (const host of hosts()) {
      const tokenId = host.tokenId;
      if (!tokenId) continue;
      const label = host.displayName?.trim() || host.hostname || host.id;
      const previous = usage.get(tokenId);
      if (previous) {
        usage.set(tokenId, {
          count: previous.count + 1,
          hosts: [...previous.hosts, { id: host.id, label }],
        });
      } else {
        usage.set(tokenId, { count: 1, hosts: [{ id: host.id, label }] });
      }
    }
    return usage;
  });

  const [tokens, setTokens] = createSignal<APITokenRecord[]>([]);
  const [tokensLoaded, setTokensLoaded] = createSignal(false);
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
      label: 'Kiosk / Dashboard',
      scopes: [MONITORING_READ_SCOPE],
      description: 'Read-only access for wall displays. Use ?token=xxx&kiosk=1 in the URL to hide navigation and filters.',
    },
    {
      label: 'Host agent',
      scopes: [HOST_AGENT_SCOPE],
      description: 'Allow pulse-host-agent to submit OS, CPU, and disk metrics.',
    },
    {
      label: 'Container report',
      scopes: [DOCKER_REPORT_SCOPE],
      description: 'Permits container agents (Docker or Podman) to stream host and container telemetry only.',
    },
    {
      label: 'Container manage',
      scopes: [DOCKER_REPORT_SCOPE, DOCKER_MANAGE_SCOPE],
      description: 'Extends container reporting with lifecycle actions (restart, stop, etc.).',
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
    setTokensLoaded(false);
    try {
      const list = await SecurityAPI.listTokens();
      setTokens(list);
      setTokensLoaded(true);
    } catch (err) {
      logger.error('Failed to load API tokens', err);
      notificationStore.error('Failed to load API tokens');
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    void loadTokens();
  });

  createEffect(() => {
    if (!tokensLoaded()) return;
    const activeTokenIds = new Set(tokens().map((token) => token.id));
    const pendingDockerByToken = new Map<string, string[]>();

    for (const host of dockerHosts()) {
      const tokenId = host.tokenId;
      if (!tokenId) continue;
      if (activeTokenIds.has(tokenId)) continue;
      if (host.revokedTokenId === tokenId) continue;

      if (!pendingDockerByToken.has(tokenId)) {
        pendingDockerByToken.set(tokenId, []);
      }
      pendingDockerByToken.get(tokenId)!.push(host.id);
    }

    pendingDockerByToken.forEach((hostIds, tokenId) => {
      if (hostIds.length === 0) return;
      markDockerHostsTokenRevoked(tokenId, hostIds);
    });

    const pendingHostsByToken = new Map<string, string[]>();
    for (const host of hosts()) {
      const tokenId = host.tokenId;
      if (!tokenId) continue;
      if (activeTokenIds.has(tokenId)) continue;
      if (host.revokedTokenId === tokenId && host.tokenRevokedAt) continue;

      if (!pendingHostsByToken.has(tokenId)) {
        pendingHostsByToken.set(tokenId, []);
      }
      pendingHostsByToken.get(tokenId)!.push(host.id);
    }

    pendingHostsByToken.forEach((hostIds, tokenId) => {
      if (hostIds.length === 0) return;
      markHostsTokenRevoked(tokenId, hostIds);
    });
  });

  const handleGenerate = async () => {
    setIsGenerating(true);
    try {
      const trimmedName = nameInput().trim() || undefined;
      const scopeSelection = [...selectedScopes()].sort();
      const scopePayload = scopeSelection.length > 0 ? scopeSelection : undefined;
      const { token, record } = await SecurityAPI.createToken(trimmedName, scopePayload);

      setTokens((prev) => [record, ...prev]);
      setNewTokenRecord(record);
      setNewTokenValue(token);
      setNameInput('');

      showTokenReveal({
        token,
        record,
        source: 'security',
        note: 'Copy this token now. You can reopen this dialog from Security → API tokens while this page stays open.',
      });
      notificationStore.success('New API token generated. Copy it below while it is still visible.');
      props.onTokensChanged?.();
    } catch (err) {
      logger.error('Failed to generate API token', err);
      notificationStore.error('Failed to generate API token');
    } finally {
      setIsGenerating(false);
    }
  };

  const tokenHint = (record: APITokenRecord | null | undefined) => {
    if (!record) return '—';
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
    const dockerUsage = dockerTokenUsage().get(record.id);
    const hostUsage = hostTokenUsage().get(record.id);
    const displayName = tokenNameForDialog(record);

    const affectedDockerHostIds = dockerUsage ? dockerUsage.hosts.map((host) => host.id) : [];
    const affectedHostAgentIds = hostUsage ? hostUsage.hosts.map((host) => host.id) : [];
    let revokeMessage: string | undefined;
    const messageChunks: string[] = [];
    if (dockerUsage) {
      const hostListPreview = dockerUsage.hosts
        .slice(0, 5)
        .map((host) => host.label)
        .join(', ');
      const extraCount = dockerUsage.hosts.length - 5;
      const hostSummary =
        extraCount > 0 ? `${hostListPreview}, +${extraCount} more` : hostListPreview;
      const hostCountLabel =
        dockerUsage.count === 1 ? 'container host' : `${dockerUsage.count} container hosts`;
      messageChunks.push(`${hostCountLabel}: ${hostSummary}`);
    }
    if (hostUsage) {
      const agentListPreview = hostUsage.hosts
        .slice(0, 5)
        .map((host) => host.label)
        .join(', ');
      const agentExtra = hostUsage.hosts.length - 5;
      const agentSummary =
        agentExtra > 0 ? `${agentListPreview}, +${agentExtra} more` : agentListPreview;
      const agentCountLabel =
        hostUsage.count === 1 ? 'host agent' : `${hostUsage.count} host agents`;
      messageChunks.push(`${agentCountLabel}: ${agentSummary}`);
    }
    if (messageChunks.length > 0) {
      revokeMessage = `Token "${displayName}" was previously used by ${messageChunks.join(' • ')}. Update those agents with a new token.`;
    }

    try {
      await SecurityAPI.deleteToken(record.id);
      setTokens((prev) => prev.filter((token) => token.id !== record.id));
      notificationStore.success(revokeMessage ? `Token revoked: ${revokeMessage}` : 'Token revoked');
      props.onTokensChanged?.();
      if (affectedDockerHostIds.length > 0) {
        markDockerHostsTokenRevoked(record.id, affectedDockerHostIds);
      }
      if (affectedHostAgentIds.length > 0) {
        markHostsTokenRevoked(record.id, affectedHostAgentIds);
      }

      const current = newTokenRecord();
      if (current && current.id === record.id) {
        setNewTokenValue(null);
        setNewTokenRecord(null);
      }
    } catch (err) {
      logger.error('Failed to revoke API token', err);
      notificationStore.error('Failed to revoke API token');
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
    <div class="space-y-5">
      <Card
        padding="lg"
        class="border border-gray-200/80 bg-gray-50 shadow-sm dark:border-gray-700/80 dark:bg-gray-900/60"
      >
        <div class="flex flex-col gap-6">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div class="flex flex-wrap items-center gap-3">
              <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-blue-600/10 text-blue-600 dark:bg-blue-500/20 dark:text-blue-200">
                <BadgeCheck class="h-5 w-5" />
              </div>
              <SectionHeader
                label="Token inventory"
                title="API tokens"
                description="Monitor usage, rotate credentials, and issue scoped access for automation."
                size="sm"
                class="flex-1"
              />
            </div>
            <button
              type="button"
              onClick={focusCreateSection}
              class="inline-flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900"
            >
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
              </svg>
              New token
            </button>
          </div>

          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <div class="rounded-lg border border-gray-200/70 bg-white/70 p-4 text-sm text-gray-700 shadow-sm dark:border-gray-700/70 dark:bg-gray-900/40 dark:text-gray-300">
              <div class="text-[0.7rem] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Total tokens
              </div>
              <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">
                {totalTokens()}
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Stored credentials across all agents
              </p>
            </div>
            <div class="rounded-lg border border-gray-200/70 bg-white/70 p-4 text-sm text-gray-700 shadow-sm dark:border-gray-700/70 dark:bg-gray-900/40 dark:text-gray-300">
              <div class="text-[0.7rem] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Scoped tokens
              </div>
              <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-gray-100">
                {scopedTokenCount()}
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Limited access tokens with defined scopes
              </p>
            </div>
            <div
              class={`rounded-lg border p-4 text-sm shadow-sm ${hasWildcardTokens()
                ? 'border-amber-300/80 bg-amber-50/80 text-amber-900 dark:border-amber-700/70 dark:bg-amber-900/20 dark:text-amber-100'
                : 'border-gray-200/70 bg-white/70 text-gray-700 dark:border-gray-700/70 dark:bg-gray-900/40 dark:text-gray-300'
                }`}
            >
              <div
                class={`text-[0.7rem] font-semibold uppercase tracking-wide ${hasWildcardTokens()
                  ? 'text-amber-700 dark:text-amber-300'
                  : 'text-gray-500 dark:text-gray-400'
                  }`}
              >
                Full access tokens
              </div>
              <div
                class={`mt-1 text-2xl font-semibold ${hasWildcardTokens()
                  ? 'text-amber-800 dark:text-amber-100'
                  : 'text-gray-900 dark:text-gray-100'
                  }`}
              >
                {wildcardCount()}
              </div>
              <p
                class={`mt-1 text-xs ${hasWildcardTokens()
                  ? 'text-amber-700 dark:text-amber-200'
                  : 'text-gray-500 dark:text-gray-400'
                  }`}
              >
                {hasWildcardTokens()
                  ? 'Legacy wildcard tokens – rotate into scoped presets when possible.'
                  : 'All tokens scoped – no wildcard credentials detected.'}
              </p>
            </div>
          </div>
        </div>
      </Card>

      <Show when={props.refreshing}>
        <Card
          tone="info"
          padding="sm"
          class="flex items-center gap-2 border border-blue-200/70 text-xs text-blue-800 dark:border-blue-800/70 dark:text-blue-200"
        >
          <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke-width="4" stroke="currentColor" />
            <path class="opacity-75" d="M4 12a8 8 0 018-8" stroke-width="4" stroke-linecap="round" stroke="currentColor" />
          </svg>
          <span>Refreshing security status…</span>
        </Card>
      </Show>

      <Show when={newTokenValue() && !isRevealActiveForCurrentToken()}>
        <div class="space-y-3">
          <Card
            tone="success"
            padding="sm"
            class="flex flex-wrap items-center justify-between gap-3 border border-green-300/70 text-sm text-green-800 dark:border-green-700/70 dark:text-green-200"
          >
            <span>
              ✓ Token generated: <strong>{newTokenRecord()?.name || 'Untitled'}</strong> ({tokenHint(newTokenRecord())})
            </span>
            <div class="flex items-center gap-3 text-xs">
              <button onClick={reopenTokenDialog} class="font-medium underline decoration-green-500/50 underline-offset-2 hover:text-green-900 dark:hover:text-green-100">
                Show
              </button>
              <button
                onClick={() => {
                  setNewTokenValue(null);
                  setNewTokenRecord(null);
                }}
                class="font-medium underline decoration-green-500/50 underline-offset-2 hover:text-green-900 dark:hover:text-green-100"
              >
                Dismiss
              </button>
            </div>
          </Card>

          <Show when={newTokenRecord()?.scopes?.length === 1 && newTokenRecord()?.scopes?.[0] === MONITORING_READ_SCOPE}>
            <div class="rounded-lg border border-blue-200 bg-blue-50/50 p-4 text-sm text-blue-900 shadow-sm dark:border-blue-800/30 dark:bg-blue-900/10 dark:text-blue-100">
              <div class="mb-2 font-semibold">Magic Kiosk Link</div>
              <p class="mb-3 text-xs text-blue-700 dark:text-blue-300">
                Use this link to open Pulse directly in Kiosk mode without logging in. Perfect for wall displays and digital signage.
              </p>
              <div class="flex items-center gap-2">
                <code class="flex-1 rounded border border-blue-200 bg-white px-3 py-2 font-mono text-xs text-blue-800 dark:border-blue-800 dark:bg-black/20 dark:text-blue-200 break-all">
                  {getPulseBaseUrl()}/?token={newTokenValue()}&kiosk=1
                </code>
                <button
                  type="button"
                  onClick={() => {
                    const url = `${getPulseBaseUrl()}/?token=${newTokenValue()}&kiosk=1`;
                    navigator.clipboard.writeText(url);
                    notificationStore.success('Link copied to clipboard');
                  }}
                  class="flex-shrink-0 rounded-md bg-blue-600 px-3 py-2 text-xs font-semibold text-white shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-1 dark:bg-blue-600 dark:hover:bg-blue-500"
                >
                  Copy Link
                </button>
              </div>
            </div>
          </Show>
        </div>
      </Show>

      <Show
        when={!loading() && totalTokens() > 0}
        fallback={
          <Card
            tone="muted"
            padding="md"
            class="border border-dashed border-gray-300/80 text-sm text-gray-600 dark:border-gray-600/70 dark:text-gray-400"
          >
            No tokens yet.{' '}
            <button onClick={focusCreateSection} class="font-medium text-blue-600 underline-offset-2 hover:underline dark:text-blue-400">
              Create one
            </button>{' '}
            to authenticate agents and integrations.
          </Card>
        }
      >
        <Card padding="none" tone="glass" class="overflow-hidden">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 bg-gray-50/60 px-5 py-4 dark:border-gray-700 dark:bg-gray-900/40">
            <div>
              <h4 class="text-sm font-semibold text-gray-800 dark:text-gray-100">Token inventory</h4>
              <p class="text-xs text-gray-600 dark:text-gray-400">
                Active credentials sorted by most recent creation date.
              </p>
            </div>
            <button
              type="button"
              onClick={focusCreateSection}
              class="inline-flex items-center gap-2 rounded-md border border-blue-200 px-3 py-1.5 text-xs font-semibold text-blue-600 transition hover:bg-blue-50 dark:border-blue-700 dark:text-blue-200 dark:hover:bg-blue-900/20"
            >
              Generate new
            </button>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead class="bg-gray-100/80 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:bg-gray-900/50 dark:text-gray-400">
                <tr>
                  <th class="px-5 py-3">Name</th>
                  <th class="px-5 py-3">Hint</th>
                  <th class="px-5 py-3">Scopes</th>
                  <th class="px-5 py-3">Usage</th>
                  <th class="px-5 py-3">Created</th>
                  <th class="px-5 py-3">Last used</th>
                  <th class="px-5 py-3 text-right">Action</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-gray-800">
                <For each={sortedTokens()}>
                  {(token) => {
                    const dockerUsageEntry = dockerTokenUsage().get(token.id);
                    const hostUsageEntry = hostTokenUsage().get(token.id);
                    const usageSegments: string[] = [];
                    const usageTitleSegments: string[] = [];
                    if (dockerUsageEntry) {
                      usageSegments.push(
                        dockerUsageEntry.count === 1
                          ? dockerUsageEntry.hosts[0]?.label ?? 'Container host'
                          : `${dockerUsageEntry.count} container hosts`,
                      );
                      usageTitleSegments.push(
                        `Container hosts: ${dockerUsageEntry.hosts.map((host) => host.label).join(', ')}`,
                      );
                    }
                    if (hostUsageEntry) {
                      usageSegments.push(
                        hostUsageEntry.count === 1
                          ? `${hostUsageEntry.hosts[0]?.label ?? 'Host agent'} (agent)`
                          : `${hostUsageEntry.count} host agents`,
                      );
                      usageTitleSegments.push(
                        `Host agents: ${hostUsageEntry.hosts.map((host) => host.label).join(', ')}`,
                      );
                    }
                    const hostSummary = usageSegments.length > 0 ? usageSegments.join(' • ') : '—';
                    const rawScopes = token.scopes && token.scopes.length > 0 ? token.scopes : ['*'];
                    const scopeBadges = rawScopes.includes('*')
                      ? [{ value: '*', label: 'Full' }]
                      : rawScopes.map((scope) => ({
                        value: scope,
                        label: API_SCOPE_LABELS[scope] ?? scope,
                      }));
                    const rowIsWildcard = scopeBadges.some((scope) => scope.value === '*');

                    return (
                      <tr
                        class={`transition-colors ${rowIsWildcard
                          ? 'bg-amber-50/50 dark:bg-amber-900/10'
                          : 'bg-white dark:bg-gray-900/10'
                          } hover:bg-blue-50/40 dark:hover:bg-gray-800/50`}
                      >
                        <td class="whitespace-nowrap px-5 py-3 font-medium text-gray-900 dark:text-gray-100">
                          {token.name || 'Untitled'}
                        </td>
                        <td class="px-5 py-3 font-mono text-xs text-gray-600 dark:text-gray-400">
                          {tokenHint(token)}
                        </td>
                        <td class="px-5 py-3">
                          <div class="flex flex-wrap gap-1.5">
                            <For each={scopeBadges}>
                              {(scope) => {
                                const isWildcard = scope.value === '*';
                                return (
                                  <span
                                    class={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${isWildcard
                                      ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200'
                                      : 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300'
                                      }`}
                                    title={scope.value}
                                  >
                                    {scope.label}
                                  </span>
                                );
                              }}
                            </For>
                          </div>
                        </td>
                        <td
                          class="px-5 py-3 text-gray-600 dark:text-gray-400"
                          title={usageTitleSegments.length > 0 ? usageTitleSegments.join('\n') : undefined}
                        >
                          <div class="flex flex-wrap items-center gap-2">
                            <span>{hostSummary}</span>
                            <Show when={hostUsageEntry && hostUsageEntry.count > 1}>
                              <span class="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-900/40 dark:text-amber-200">
                                <svg class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
                                  <path
                                    fill-rule="evenodd"
                                    d="M8.257 3.099c.764-1.36 2.722-1.36 3.486 0l6.518 11.62c.75 1.338-.213 3.005-1.743 3.005H3.482c-1.53 0-2.493-1.667-1.743-3.005l6.518-11.62ZM11 5a1 1 0 1 0-2 0v4.5a1 1 0 1 0 2 0V5Zm0 8a1 1 0 1 0-2 0 1 1 0 0 0 2 0Z"
                                    clip-rule="evenodd"
                                  />
                                </svg>
                                Host agents sharing this token ({hostUsageEntry!.count})
                              </span>
                            </Show>
                          </div>
                        </td>
                        <td class="px-5 py-3 text-gray-600 dark:text-gray-400">
                          {formatRelativeTime(new Date(token.createdAt).getTime())}
                        </td>
                        <td class="px-5 py-3 text-gray-600 dark:text-gray-400">
                          {token.lastUsedAt
                            ? formatRelativeTime(new Date(token.lastUsedAt).getTime())
                            : 'Never'}
                        </td>
                        <td class="px-5 py-3 text-right">
                          <button
                            onClick={() => handleDelete(token)}
                            class="text-sm font-semibold text-red-600 transition hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
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
        </Card>
      </Show>

      <Card
        padding="lg"
        class={`border border-gray-200 dark:border-gray-700 transition-shadow ${createHighlight() ? 'ring-2 ring-blue-500/60 shadow-lg' : ''
          }`}
        ref={(el: HTMLDivElement) => {
          createSectionRef = el;
        }}
      >
        <div class="flex flex-col gap-6">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <SectionHeader
              size="sm"
              title="Create token"
              description="Name the token and choose a scope preset or build a custom set of capabilities."
              class="flex-1"
            />
            <button
              onClick={handleGenerate}
              disabled={isGenerating()}
              class="inline-flex items-center justify-center rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-blue-500 dark:hover:bg-blue-400"
            >
              {isGenerating() ? 'Generating…' : 'Generate'}
            </button>
          </div>

          <div class="space-y-4">
            <div class="space-y-2">
              <label class="text-xs font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-400">
                Token name
              </label>
              <input
                type="text"
                value={nameInput()}
                onInput={(e) => setNameInput(e.currentTarget.value)}
                placeholder="e.g. Container pipeline"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm transition focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:focus:border-blue-400 dark:focus:ring-blue-500/40"
              />
            </div>

            <div class="space-y-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="text-xs font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-400">
                  Quick presets
                </span>
                <button
                  type="button"
                  class="text-xs font-medium text-blue-600 underline-offset-2 hover:underline dark:text-blue-300"
                  onClick={clearScopes}
                  title="Legacy wildcard – all permissions"
                >
                  Clear selection
                </button>
              </div>

              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-semibold transition ${isFullAccessSelected()
                    ? 'border-blue-500 bg-blue-600 text-white shadow-sm'
                    : 'border-gray-300 bg-white text-gray-700 hover:border-blue-400 hover:text-blue-600 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-blue-400 dark:hover:text-blue-200'
                    }`}
                  onClick={clearScopes}
                  title="Legacy wildcard – all permissions"
                >
                  Full access
                </button>

                <For each={scopePresets}>
                  {(preset) => (
                    <button
                      type="button"
                      class={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-semibold transition ${presetMatchesSelection(preset.scopes)
                        ? 'border-blue-500 bg-blue-600 text-white shadow-sm'
                        : 'border-gray-300 bg-white text-gray-700 hover:border-blue-400 hover:text-blue-600 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-blue-400 dark:hover:text-blue-200'
                        }`}
                      onClick={() => applyScopePreset(preset.scopes)}
                      title={preset.description}
                    >
                      {preset.label}
                    </button>
                  )}
                </For>
              </div>
            </div>

            <details class="group rounded-lg border border-gray-200 bg-gray-50/60 p-4 text-sm transition dark:border-gray-700 dark:bg-gray-900/40">
              <summary class="cursor-pointer text-sm font-semibold text-gray-700 transition hover:text-blue-600 dark:text-gray-200 dark:hover:text-blue-300">
                Custom scopes
              </summary>
              <div class="mt-3 space-y-4">
                <For each={scopeGroups()}>
                  {([group, options]) => (
                    <div class="space-y-2">
                      <div class="text-[0.7rem] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        {group}
                      </div>
                      <div class="flex flex-wrap gap-2">
                        <For each={options}>
                          {(option) => {
                            const isActive = () => selectedScopes().includes(option.value);
                            return (
                              <button
                                type="button"
                                class={`rounded-full border px-3 py-1 text-xs font-semibold transition ${isActive()
                                  ? 'border-blue-500 bg-blue-600 text-white shadow-sm'
                                  : 'border-gray-300 bg-white text-gray-700 hover:border-blue-400 hover:text-blue-600 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-blue-400 dark:hover:text-blue-200'
                                  }`}
                                onClick={() => {
                                  setSelectedScopes((prev) => {
                                    if (prev.includes(option.value)) {
                                      return prev.filter((v) => v !== option.value);
                                    }
                                    return [...prev, option.value];
                                  });
                                }}
                                title={option.description}
                              >
                                {option.label}
                              </button>
                            );
                          }}
                        </For>
                      </div>
                    </div>
                  )}
                </For>
              </div>
            </details>
          </div>
        </div>
      </Card>

      <Show when={!loading() && hasWildcardTokens()}>
        <Card
          tone="warning"
          padding="sm"
          class="flex flex-wrap items-center gap-3 border border-amber-300/70 text-sm text-amber-800 dark:border-amber-700/70 dark:text-amber-100"
        >
          ⚠ {wildcardCount()} full access {wildcardCount() === 1 ? 'token' : 'tokens'} – consider switching to scoped presets for least privilege.
        </Card>
      </Show>

      <Card tone="muted" padding="sm" class="text-xs text-gray-600 dark:text-gray-400">
        Separate tokens per integration • Rotate regularly •{' '}
        <a
          class="font-medium text-blue-600 underline-offset-2 hover:underline dark:text-blue-400"
          href={SCOPES_DOC_URL}
          target="_blank"
          rel="noreferrer"
        >
          Scope reference
        </a>
      </Card>
    </div>
  );
};

export default APITokenManager;
