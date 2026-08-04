import { Component, For, Show, type Accessor } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import type { APIScopeOption } from '@/constants/apiScopes';
import type { APITokenRecord } from '@/types/api';

interface APITokenManagerDialogsProps {
  tokenToEdit: Accessor<APITokenRecord | null>;
  tokenToRevoke: Accessor<APITokenRecord | null>;
  editScopes: Accessor<string[]>;
  scopeGroups: Accessor<[APIScopeOption['group'], APIScopeOption[]][]>;
  updatingTokenId: Accessor<string | null>;
  editScopesChanged: () => boolean;
  onToggleEditScope: (scope: string) => void;
  onCloseScopeEditor: () => void;
  onSaveEditedScopes: () => void;
  onCancelRevoke: () => void;
  onRevoke: (token: APITokenRecord) => void;
}

export const APITokenManagerDialogs: Component<APITokenManagerDialogsProps> = (props) => (
  <>
    <Show when={props.tokenToEdit()}>
      <Dialog
        isOpen={true}
        onClose={props.onCloseScopeEditor}
        panelClass="max-w-2xl"
        ariaLabel="Edit API token scopes"
      >
        <div class="w-full space-y-5 p-6">
          <div>
            <h3 class="text-lg font-semibold text-base-content">Edit token scopes</h3>
            <p class="mt-1 text-sm text-muted">
              Changes to{' '}
              <span class="font-medium text-base-content">
                {props.tokenToEdit()!.name || props.tokenToEdit()!.id}
              </span>{' '}
              take effect on its next request. The token value and expiry do not change.
            </p>
          </div>

          <label class="flex cursor-pointer items-start gap-3 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm dark:border-amber-700 dark:bg-amber-900">
            <input
              type="checkbox"
              checked={props.editScopes().includes('*')}
              onChange={() => props.onToggleEditScope('*')}
              disabled={props.updatingTokenId() !== null}
              class="mt-0.5 h-4 w-4 rounded border-border text-amber-600 focus:ring-amber-500"
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
                        <label class="flex cursor-pointer items-start gap-3 rounded-md border border-border p-3 text-sm transition hover:bg-surface-hover">
                          <input
                            type="checkbox"
                            checked={props.editScopes().includes(option.value)}
                            onChange={() => props.onToggleEditScope(option.value)}
                            disabled={props.updatingTokenId() !== null}
                            class="mt-0.5 h-4 w-4 rounded border-border text-blue-600 focus:ring-blue-500"
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

          <Show when={props.editScopes().length === 0}>
            <p class="text-sm text-red-600 dark:text-red-300">
              Select at least one scope before saving.
            </p>
          </Show>

          <div class="flex justify-end gap-3 border-t border-border pt-4">
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

    <Show when={props.tokenToRevoke()}>
      <Dialog
        isOpen={true}
        onClose={props.onCancelRevoke}
        panelClass="max-w-md"
        ariaLabel="Revoke API token"
      >
        <div class="w-full p-6">
          <h3 class="text-lg font-semibold text-base-content mb-2">Revoke API token?</h3>
          <p class="text-sm text-muted mb-4">
            This permanently revokes{' '}
            <span class="font-medium text-base-content">
              {props.tokenToRevoke()!.name || props.tokenToRevoke()!.id}
            </span>
            . Any agents, scripts, or integrations using it will stop authenticating until you issue
            and configure a replacement token.
          </p>
          <div class="flex justify-end gap-3">
            <button
              type="button"
              onClick={props.onCancelRevoke}
              class="px-4 py-2 text-sm font-medium text-base-content border border-border rounded-md hover:bg-surface-hover"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() => {
                const token = props.tokenToRevoke();
                if (token) props.onRevoke(token);
              }}
              class="px-4 py-2 text-sm font-medium bg-red-600 text-white rounded-md hover:bg-red-700"
            >
              Revoke token
            </button>
          </div>
        </div>
      </Dialog>
    </Show>
  </>
);
