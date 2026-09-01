import { Component, For, Show } from 'solid-js';
import { Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACFeatureGateSection } from './RBACFeatureGateSection';
import { UserAssignmentsDialog } from './UserAssignmentsDialog';
import { useUserAssignmentsPanelState } from './useUserAssignmentsPanelState';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import Users from 'lucide-solid/icons/users';
import Shield from 'lucide-solid/icons/shield';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import { SearchField } from '@/components/shared/SearchField';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import {
  getUserAssignmentsEmptyStateCopy,
  getUserIdentityDisplayName,
  getUserIdentityProviderLabel,
} from '@/utils/rbacPresentation';
import { InlineNotice } from '@/components/shared/InlineNotice';
import TriangleAlert from 'lucide-solid/icons/triangle-alert';
import { Dialog } from '@/components/shared/Dialog';

export const UserAssignmentsPanel: Component = () => {
  const state = useUserAssignmentsPanelState();
  const emptyStateCopy = () => getUserAssignmentsEmptyStateCopy();
  let searchInputRef: HTMLInputElement | undefined;

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
            disabled={!state.featureGate.rbacEnabled() || Boolean(state.loadError())}
            class="min-w-[15rem]"
            inputClass="min-h-10 sm:min-h-9 py-2.5"
          />
        }
        noPadding
        bodyClass="divide-y divide-border"
      >
        <Show when={state.featureGate.paywallVisible()}>
          <RBACFeatureGateSection
            copy={state.featureGate.featureGateCopy()}
            paywallLocation="settings_user_assignments_panel"
            showUpgradePrompts={state.featureGate.showUpgradePrompts()}
          />
        </Show>

        <Show when={state.loading()}>
          <div class="flex items-center justify-center py-8">
            <LoadingSpinner size="xl" tone="info" label="Loading user access" />
          </div>
        </Show>

        <Show when={!state.loading() && state.featureGate.rbacEnabled() && state.loadError()}>
          {(message) => (
            <InlineNotice
              role="alert"
              aria-live="polite"
              tone="danger"
              layout="banner"
              icon={<TriangleAlert />}
              actionLabel="Retry"
              actionOnClick={() => void state.loadData()}
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show
          when={
            !state.loading() &&
            state.featureGate.rbacEnabled() &&
            !state.loadError() &&
            state.filteredAssignments().length === 0
          }
        >
          <div class="text-center py-12 px-6">
            <Users class="w-12 h-12 mx-auto text-slate-300 mb-4" />
            <h4 class="text-base font-medium text-base-content mb-2">{emptyStateCopy().title}</h4>
            <p class="text-sm text-muted max-w-md mx-auto">{emptyStateCopy().body}</p>
            <div class="mt-6 flex flex-col sm:flex-row items-center justify-center gap-3 text-xs text-muted">
              <span class="flex items-center gap-1.5">
                <Shield class="w-3.5 h-3.5" />
                {emptyStateCopy().ssoHint}
              </span>
              <span class="hidden sm:inline">•</span>
              <span>{emptyStateCopy().syncHint}</span>
            </div>
          </div>
        </Show>

        <Show
          when={
            !state.loading() &&
            state.featureGate.rbacEnabled() &&
            !state.loadError() &&
            state.filteredAssignments().length > 0
          }
        >
          <PulseDataGrid
            data={state.filteredAssignments()}
            columns={[
              {
                key: 'username',
                label: 'User',
                render: (assignment) => (
                  <div class="min-w-0">
                    <div class="font-medium text-base-content">
                      {getUserIdentityDisplayName(assignment)}
                    </div>
                    <Show when={assignment.email && assignment.email !== assignment.displayName}>
                      <div class="truncate text-xs text-muted">{assignment.email}</div>
                    </Show>
                    <Show when={getUserIdentityProviderLabel(assignment)}>
                      <div class="text-xs text-muted">
                        {getUserIdentityProviderLabel(assignment)}
                      </div>
                    </Show>
                    <Show when={assignment.username !== getUserIdentityDisplayName(assignment)}>
                      <div
                        class="max-w-md truncate font-mono text-[10px] text-muted"
                        title={assignment.username}
                      >
                        {assignment.username}
                      </div>
                    </Show>
                  </div>
                ),
              },
              {
                key: 'assignedRoles',
                label: 'Assigned Roles',
                render: (assignment) => (
                  <div class="flex flex-wrap gap-1">
                    <Show when={assignment.roleIds.length === 0}>
                      <span class="text-xs text-slate-400 italic">No roles assigned</span>
                    </Show>
                    <For each={assignment.roleIds}>
                      {(roleId) => (
                        <span class="inline-flex items-center gap-1 rounded-md bg-surface-alt px-2 py-0.5 text-xs font-medium text-base-content border border-border">
                          <Shield class="w-3 h-3" />
                          {state.getRoleName(roleId)}
                        </span>
                      )}
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
                      aria-label={`Manage access for ${getUserIdentityDisplayName(assignment)}`}
                      title="Manage access"
                      onClick={() => state.openManageAccess(assignment)}
                    >
                      <Pencil class="w-4 h-4" />
                      <span class="hidden sm:inline">Manage Access</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="settingsAction"
                      class="min-h-11 min-w-11 gap-2 px-2 text-red-600 hover:text-red-700 sm:min-h-9 sm:min-w-0"
                      aria-label={`Remove ${getUserIdentityDisplayName(assignment)}`}
                      title="Remove user access"
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
            emptyState={emptyStateCopy().title}
            desktopMinWidth="620px"
            frame="flush"
          />
        </Show>
      </SettingsPanel>

      <UserAssignmentsDialog
        editingUser={state.editingUser()}
        formRoleIds={state.formRoleIds()}
        isOpen={state.showModal()}
        loadingPermissions={state.loadingPermissions()}
        roles={state.roles()}
        saving={state.saving()}
        userPermissions={state.userPermissions()}
        onClose={state.closeModal}
        onSave={state.handleSaveAssignments}
        onToggleRole={state.toggleRole}
      />

      <Show when={state.userPendingDeletion()}>
        {(user) => (
          <Dialog
            isOpen={true}
            onClose={state.closeDeleteUser}
            closeOnBackdrop={false}
            panelClass="max-w-lg"
            ariaLabel={`Remove user access: ${getUserIdentityDisplayName(user())}`}
            returnFocus={() => searchInputRef}
          >
            <div class="space-y-5 p-6">
              <div>
                <h3 class="text-lg font-semibold text-base-content">Remove user access?</h3>
                <p class="mt-2 text-sm text-muted">
                  This removes all role assignments for {getUserIdentityDisplayName(user())} and
                  revokes their active Pulse sessions.
                </p>
              </div>
              <div class="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-100">
                This does not disable the account at the identity provider. A later authorized SSO
                login will create the user record again.
              </div>
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
