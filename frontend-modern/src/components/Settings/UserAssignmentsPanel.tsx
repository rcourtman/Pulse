import { Component, For, Show, createEffect, on, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACFeatureGateSection } from './RBACFeatureGateSection';
import {
  RoleAccessIdentity,
  UserAccessIdentity,
  UserAssignmentsDialog,
} from './UserAssignmentsDialog';
import { useUserAssignmentsPanelState } from './useUserAssignmentsPanelState';
import type { UserAssignmentsPanelProps } from './useUserAssignmentsPanelState';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import Users from 'lucide-solid/icons/users';
import Shield from 'lucide-solid/icons/shield';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import { SearchField } from '@/components/shared/SearchField';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import {
  getUserAssignmentsEmptyStateCopy,
  getUserIdentityLabel,
  USER_ACCESS_REMOVAL_COPY,
} from '@/utils/rbacPresentation';
import { InlineNotice } from '@/components/shared/InlineNotice';
import TriangleAlert from 'lucide-solid/icons/triangle-alert';
import { Dialog } from '@/components/shared/Dialog';

export const UserAssignmentsPanel: Component<UserAssignmentsPanelProps> = (props) => {
  const state = useUserAssignmentsPanelState(props);
  const emptyStateCopy = () => getUserAssignmentsEmptyStateCopy();
  const inventoryReady = () =>
    !state.loading() && state.accessAllowed() && !state.loadError() && !state.unconfirmedChange();
  let searchInputRef: HTMLInputElement | undefined;
  let inventoryNotice: HTMLDivElement | undefined;
  let loadFailureNotice: HTMLDivElement | undefined;
  let editorFailureNotice: HTMLDivElement | undefined;
  let panel: HTMLDivElement | undefined;
  let savedUser: { reader: object; username?: string } | undefined;
  let focusTimer: number | undefined;
  createEffect(
    on(state.unconfirmedChange, (change) => {
      window.clearTimeout(focusTimer);
      if (!change) return;
      // The editor/confirmation has withdrawn; keep keyboard recovery local.
      focusTimer = window.setTimeout(() => {
        if (state.unconfirmedChange() === change) inventoryNotice?.focus();
      }, 0);
    }),
  );
  onCleanup(() => window.clearTimeout(focusTimer));

  const restartRead = () => {
    window.clearTimeout(focusTimer);
    panel?.focus({ preventScroll: true });
    void state.restartRead();
  };

  const saveOrReload = async (reload = false) => {
    const reader = state.readerKey();
    const username = reload
      ? savedUser?.reader === reader
        ? savedUser.username
        : undefined
      : state.editingUser()?.username;
    if (reload) {
      // The recovery action disappears during the read. Keep its place in the
      // inventory, and don't reuse an older save target for an uncertain write.
      window.clearTimeout(focusTimer);
      panel?.focus({ preventScroll: true });
      if (state.unconfirmedChange()) return state.loadData();
    } else if (!username) return;
    await (reload ? state.loadData() : state.handleSaveAssignments());
    // The state owner already replaces this identity on every reader change.
    // In particular, an organization round trip cannot revive a return target.
    if (reader !== state.readerKey() || state.unconfirmedChange()) return;
    if (!state.showModal()) savedUser = { reader, username };
    window.clearTimeout(focusTimer);
    focusTimer = window.setTimeout(() => {
      if (
        reader !== state.readerKey() ||
        !state.accessAllowed() ||
        state.loading() ||
        state.userPendingDeletion() ||
        state.unconfirmedChange() ||
        document.activeElement !== (reload ? panel : document.body)
      )
        return;
      // A saved read replaces the original row; match the literal principal,
      // not a mutable name or a selector interpolated from an opaque SSO ID.
      let target: HTMLElement | undefined;
      if (state.showModal()) {
        if (state.editingUser()?.username === username && state.actionError())
          target = editorFailureNotice;
      } else if (state.loadError()) {
        target = loadFailureNotice;
      } else if (state.canEdit()) {
        target =
          Array.from(
            panel?.querySelectorAll<HTMLButtonElement>('button[data-user-access]') ?? [],
          ).find((button) => button.dataset.userAccess === username) ?? searchInputRef;
      }
      target?.focus();
    }, 0);
  };

  return (
    <section ref={panel} aria-label="User Access" tabIndex={-1} class="focus:outline-2">
      <SettingsPanel
        title="User Access"
        action={
          <SearchField
            inputRef={(element) => {
              searchInputRef = element;
            }}
            placeholder="Search users..."
            value={state.searchQuery()}
            onChange={state.setSearchQuery}
            disabled={
              !state.accessAllowed() || Boolean(state.loadError() || state.unconfirmedChange())
            }
            class="min-w-60"
            inputClass="min-h-10 sm:min-h-9 py-2.5"
          />
        }
        noPadding
        bodyClass="divide-y divide-border"
      >
        <Show when={state.successNotice()}>
          {(message) => (
            <InlineNotice tone="success" role="status" aria-live="polite">
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show when={state.featureGate.paywallVisible()}>
          <RBACFeatureGateSection
            copy={state.featureGate.featureGateCopy()}
            paywallLocation="settings_user_assignments_panel"
            showUpgradePrompts={state.featureGate.showUpgradePrompts()}
          />
        </Show>

        <Show when={state.accessAllowed() && state.unconfirmedChange()}>
          {(message) => (
            <InlineNotice
              ref={inventoryNotice}
              tabindex={-1}
              tone="warning"
              layout="banner"
              role="alert"
              aria-live="polite"
              class="break-words"
              icon={<TriangleAlert />}
              actionLabel={state.loading() ? undefined : 'Check saved access'}
              actionOnClick={() => void saveOrReload(true)}
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show when={state.loading()}>
          <div class="flex flex-col items-center gap-3 px-4 py-8">
            <div
              class="flex items-center gap-2 text-sm text-muted"
              role="status"
              aria-label="Loading user access"
            >
              <LoadingSpinner size="md" tone="info" />
              Loading user access…
            </div>
            <Show when={state.accessAllowed()}>
              <Button size="mdCompact" class="min-h-11 sm:min-h-9" onClick={restartRead}>
                Restart loading
              </Button>
            </Show>
          </div>
        </Show>

        <Show when={!state.loading() && state.featureGate.rbacEnabled() && !state.accessAllowed()}>
          <InlineNotice tone="warning" role="status">
            User access is unavailable for this session.
          </InlineNotice>
        </Show>

        <Show when={!state.loading() && state.accessAllowed() && state.loadError()}>
          {(message) => (
            <InlineNotice
              ref={loadFailureNotice}
              tabindex={-1}
              role="alert"
              aria-live="polite"
              tone="danger"
              layout="banner"
              icon={<TriangleAlert />}
              actionLabel={state.unconfirmedChange() ? undefined : 'Retry'}
              actionOnClick={() => void saveOrReload(true)}
              class="focus:outline-hidden focus:ring-2 focus:ring-inset focus:ring-blue-500"
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show when={inventoryReady() && state.assignmentCount() > 0}>
          <div
            role="status"
            aria-label="User search results"
            aria-live="polite"
            aria-atomic="true"
            class="px-3 py-3 text-sm text-muted sm:px-6"
          >
            <Show
              when={state.searchQuery().trim()}
              fallback={`${state.assignmentCount()} ${state.assignmentCount() === 1 ? 'user' : 'users'}`}
            >
              Showing {state.filteredAssignments().length} of {state.assignmentCount()}{' '}
              {state.assignmentCount() === 1 ? 'user' : 'users'}
            </Show>
          </div>
        </Show>

        <Show when={inventoryReady() && state.filteredAssignments().length === 0}>
          <div class="text-center py-12 px-6">
            <Users class="w-12 h-12 mx-auto text-slate-300 mb-4" />
            <Show
              when={state.assignmentCount() > 0}
              fallback={
                <>
                  <h4 class="text-base font-medium text-base-content mb-2">
                    {emptyStateCopy().title}
                  </h4>
                  <p class="text-sm text-muted max-w-md mx-auto">{emptyStateCopy().body}</p>
                  <div class="mt-6 flex flex-col sm:flex-row items-center justify-center gap-3 text-xs text-muted">
                    <span class="flex items-center gap-1.5">
                      <Shield class="w-3.5 h-3.5" />
                      {emptyStateCopy().ssoHint}
                    </span>
                    <span class="hidden sm:inline">•</span>
                    <span>{emptyStateCopy().syncHint}</span>
                  </div>
                </>
              }
            >
              <h4 class="text-base font-medium text-base-content mb-2">No matching users</h4>
              <p class="text-sm text-muted max-w-md mx-auto">
                No saved users match this search. Clear it to review all user access.
              </p>
              <Button
                class="mt-6"
                onClick={() => {
                  state.setSearchQuery('');
                  searchInputRef?.focus();
                }}
              >
                Show all users
              </Button>
            </Show>
          </div>
        </Show>

        <Show when={inventoryReady() && state.filteredAssignments().length > 0}>
          <PulseDataGrid
            data={state.filteredAssignments()}
            columns={[
              {
                key: 'username',
                label: 'User',
                render: (assignment) => <UserAccessIdentity user={assignment} />,
              },
              {
                key: 'assignedRoles',
                label: 'Assigned Roles',
                render: (assignment) => (
                  <div class="flex flex-wrap gap-1 whitespace-normal [overflow-wrap:anywhere]">
                    <Show when={assignment.roleIds.length === 0}>
                      <span class="text-xs text-slate-400 italic">No roles assigned</span>
                    </Show>
                    <For each={assignment.roleIds}>
                      {(roleId) => {
                        const role = () =>
                          state.roles().find((candidate) => candidate.id === roleId);
                        return (
                          <span class="inline-flex max-w-full items-start gap-1 whitespace-normal [overflow-wrap:anywhere] rounded-md bg-surface-alt px-2 py-0.5 text-xs font-medium text-base-content border border-border">
                            <Shield class="mt-0.5 w-3 h-3 shrink-0" />
                            <RoleAccessIdentity
                              id={roleId}
                              name={role()?.name}
                              unlisted={!role()}
                            />
                          </span>
                        );
                      }}
                    </For>
                  </div>
                ),
              },
              {
                key: 'actions',
                label: 'Actions',
                align: 'right',
                render: (assignment) => {
                  const identity = getUserIdentityLabel(assignment);
                  return (
                    <div class="flex flex-wrap items-center justify-end gap-1 whitespace-normal text-xs text-muted">
                      <Button
                        variant="ghost"
                        size="settingsAction"
                        class="min-h-11 min-w-11 gap-2 px-2 sm:min-h-9 sm:min-w-0"
                        aria-label={`Manage access for ${identity}`}
                        title="Manage access"
                        disabled={!state.canEdit()}
                        data-user-access={assignment.username}
                        onClick={() => void state.openManageAccess(assignment)}
                      >
                        <Pencil class="w-4 h-4" />
                        <span class="hidden sm:inline">Manage Access</span>
                      </Button>
                      {state.isOwnAccount(assignment) ? (
                        USER_ACCESS_REMOVAL_COPY.self
                      ) : (
                        <Button
                          variant="ghost"
                          size="settingsAction"
                          class="min-h-11 min-w-11 gap-2 px-2 text-red-600 hover:text-red-700 sm:min-h-9 sm:min-w-0"
                          aria-label={`Remove ${identity}`}
                          title="Remove user access"
                          disabled={!state.canEdit()}
                          onClick={() => state.openDeleteUser(assignment)}
                        >
                          <Trash2 class="w-4 h-4" />
                          <span class="hidden sm:inline">Remove</span>
                        </Button>
                      )}
                    </div>
                  );
                },
              },
            ]}
            keyExtractor={(assignment) => assignment.username}
            class="[&_table]:table-fixed sm:[&_table]:table-auto [&_th]:whitespace-normal sm:[&_th]:whitespace-nowrap [&_th:first-child]:w-5/12 sm:[&_th:first-child]:w-auto [&_th:last-child]:w-24 sm:[&_th:last-child]:w-auto"
            emptyState={emptyStateCopy().title}
            desktopMinWidth="620px"
            frame="flush"
          />
        </Show>
      </SettingsPanel>

      <UserAssignmentsDialog
        actionError={state.actionError()}
        actionErrorRef={(element) => {
          editorFailureNotice = element;
        }}
        editingUser={state.editingUser()}
        formRoleIds={state.formRoleIds()}
        isOpen={state.showModal()}
        loadingPermissions={state.loadingPermissions()}
        permissionsError={state.permissionsError()}
        roles={state.roles()}
        saving={state.saving()}
        userPermissions={state.userPermissions()}
        onClose={state.closeModal}
        onSave={() => void saveOrReload()}
        onReloadPermissions={state.loadUserPermissions}
        onToggleRole={state.toggleRole}
      />

      <Show when={state.userPendingDeletion()}>
        {(user) => (
          <Dialog
            isOpen={true}
            onClose={state.closeDeleteUser}
            closeOnBackdrop={false}
            panelClass="max-w-lg"
            ariaLabel={`Remove user access: ${getUserIdentityLabel(user())}`}
            returnFocus={() => searchInputRef}
          >
            <div class="space-y-5 p-6">
              <div>
                <h3 class="text-lg font-semibold text-base-content">Remove user access?</h3>
                <p class="mt-2 text-sm text-muted">{USER_ACCESS_REMOVAL_COPY.effect}</p>
                <div class="mt-2">
                  <UserAccessIdentity user={user()} />
                </div>
              </div>
              <div class="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/25 dark:text-amber-100">
                {USER_ACCESS_REMOVAL_COPY.identityProvider}
              </div>
              <Show when={state.actionError()}>
                {(message) => (
                  <InlineNotice tone="danger" role="alert" aria-live="polite">
                    {message()}
                  </InlineNotice>
                )}
              </Show>
              <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
                <Button onClick={state.closeDeleteUser} disabled={state.deletingUser()}>
                  Cancel
                </Button>
                <Button
                  variant="danger"
                  isLoading={state.deletingUser()}
                  onClick={() => void state.handleDeleteUser()}
                >
                  Remove User Access
                </Button>
              </div>
            </div>
          </Dialog>
        )}
      </Show>
    </section>
  );
};

export default UserAssignmentsPanel;
