import { Component, For, Show, createEffect, on, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACFeatureGateSection } from './RBACFeatureGateSection';
import { UserAccessIdentity, UserAssignmentsDialog } from './UserAssignmentsDialog';
import { useUserAssignmentsPanelState } from './useUserAssignmentsPanelState';
import type { UserAssignmentsPanelProps } from './useUserAssignmentsPanelState';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import Users from 'lucide-solid/icons/users';
import Shield from 'lucide-solid/icons/shield';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import { SearchField } from '@/components/shared/SearchField';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { getUserAssignmentsEmptyStateCopy, getUserIdentityLabel } from '@/utils/rbacPresentation';
import { InlineNotice } from '@/components/shared/InlineNotice';
import TriangleAlert from 'lucide-solid/icons/triangle-alert';
import { Dialog } from '@/components/shared/Dialog';

export const UserAssignmentsPanel: Component<UserAssignmentsPanelProps> = (props) => {
  const state = useUserAssignmentsPanelState(props);
  const emptyStateCopy = () => getUserAssignmentsEmptyStateCopy();
  let searchInputRef: HTMLInputElement | undefined;
  let inventoryNotice: HTMLDivElement | undefined;
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

  return (
    <div class="space-y-6">
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
              actionOnClick={() => void state.loadData()}
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show when={state.loading()}>
          <div class="flex items-center justify-center py-8">
            <LoadingSpinner size="xl" tone="info" label="Loading user access" />
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
              role="alert"
              aria-live="polite"
              tone="danger"
              layout="banner"
              icon={<TriangleAlert />}
              actionLabel={state.unconfirmedChange() ? undefined : 'Retry'}
              actionOnClick={() => void state.loadData()}
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show
          when={
            !state.loading() &&
            state.accessAllowed() &&
            !state.loadError() &&
            !state.unconfirmedChange() &&
            state.assignmentCount() > 0
          }
        >
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

        <Show
          when={
            !state.loading() &&
            state.accessAllowed() &&
            !state.loadError() &&
            !state.unconfirmedChange() &&
            state.filteredAssignments().length === 0
          }
        >
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

        <Show
          when={
            !state.loading() &&
            state.accessAllowed() &&
            !state.loadError() &&
            !state.unconfirmedChange() &&
            state.filteredAssignments().length > 0
          }
        >
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
                            <span class="min-w-0">
                              <Show when={role()?.name.trim() && role()?.name !== roleId}>
                                <bdi class="block">{role()!.name}</bdi>
                              </Show>
                              <span class="block text-[10px] leading-4 text-muted">
                                Role ID: <bdi class="font-mono whitespace-pre-wrap">{roleId}</bdi>
                              </span>
                              <Show when={!role()}>
                                <span class="block text-[10px] leading-4 text-muted">
                                  Not in role list
                                </span>
                              </Show>
                            </span>
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
                render: (assignment) => (
                  <div class="flex flex-wrap justify-end gap-1">
                    <Button
                      variant="ghost"
                      size="settingsAction"
                      class="min-h-11 min-w-11 gap-2 px-2 sm:min-h-9 sm:min-w-0"
                      aria-label={`Manage access for ${getUserIdentityLabel(assignment)}`}
                      title="Manage access"
                      disabled={!state.canEdit()}
                      onClick={() => state.openManageAccess(assignment)}
                    >
                      <Pencil class="w-4 h-4" />
                      <span class="hidden sm:inline">Manage Access</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="settingsAction"
                      class="min-h-11 min-w-11 gap-2 px-2 text-red-600 hover:text-red-700 sm:min-h-9 sm:min-w-0"
                      aria-label={`Remove ${getUserIdentityLabel(assignment)}`}
                      title="Remove user access"
                      disabled={!state.canEdit()}
                      onClick={() => state.openDeleteUser(assignment)}
                    >
                      <Trash2 class="w-4 h-4" />
                      <span class="hidden sm:inline">Remove</span>
                    </Button>
                  </div>
                ),
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
        editingUser={state.editingUser()}
        formRoleIds={state.formRoleIds()}
        isOpen={state.showModal()}
        loadingPermissions={state.loadingPermissions()}
        permissionsError={state.permissionsError()}
        roles={state.roles()}
        saving={state.saving()}
        userPermissions={state.userPermissions()}
        onClose={state.closeModal}
        onSave={state.handleSaveAssignments}
        onRetryPermissions={() => void state.loadUserPermissions()}
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
                <p class="mt-2 text-sm text-muted">
                  This removes all role assignments and revokes active Pulse sessions for:
                </p>
                <div class="mt-2">
                  <UserAccessIdentity user={user()} />
                </div>
              </div>
              <div class="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/25 dark:text-amber-100">
                This does not disable the account at the identity provider. A later authorized SSO
                login will create the user record again.
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
    </div>
  );
};

export default UserAssignmentsPanel;
