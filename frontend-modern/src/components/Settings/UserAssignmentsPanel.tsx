import { Component, For, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACFeatureGateSection } from './RBACFeatureGateSection';
import { UserAssignmentsDialog } from './UserAssignmentsDialog';
import { useUserAssignmentsPanelState } from './useUserAssignmentsPanelState';
import Users from 'lucide-solid/icons/users';
import Shield from 'lucide-solid/icons/shield';
import Pencil from 'lucide-solid/icons/pencil';
import { SearchField } from '@/components/shared/SearchField';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import {
  getUserAssignmentsEmptyStateCopy,
} from '@/utils/rbacPresentation';

export const UserAssignmentsPanel: Component = () => {
  const state = useUserAssignmentsPanelState();
  const emptyStateCopy = () => getUserAssignmentsEmptyStateCopy();

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="User Access"
        description="Assign roles to users and review effective permissions."
        icon={<Users class="w-5 h-5" />}
        action={
          <SearchField
            placeholder="Search users..."
            value={state.searchQuery()}
            onChange={state.setSearchQuery}
            disabled={!state.featureGate.rbacEnabled()}
            class="min-w-[15rem]"
            inputClass="min-h-10 sm:min-h-9 py-2.5"
          />
        }
        noPadding
        bodyClass="divide-y divide-border"
      >
        <Show when={state.featureGate.paywallVisible()}>
          <RBACFeatureGateSection
            canStartTrial={state.featureGate.canStartTrial()}
            copy={state.featureGate.featureGateCopy()}
            paywallLocation="settings_user_assignments_panel"
            startingTrial={state.featureGate.startingTrial()}
            onStartTrial={state.featureGate.handleStartTrial}
          />
        </Show>

        <Show when={state.loading()}>
          <div class="flex items-center justify-center py-8">
            <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" />
          </div>
        </Show>

        <Show
          when={
            !state.loading() &&
            state.featureGate.rbacEnabled() &&
            state.filteredAssignments().length === 0
          }
        >
          <div class="text-center py-12 px-6">
            <Users class="w-12 h-12 mx-auto text-slate-300 mb-4" />
            <h4 class="text-base font-medium text-base-content mb-2">{emptyStateCopy().title}</h4>
            <p class="text-sm text-muted max-w-md mx-auto">
              {emptyStateCopy().body}
            </p>
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
            state.filteredAssignments().length > 0
          }
        >
          <div class="w-full overflow-x-auto">
            <PulseDataGrid
              data={state.filteredAssignments()}
              columns={[
                {
                  key: 'username',
                  label: 'Username',
                  render: (assignment) => (
                    <span class="font-medium text-base-content">{assignment.username}</span>
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
                    <button
                      type="button"
                      onClick={() => state.openManageAccess(assignment)}
                      class="inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium text-base-content hover:bg-surface-hover transition-colors"
                    >
                      <Pencil class="w-4 h-4" />
                      Manage Access
                    </button>
                  ),
                },
              ]}
              keyExtractor={(assignment) => assignment.username}
              emptyState={emptyStateCopy().title}
              desktopMinWidth="620px"
              class="border-x-0 sm:border-x"
            />
          </div>
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
    </div>
  );
};

export default UserAssignmentsPanel;
