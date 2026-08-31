import { Component, For, Show, createSignal, lazy } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { SelectablePillButton } from '@/components/shared/SelectablePillButton';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import { MONITORING_READ_SCOPE } from '@/constants/apiScopes';
import {
  API_TOKEN_NAME_PLACEHOLDER,
  getAPITokenDockerPodmanUsageSummary,
  getAPITokenDockerPodmanUsageTitle,
} from '@/utils/apiTokenPresentation';
import { API_TOKEN_CREATE_ANCHOR } from '@/routing/resourceLinks';
import { useAPITokenManagerState } from './useAPITokenManagerState';
import type { APITokenRecord } from '@/types/api';

interface APITokenManagerProps {
  currentTokenHint?: string;
  onTokensChanged?: () => void;
  refreshing?: boolean;
  canManage?: boolean;
}

const APITokenManagerDialogs = lazy(() =>
  import('./APITokenManagerDialogs').then((module) => ({
    default: module.APITokenManagerDialogs,
  })),
);

export const APITokenManager: Component<APITokenManagerProps> = (props) => {
  const [tokenToRevoke, setTokenToRevoke] = createSignal<APITokenRecord | null>(null);
  const [tokenToEdit, setTokenToEdit] = createSignal<APITokenRecord | null>(null);
  const [editScopes, setEditScopes] = createSignal<string[]>([]);
  const [tokenToRename, setTokenToRename] = createSignal<APITokenRecord | null>(null);
  const [renameInput, setRenameInput] = createSignal('');
  let dialogTrigger: HTMLButtonElement | undefined;
  const {
    API_SCOPE_LABELS,
    API_TOKEN_SCOPES_DOC_URL,
    agentTokenUsage,
    applyFullAccessPreset,
    applyScopePreset,
    canManage,
    canGenerateToken,
    clearScopes,
    copyNewMonitoringKioskLink,
    createHighlight,
    dismissNewToken,
    dockerTokenUsage,
    focusCreateSection,
    formatRelativeTime,
    handleDelete,
    handleGenerate,
    handleUpdateScopes,
    handleRename,
    hasWildcardTokens,
    hasScopeSelection,
    isFullAccessSelected,
    isGenerating,
    isRenaming,
    isRevealActiveForCurrentToken,
    loading,
    nameInput,
    newMonitoringKioskLink,
    newTokenRecord,
    newTokenValue,
    reopenTokenDialog,
    scopedTokenCount,
    scopeGroups,
    scopePresets,
    selectedScopes,
    setCreateSectionRef,
    setNameInput,
    sortedTokens,
    tokenHint,
    toggleScope,
    totalTokens,
    updatingTokenId,
    wildcardCount,
    presetMatchesSelection,
  } = useAPITokenManagerState(props);

  const effectiveScopes = (token: APITokenRecord) =>
    token.scopes && token.scopes.length > 0 ? token.scopes : ['*'];

  const rememberDialogTrigger = (trigger: HTMLButtonElement) => {
    dialogTrigger = trigger;
  };

  const restoreDialogTrigger = () => {
    const trigger = dialogTrigger;
    const tokenId = trigger?.dataset.apiTokenId;
    const action = trigger?.dataset.apiTokenAction;
    dialogTrigger = undefined;
    window.setTimeout(() => {
      if (trigger?.isConnected && !trigger.disabled) {
        trigger.focus();
        return;
      }
      if (!tokenId || !action) return;
      const replacement = Array.from(
        document.querySelectorAll<HTMLButtonElement>(
          'button[data-api-token-id][data-api-token-action]',
        ),
      ).find(
        (button) =>
          button.dataset.apiTokenId === tokenId &&
          button.dataset.apiTokenAction === action &&
          button.offsetParent !== null &&
          !button.disabled,
      );
      replacement?.focus();
    }, 0);
  };

  const openScopeEditor = (token: APITokenRecord, trigger: HTMLButtonElement) => {
    rememberDialogTrigger(trigger);
    setEditScopes([...effectiveScopes(token)]);
    setTokenToEdit(token);
  };

  const openRevokeDialog = (token: APITokenRecord, trigger: HTMLButtonElement) => {
    rememberDialogTrigger(trigger);
    setTokenToRevoke(token);
  };

  const openRenameDialog = (token: APITokenRecord, trigger: HTMLButtonElement) => {
    rememberDialogTrigger(trigger);
    setRenameInput(token.name ?? '');
    setTokenToRename(token);
  };

  const closeScopeEditor = () => {
    if (updatingTokenId() !== null) return;
    setTokenToEdit(null);
    setEditScopes([]);
    restoreDialogTrigger();
  };

  const toggleEditScope = (scope: string) => {
    setEditScopes((previous) => {
      if (scope === '*') {
        return previous.includes('*') ? [] : ['*'];
      }
      const scoped = previous.filter((value) => value !== '*');
      if (scoped.includes(scope)) {
        return scoped.filter((value) => value !== scope);
      }
      return [...scoped, scope];
    });
  };

  const scopeSelectionKey = (scopes: string[]) => Array.from(new Set(scopes)).sort().join('\u0000');

  const editScopesChanged = () => {
    const token = tokenToEdit();
    return token
      ? scopeSelectionKey(editScopes()) !== scopeSelectionKey(effectiveScopes(token))
      : false;
  };

  const saveEditedScopes = async () => {
    const token = tokenToEdit();
    if (!token || editScopes().length === 0 || !editScopesChanged()) return;
    if (await handleUpdateScopes(token, editScopes())) {
      setTokenToEdit(null);
      setEditScopes([]);
      restoreDialogTrigger();
    }
  };

  const revokeToken = (token: APITokenRecord) => {
    setTokenToRevoke(null);
    restoreDialogTrigger();
    void handleDelete(token);
  };

  const cancelRevoke = () => {
    setTokenToRevoke(null);
    restoreDialogTrigger();
  };

  const closeRenameDialog = () => {
    if (isRenaming()) return;
    setTokenToRename(null);
    setRenameInput('');
    restoreDialogTrigger();
  };

  const renameToken = async () => {
    const token = tokenToRename();
    if (!token) return;
    if (await handleRename(token, renameInput())) {
      setTokenToRename(null);
      setRenameInput('');
      restoreDialogTrigger();
    }
  };

  const renderTokenScopes = (token: APITokenRecord) => {
    const rawScopes = effectiveScopes(token);
    const scopeBadges = rawScopes.includes('*')
      ? [{ value: '*', label: 'Full' }]
      : rawScopes.map((scope) => ({
          value: scope,
          label: API_SCOPE_LABELS[scope] ?? scope,
        }));

    return (
      <div class="flex flex-wrap gap-1.5">
        <For each={scopeBadges}>
          {(scope) => {
            const isWildcard = scope.value === '*';
            return (
              <span
                class={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  isWildcard
                    ? 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200'
                    : 'bg-surface-alt text-base-content'
                }`}
                title={scope.value}
              >
                {scope.label}
              </span>
            );
          }}
        </For>
      </div>
    );
  };

  const renderTokenUsage = (token: APITokenRecord) => {
    const dockerUsageEntry = dockerTokenUsage().get(token.id);
    const agentUsageEntry = agentTokenUsage().get(token.id);
    const usageSegments: string[] = [];
    const usageTitleSegments: string[] = [];
    if (dockerUsageEntry) {
      usageSegments.push(getAPITokenDockerPodmanUsageSummary(dockerUsageEntry));
      usageTitleSegments.push(getAPITokenDockerPodmanUsageTitle(dockerUsageEntry));
    }
    if (agentUsageEntry) {
      usageSegments.push(
        agentUsageEntry.count === 1
          ? `${agentUsageEntry.items[0]?.label ?? 'Agent'}`
          : `${agentUsageEntry.count} agents`,
      );
      usageTitleSegments.push(
        `Agents: ${agentUsageEntry.items.map((agent) => agent.label).join(', ')}`,
      );
    }
    const usageSummary = usageSegments.length > 0 ? usageSegments.join(' • ') : '—';

    return (
      <div
        class="flex flex-wrap items-center gap-2"
        title={usageTitleSegments.length > 0 ? usageTitleSegments.join('\n') : undefined}
      >
        <span class="text-muted">{usageSummary}</span>
        <Show when={agentUsageEntry && agentUsageEntry.count > 1}>
          <span class="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-900 dark:text-amber-200">
            <svg class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
              <path
                fill-rule="evenodd"
                d="M8.257 3.099c.764-1.36 2.722-1.36 3.486 0l6.518 11.62c.75 1.338-.213 3.005-1.743 3.005H3.482c-1.53 0-2.493-1.667-1.743-3.005l6.518-11.62ZM11 5a1 1 0 1 0-2 0v4.5a1 1 0 1 0 2 0V5Zm0 8a1 1 0 1 0-2 0 1 1 0 0 0 2 0Z"
                clip-rule="evenodd"
              />
            </svg>
            Agents sharing this token ({agentUsageEntry!.count})
          </span>
        </Show>
      </div>
    );
  };

  const renderTokenActions = (token: APITokenRecord, compact = false) => (
    <div
      class={
        compact
          ? 'flex w-full items-center gap-2 border-t border-border pt-3'
          : 'flex items-center justify-end gap-1'
      }
    >
      <button
        type="button"
        data-api-token-id={token.id}
        data-api-token-action="edit-scopes"
        onClick={(event) => openScopeEditor(token, event.currentTarget)}
        disabled={!canManage()}
        class={`inline-flex min-h-10 items-center justify-center rounded-md px-2.5 py-1.5 text-sm font-semibold text-blue-600 transition hover:bg-blue-50 hover:text-blue-700 disabled:cursor-not-allowed disabled:opacity-60 dark:text-blue-300 dark:hover:bg-blue-900 dark:hover:text-blue-200 ${
          compact ? 'flex-1 border border-blue-200 dark:border-blue-800' : 'sm:min-h-9'
        }`}
      >
        Edit scopes
      </button>
      <button
        type="button"
        data-api-token-id={token.id}
        data-api-token-action="rename"
        onClick={(event) => openRenameDialog(token, event.currentTarget)}
        disabled={!canManage()}
        class={`inline-flex min-h-10 items-center justify-center rounded-md px-2.5 py-1.5 text-sm font-semibold text-blue-600 transition hover:bg-blue-50 hover:text-blue-700 disabled:cursor-not-allowed disabled:opacity-60 dark:text-blue-300 dark:hover:bg-blue-900 dark:hover:text-blue-200 ${
          compact ? 'flex-1 border border-blue-200 dark:border-blue-800' : 'sm:min-h-9'
        }`}
      >
        Rename
      </button>
      <button
        type="button"
        data-api-token-id={token.id}
        data-api-token-action="revoke"
        onClick={(event) => openRevokeDialog(token, event.currentTarget)}
        disabled={!canManage()}
        class={`inline-flex min-h-10 items-center justify-center rounded-md px-2.5 py-1.5 text-sm font-semibold text-red-600 transition hover:bg-red-50 hover:text-red-700 disabled:cursor-not-allowed disabled:opacity-60 dark:text-red-400 dark:hover:bg-red-900 dark:hover:text-red-300 ${
          compact ? 'flex-1 border border-red-200 dark:border-red-900' : 'sm:min-h-9'
        }`}
      >
        Revoke
      </button>
    </div>
  );

  return (
    <div class="space-y-5">
      <Card padding="none" class="border border-border shadow-sm">
        <div class="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div class="flex flex-wrap items-center gap-3">
              <div class="flex h-10 w-10 items-center justify-center rounded-md bg-blue-600 text-blue-600 dark:bg-blue-500 dark:text-blue-200">
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
              disabled={!canManage()}
              class="inline-flex min-h-10 sm:min-h-10 items-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-white"
            >
              <svg
                class="h-4 w-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
              </svg>
              New token
            </button>
          </div>

          <Show when={!canManage()}>
            <Card
              tone="info"
              padding="sm"
              class="border border-blue-200 text-xs text-blue-800 dark:border-blue-800 dark:text-blue-200"
            >
              Token management is read-only for this account. You can review existing tokens but
              cannot create or revoke them.
            </Card>
          </Show>

          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <div class="rounded-md border border-border p-4 text-sm shadow-sm">
              <div class="text-[0.7rem] font-semibold uppercase tracking-wide text-muted">
                Total tokens
              </div>
              <div class="mt-1 text-2xl font-semibold text-base-content">{totalTokens()}</div>
              <p class="mt-1 text-xs text-muted">Stored credentials across all agents</p>
            </div>
            <div class="rounded-md border border-border p-4 text-sm shadow-sm">
              <div class="text-[0.7rem] font-semibold uppercase tracking-wide text-muted">
                Scoped tokens
              </div>
              <div class="mt-1 text-2xl font-semibold text-base-content">{scopedTokenCount()}</div>
              <p class="mt-1 text-xs text-muted">Limited access tokens with defined scopes</p>
            </div>
            <div
              class={`rounded-md border p-4 text-sm shadow-sm ${hasWildcardTokens() ? 'border-amber-300 bg-amber-50 text-amber-900 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-100' : 'border-border bg-surface text-base-content'}`}
            >
              <div
                class={`text-[0.7rem] font-semibold uppercase tracking-wide ${
                  hasWildcardTokens() ? 'text-amber-700 dark:text-amber-300' : 'text-muted'
                }`}
              >
                Full access tokens
              </div>
              <div
                class={`mt-1 text-2xl font-semibold ${
                  hasWildcardTokens() ? 'text-amber-800 dark:text-amber-100' : 'text-base-content'
                }`}
              >
                {wildcardCount()}
              </div>
              <p
                class={`mt-1 text-xs ${
                  hasWildcardTokens() ? 'text-amber-700 dark:text-amber-200' : 'text-muted'
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
          class="flex items-center gap-2 border border-blue-200 text-xs text-blue-800 dark:border-blue-800 dark:text-blue-200"
        >
          <LoadingSpinner size="md" tone="info" />
          <span>Refreshing security status…</span>
        </Card>
      </Show>

      <Show when={newTokenValue() && !isRevealActiveForCurrentToken()}>
        <div class="space-y-3">
          <Card
            tone="success"
            padding="sm"
            class="flex flex-wrap items-center justify-between gap-3 border border-green-300 text-sm text-green-800 dark:border-green-700 dark:text-green-200"
          >
            <span>
              ✓ Token generated: <strong>{newTokenRecord()?.name || 'Untitled'}</strong> (
              {tokenHint(newTokenRecord())})
            </span>
            <div class="flex items-center gap-3 text-xs">
              <button
                onClick={reopenTokenDialog}
                disabled={!canManage()}
                class="font-medium underline decoration-green-500 underline-offset-2 hover:text-green-900 dark:hover:text-green-100"
              >
                Show
              </button>
              <button
                onClick={() => {
                  dismissNewToken();
                }}
                class="font-medium underline decoration-green-500 underline-offset-2 hover:text-green-900 dark:hover:text-green-100"
              >
                Dismiss
              </button>
            </div>
          </Card>

          <Show
            when={
              newTokenRecord()?.scopes?.length === 1 &&
              newTokenRecord()?.scopes?.[0] === MONITORING_READ_SCOPE
            }
          >
            <div class="rounded-md border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900 shadow-sm dark:border-blue-800 dark:bg-blue-900 dark:text-blue-100">
              <div class="mb-2 font-semibold">Magic Kiosk Link</div>
              <p class="mb-3 text-xs text-blue-700 dark:text-blue-300">
                Use this link to open Pulse directly in Kiosk mode without logging in. Perfect for
                wall displays and digital signage.
              </p>
              <div class="flex items-center gap-2">
                <code class="flex-1 rounded border border-blue-200 bg-surface px-3 py-2 font-mono text-xs text-blue-800 dark:border-blue-800 dark:bg-black dark:text-blue-200 break-all">
                  {newMonitoringKioskLink()}
                </code>
                <button
                  type="button"
                  onClick={() => void copyNewMonitoringKioskLink()}
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
            class="border border-dashed border-border text-sm text-muted"
          >
            No tokens yet.{' '}
            <button
              onClick={focusCreateSection}
              disabled={!canManage()}
              class="font-medium text-blue-600 underline-offset-2 hover:underline dark:text-blue-400"
            >
              Create one
            </button>{' '}
            to authenticate agents and integrations.
          </Card>
        }
      >
        <Card padding="none" tone="card" class="overflow-hidden">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-surface-hover px-5 py-4">
            <div>
              <h4 class="text-sm font-semibold text-base-content">Token inventory</h4>
              <p class="text-xs text-muted">
                Active credentials sorted by most recent creation date.
              </p>
            </div>
            <button
              type="button"
              onClick={focusCreateSection}
              disabled={!canManage()}
              class="inline-flex min-h-10 sm:min-h-10 items-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-600 transition hover:bg-blue-50 dark:border-blue-700 dark:text-blue-200 dark:hover:bg-blue-900"
            >
              Generate new
            </button>
          </div>

          <div
            data-testid="api-token-compact-list"
            class="grid gap-px bg-border xl:grid-cols-2 min-[1600px]:hidden"
          >
            <For each={sortedTokens()}>
              {(token) => (
                <article
                  aria-label={`API token ${token.name || 'Untitled'}`}
                  class="space-y-3 bg-surface p-4 sm:p-5"
                  data-api-token-card={token.id}
                >
                  <div class="flex min-w-0 items-start justify-between gap-3">
                    <div class="min-w-0">
                      <h5 class="break-words text-sm font-semibold text-base-content">
                        {token.name || 'Untitled'}
                      </h5>
                      <div class="mt-1 font-mono text-xs text-muted">{tokenHint(token)}</div>
                    </div>
                    <div class="shrink-0 text-right">
                      <div class="text-[0.65rem] font-semibold uppercase tracking-wide text-muted">
                        Last used
                      </div>
                      <div class="mt-1 text-xs text-base-content">
                        {token.lastUsedAt
                          ? formatRelativeTime(new Date(token.lastUsedAt).getTime())
                          : 'Never'}
                      </div>
                    </div>
                  </div>

                  <dl class="grid gap-3 text-xs sm:grid-cols-2">
                    <div class="space-y-1 sm:col-span-2">
                      <dt class="font-semibold uppercase tracking-wide text-muted">Scopes</dt>
                      <dd>{renderTokenScopes(token)}</dd>
                    </div>
                    <div class="space-y-1">
                      <dt class="font-semibold uppercase tracking-wide text-muted">Usage</dt>
                      <dd>{renderTokenUsage(token)}</dd>
                    </div>
                    <div class="space-y-1">
                      <dt class="font-semibold uppercase tracking-wide text-muted">Created</dt>
                      <dd class="text-base-content">
                        {formatRelativeTime(new Date(token.createdAt).getTime())}
                      </dd>
                    </div>
                  </dl>

                  {renderTokenActions(token, true)}
                </article>
              )}
            </For>
          </div>

          <PulseDataGrid
            class="hidden min-[1600px]:block"
            data={sortedTokens()}
            columns={[
              {
                key: 'name',
                label: 'Name',
                render: (token) => (
                  <span class="font-medium text-base-content">{token.name || 'Untitled'}</span>
                ),
              },
              {
                key: 'hint',
                label: 'Hint',
                render: (token) => (
                  <span class="font-mono text-xs text-muted">{tokenHint(token)}</span>
                ),
              },
              {
                key: 'scopes',
                label: 'Scopes',
                render: renderTokenScopes,
              },
              {
                key: 'usage',
                label: 'Usage',
                render: renderTokenUsage,
              },
              {
                key: 'createdAt',
                label: 'Created',
                render: (token) => (
                  <span class="text-muted">
                    {formatRelativeTime(new Date(token.createdAt).getTime())}
                  </span>
                ),
              },
              {
                key: 'lastUsedAt',
                label: 'Last used',
                render: (token) => (
                  <span class="text-muted">
                    {token.lastUsedAt
                      ? formatRelativeTime(new Date(token.lastUsedAt).getTime())
                      : 'Never'}
                  </span>
                ),
              },
              {
                key: 'action',
                label: 'Actions',
                align: 'right',
                render: renderTokenActions,
              },
            ]}
            keyExtractor={(token) => token.id}
            desktopMinWidth="960px"
            frame="flush"
          />
        </Card>
      </Show>

      <Card
        id={API_TOKEN_CREATE_ANCHOR}
        padding="none"
        class={`border border-border transition-shadow ${
          createHighlight() ? 'ring-2 ring-blue-500 shadow-sm' : ''
        }`}
        ref={setCreateSectionRef}
      >
        <div class="flex flex-col gap-6 p-4 sm:p-6 lg:p-8">
          <div class="flex flex-col items-stretch gap-4 sm:flex-row sm:items-start sm:justify-between">
            <SectionHeader
              size="sm"
              title="Create token"
              description="Name the token and choose a scope preset or build a custom set of capabilities."
              class="flex-1"
            />
            <button
              onClick={handleGenerate}
              disabled={!canGenerateToken()}
              title={
                hasScopeSelection()
                  ? undefined
                  : 'Choose a scope preset or custom scope before generating a token.'
              }
              class="inline-flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60 sm:min-h-10 sm:w-auto dark:bg-blue-500 dark:hover:bg-blue-400"
            >
              {isGenerating() ? 'Generating…' : 'Generate'}
            </button>
          </div>

          <div class="space-y-4">
            <div class="space-y-2">
              <label
                for="api-token-name"
                class="text-xs font-semibold uppercase tracking-wide text-muted"
              >
                Token name
              </label>
              <input
                id="api-token-name"
                type="text"
                value={nameInput()}
                onInput={(e) => setNameInput(e.currentTarget.value)}
                placeholder={API_TOKEN_NAME_PLACEHOLDER}
                disabled={!canManage()}
                class="w-full min-h-10 sm:min-h-10 rounded-md border border-border bg-surface px-3 py-2.5 text-sm text-base-content shadow-sm transition focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-500"
              />
            </div>

            <div class="space-y-3">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Quick presets
                </span>
                <button
                  type="button"
                  class="inline-flex min-h-10 sm:min-h-10 items-center rounded-md px-2.5 py-2 text-sm font-medium text-blue-600 underline-offset-2 hover:underline dark:text-blue-300"
                  onClick={clearScopes}
                  disabled={!canManage()}
                  title="Legacy wildcard – all permissions"
                >
                  Clear selection
                </button>
              </div>

              <div class="flex flex-wrap gap-2">
                <SelectablePillButton
                  active={isFullAccessSelected()}
                  onClick={applyFullAccessPreset}
                  disabled={!canManage()}
                  title="Legacy wildcard – all permissions"
                >
                  Full access
                </SelectablePillButton>

                <For each={scopePresets()}>
                  {(preset) => (
                    <SelectablePillButton
                      active={presetMatchesSelection(preset.scopes)}
                      onClick={() => applyScopePreset(preset.scopes)}
                      disabled={!canManage()}
                      title={preset.description}
                    >
                      {preset.label}
                    </SelectablePillButton>
                  )}
                </For>
              </div>
              <Show when={!hasScopeSelection()}>
                <p class="text-xs text-muted">
                  Choose a scoped preset for least privilege, or deliberately choose Full access.
                </p>
              </Show>
            </div>

            <details class="group rounded-md border border-border bg-surface-hover p-4 text-sm transition">
              <summary class="min-h-10 sm:min-h-10 cursor-pointer text-sm font-semibold text-base-content transition hover:text-blue-600 dark:hover:text-blue-300">
                Custom scopes
              </summary>
              <div class="mt-3 space-y-4">
                <For each={scopeGroups()}>
                  {([group, options]) => (
                    <div class="space-y-2">
                      <div class="text-[0.7rem] font-semibold uppercase tracking-wide text-muted">
                        {group}
                      </div>
                      <div class="flex flex-wrap gap-2">
                        <For each={options}>
                          {(option) => {
                            const isActive = () => selectedScopes().includes(option.value);
                            return (
                              <SelectablePillButton
                                active={isActive()}
                                onClick={() => toggleScope(option.value)}
                                disabled={!canManage()}
                                title={option.description}
                              >
                                {option.label}
                              </SelectablePillButton>
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
          class="flex flex-wrap items-center gap-3 border border-amber-300 text-sm text-amber-800 dark:border-amber-700 dark:text-amber-100"
        >
          ⚠ {wildcardCount()} full access {wildcardCount() === 1 ? 'token' : 'tokens'} – consider
          switching to scoped presets for least privilege.
        </Card>
      </Show>

      <Card tone="muted" padding="sm" class="text-xs text-muted">
        Separate tokens per integration • Rotate regularly •{' '}
        <ExternalTextLink
          href={API_TOKEN_SCOPES_DOC_URL}
          variant="compact"
          class="px-2 py-1.5 font-medium"
        >
          Scope reference
        </ExternalTextLink>
      </Card>

      <Show when={tokenToEdit() || tokenToRevoke() || tokenToRename()}>
        <APITokenManagerDialogs
          tokenToEdit={tokenToEdit}
          tokenToRevoke={tokenToRevoke}
          tokenToRename={tokenToRename}
          renameInput={renameInput}
          editScopes={editScopes}
          scopeGroups={scopeGroups}
          updatingTokenId={updatingTokenId}
          isRenaming={isRenaming}
          editScopesChanged={editScopesChanged}
          onToggleEditScope={toggleEditScope}
          onCloseScopeEditor={closeScopeEditor}
          onSaveEditedScopes={() => void saveEditedScopes()}
          onCancelRevoke={cancelRevoke}
          onRevoke={revokeToken}
          onRenameInput={setRenameInput}
          onCloseRename={closeRenameDialog}
          onRename={() => void renameToken()}
        />
      </Show>
    </div>
  );
};

export default APITokenManager;
