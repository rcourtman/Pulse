import { Component, For, Show, createUniqueId, type Accessor } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { InlineNotice } from '@/components/shared/InlineNotice';
import type { APIScopeOption } from '@/constants/apiScopes';
import type { APITokenRecord } from '@/types/api';
import { getAPITokenLastUsePresentation } from '@/utils/apiTokenPresentation';
import { getAPITokenHint, getAPITokenIdentityLabel } from './apiTokenManagerModel';

interface APITokenManagerDialogsProps {
  tokenToEdit: Accessor<APITokenRecord | null>;
  tokenToRevoke: Accessor<APITokenRecord | null>;
  revokeUsage: string[];
  tokenToRename: Accessor<APITokenRecord | null>;
  renameInput: Accessor<string>;
  editScopes: Accessor<string[]>;
  scopeGroups: Accessor<[APIScopeOption['group'], APIScopeOption[]][]>;
  updatingTokenId: Accessor<string | null>;
  isRenaming: Accessor<boolean>;
  scopeError?: string | null;
  renameError?: string | null;
  editScopesChanged: () => boolean;
  onToggleEditScope: (scope: string) => void;
  onCloseScopeEditor: () => void;
  onSaveEditedScopes: () => void;
  onCancelRevoke: () => void;
  onRevoke: (token: APITokenRecord) => void;
  onRenameInput: (name: string) => void;
  onCloseRename: () => void;
  onRename: () => void;
}

export const APITokenManagerDialogs: Component<APITokenManagerDialogsProps> = (props) => (
  <>
    <Show when={props.tokenToEdit()}>
      <Dialog
        isOpen={true}
        onClose={props.onCloseScopeEditor}
        panelClass="max-w-2xl"
        ariaLabel={`Edit API token scopes for ${getAPITokenIdentityLabel(props.tokenToEdit()!)}`}
      >
        <div
          class={`min-h-0 w-full overflow-y-auto px-4 sm:px-6 ${props.scopeError ? 'space-y-2 py-2 sm:space-y-3 sm:py-4' : 'space-y-3 py-3 sm:space-y-5 sm:py-6'}`}
        >
          <div>
            <h3 class="text-lg font-semibold text-base-content">
              Edit token scopes{' '}
              <span class="ml-2 break-all font-mono text-xs font-normal text-muted">
                {getAPITokenHint(props.tokenToEdit())}
              </span>
            </h3>
            <p class="mt-1 text-sm leading-tight text-muted wrap-anywhere sm:leading-normal">
              <bdi class="block font-medium text-base-content">
                {props.tokenToEdit()!.name || props.tokenToEdit()!.id}
              </bdi>
              Scope changes take effect on the token's next request. The value and expiry do not
              change.
            </p>
          </div>

          <label class="flex cursor-pointer items-start gap-2 rounded-md border border-amber-300 bg-amber-50 p-2 text-sm sm:gap-3 sm:p-3 dark:border-amber-700 dark:bg-amber-900/25">
            <input
              type="checkbox"
              checked={props.editScopes().includes('*')}
              onChange={() => props.onToggleEditScope('*')}
              disabled={props.updatingTokenId() !== null}
              class="mt-0.5 h-4 w-4 rounded-sm border-border text-amber-600 focus:ring-amber-500"
            />
            <span>
              <span class="block font-semibold text-amber-900 dark:text-amber-100">
                Full access
              </span>
              <span class="text-amber-800 dark:text-amber-200">
                Legacy wildcard access to every API capability.
              </span>
            </span>
          </label>

          <div class="max-h-[50vh] space-y-4 overflow-y-auto pr-1">
            <For each={props.scopeGroups()}>
              {([group, options]) => (
                <fieldset class="space-y-2">
                  <legend class="text-[0.7rem] font-semibold uppercase tracking-wide text-muted">
                    {group}
                  </legend>
                  <div class="grid gap-2 sm:grid-cols-2">
                    <For each={options}>
                      {(option) => (
                        <label class="flex cursor-pointer items-start gap-2 rounded-md border border-border p-2 text-sm transition hover:bg-surface-hover sm:gap-3 sm:p-3">
                          <input
                            type="checkbox"
                            checked={props.editScopes().includes(option.value)}
                            onChange={() => props.onToggleEditScope(option.value)}
                            disabled={props.updatingTokenId() !== null}
                            class="mt-0.5 h-4 w-4 rounded-sm border-border text-blue-600 focus:ring-blue-500"
                          />
                          <span>
                            <span class="block font-medium text-base-content">{option.label}</span>
                            <span class="text-xs text-muted">{option.description}</span>
                          </span>
                        </label>
                      )}
                    </For>
                  </div>
                </fieldset>
              )}
            </For>
          </div>
        </div>

        <div class="shrink-0 space-y-1.5 border-t border-border px-4 py-2 sm:space-y-2 sm:px-6 sm:pt-4 sm:pb-6">
          <Show when={props.editScopes().length === 0}>
            <p class="text-sm text-red-600 dark:text-red-300">
              Select at least one scope before saving.
            </p>
          </Show>

          <Show when={props.scopeError}>
            <InlineNotice
              tone="danger"
              role="alert"
              tabIndex={0}
              class="max-h-[25dvh] overflow-y-auto wrap-anywhere focus-visible:outline-2 focus-visible:-outline-offset-2"
            >
              {props.scopeError}
            </InlineNotice>
          </Show>

          <div class="flex flex-wrap justify-end gap-3">
            <button
              type="button"
              onClick={props.onCloseScopeEditor}
              disabled={props.updatingTokenId() !== null}
              class="rounded-md border border-border px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={props.onSaveEditedScopes}
              disabled={
                props.editScopes().length === 0 ||
                !props.editScopesChanged() ||
                props.updatingTokenId() !== null
              }
              class="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {props.updatingTokenId() !== null ? 'Saving…' : 'Save scopes'}
            </button>
          </div>
        </div>
      </Dialog>
    </Show>

    <Show when={props.tokenToRename()}>
      <Dialog
        isOpen={true}
        onClose={props.onCloseRename}
        panelClass="max-w-md"
        ariaLabel={`Rename API token for ${getAPITokenIdentityLabel(props.tokenToRename()!)}`}
      >
        <form
          class="w-full p-6"
          onSubmit={(event) => {
            event.preventDefault();
            props.onRename();
          }}
        >
          <h3 class="mb-2 text-lg font-semibold text-base-content">
            Rename API token{' '}
            <span class="ml-2 break-all font-mono text-xs font-normal text-muted">
              {getAPITokenHint(props.tokenToRename())}
            </span>
          </h3>
          <p class="mb-4 text-sm text-muted">
            This only changes the name shown in Pulse. The token value and connected agents stay
            unchanged.
          </p>
          <label class="block space-y-2">
            <span class="text-xs font-semibold uppercase tracking-wide text-muted">Token name</span>
            <input
              type="text"
              value={props.renameInput()}
              onInput={(event) => props.onRenameInput(event.currentTarget.value)}
              disabled={props.isRenaming()}
              class="w-full min-h-10 rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-200"
              autofocus
            />
          </label>
          <Show when={props.renameError}>
            <InlineNotice tone="danger" role="alert" class="mt-4">
              {props.renameError}
            </InlineNotice>
          </Show>
          <div class="mt-5 flex justify-end gap-3">
            <button
              type="button"
              onClick={props.onCloseRename}
              disabled={props.isRenaming()}
              class="rounded-md border border-border px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={props.isRenaming() || !props.renameInput().trim()}
              class="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {props.isRenaming() ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </Dialog>
    </Show>

    <Show keyed when={props.tokenToRevoke()}>
      {(token) => {
        const warningId = createUniqueId();
        const usageId = createUniqueId();
        const lastUse = getAPITokenLastUsePresentation(token.lastUsedAt);
        return (
          <Dialog
            isOpen={true}
            onClose={props.onCancelRevoke}
            panelClass="max-w-md"
            ariaLabel={`Revoke API token for ${getAPITokenIdentityLabel(token)}`}
            ariaDescribedBy={`${warningId} ${usageId}`}
          >
            <div
              role="region"
              aria-label="Token and revocation details"
              tabIndex={0}
              class="min-h-0 w-full overflow-y-auto px-4 pt-3 focus-visible:outline-2 focus-visible:-outline-offset-2 sm:px-6 sm:pt-6"
            >
              <h3 class="text-lg leading-tight font-semibold text-base-content">
                Revoke API token?{' '}
                <span class="ml-2 break-all font-mono text-xs font-normal text-muted">
                  {getAPITokenHint(token)}
                </span>
              </h3>
              <bdi class="mt-1 block text-sm leading-tight font-medium text-base-content wrap-anywhere">
                {token.name || token.id}
              </bdi>
              <p
                id={warningId}
                class="mt-1 text-sm leading-tight text-muted sm:mt-2 sm:leading-normal"
              >
                Revocation is permanent. Agents, scripts, and integrations using it need a
                replacement token to authenticate.
              </p>
              <div id={usageId} class="mt-2 space-y-1 text-xs text-muted wrap-anywhere">
                <p>
                  <span class="font-semibold text-base-content">Last recorded use:</span>{' '}
                  {lastUse.state === 'recorded' ? (
                    <time dateTime={lastUse.dateTime}>{lastUse.timestamp}</time>
                  ) : (
                    lastUse.label
                  )}
                </p>
                <Show when={props.revokeUsage.length > 0}>
                  <p class="font-semibold text-base-content">Previously seen using this token</p>
                  <ul class="list-disc pl-4">
                    <For each={props.revokeUsage}>
                      {(group) => (
                        <li>
                          <bdi>{group}</bdi>
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
                <p>Pulse may not know about every client using this token.</p>
              </div>
            </div>
            <div class="flex shrink-0 flex-wrap justify-end gap-3 px-4 pt-2 pb-3 sm:px-6 sm:pt-4 sm:pb-6">
              <button
                type="button"
                onClick={props.onCancelRevoke}
                class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md hover:bg-surface-hover"
                autofocus
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => {
                  const current = props.tokenToRevoke();
                  if (current) props.onRevoke(current);
                }}
                class="px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700"
              >
                Revoke token
              </button>
            </div>
          </Dialog>
        );
      }}
    </Show>
  </>
);
