import { Component, For, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACFeatureGateSection } from './RBACFeatureGateSection';
import { RolesEditorDialog } from './RolesEditorDialog';
import { useRolesPanelState } from './useRolesPanelState';
import { getRolesEmptyState } from '@/utils/rbacPresentation';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import Shield from 'lucide-solid/icons/shield';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';

export const RolesPanel: Component = () => {
  const state = useRolesPanelState();

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Roles"
        description="Manage built-in and custom roles with granular permissions."
        icon={<Shield class="w-5 h-5" />}
        action={
          <button
            type="button"
            onClick={state.openCreateRole}
            disabled={!state.featureGate.rbacEnabled()}
            class="inline-flex w-full sm:w-auto min-h-10 sm:min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-blue-700"
          >
            <Plus class="w-4 h-4" />
            New Role
          </button>
        }
        noPadding
        bodyClass="divide-y divide-border"
      >
        <Show when={state.featureGate.paywallVisible()}>
          <RBACFeatureGateSection
            canStartTrial={state.featureGate.canStartTrial()}
            copy={state.featureGate.featureGateCopy()}
            paywallLocation="settings_roles_panel"
            startingTrial={state.featureGate.startingTrial()}
            onStartTrial={state.featureGate.handleStartTrial}
          />
        </Show>

        <Show when={state.loading()}>
          <div class="flex items-center justify-center py-8">
            <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" />
          </div>
        </Show>

        <Show when={!state.loading() && state.featureGate.rbacEnabled()}>
          <div class="w-full overflow-x-auto">
            <PulseDataGrid
              data={state.roles()}
              columns={[
                {
                  key: 'role',
                  label: 'Role',
                  render: (role) => (
                    <div class="flex flex-col">
                      <span class="font-medium text-base-content flex items-center gap-1">
                        {role.name}
                        <Show when={role.isBuiltIn}>
                          <BadgeCheck class="w-4 h-4 text-blue-500" />
                        </Show>
                      </span>
                      <span class="text-xs text-muted">{role.description}</span>
                    </div>
                  ),
                },
                {
                  key: 'permissions',
                  label: 'Permissions',
                  render: (role) => (
                    <div class="flex flex-wrap gap-1">
                      <For each={role.permissions}>
                        {(perm) => (
                          <span class="inline-flex items-center rounded-md bg-surface-alt px-2 py-0.5 text-xs font-medium text-muted border border-border">
                            {perm.action}:{perm.resource}
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
                  render: (role) => (
                    <div class="inline-flex items-center gap-1">
                      <Show when={!role.isBuiltIn}>
                        <button
                          type="button"
                          onClick={() => state.openEditRole(role)}
                          class="p-1.5 rounded-md text-slate-500 hover:text-blue-600 hover:bg-surface-hover dark:hover:text-blue-300"
                          title="Edit role"
                        >
                          <Pencil class="w-4 h-4" />
                        </button>
                        <button
                          type="button"
                          onClick={() => state.handleDeleteRole(role)}
                          class="p-1.5 rounded-md text-slate-500 hover:text-red-600 hover:bg-red-50 dark:hover:text-red-400 dark:hover:bg-red-900"
                          title="Delete role"
                        >
                          <Trash2 class="w-4 h-4" />
                        </button>
                      </Show>
                      <Show when={role.isBuiltIn}>
                        <span class="text-xs text-slate-400 italic">Read-only</span>
                      </Show>
                    </div>
                  ),
                },
              ]}
              keyExtractor={(role) => role.id}
              emptyState={getRolesEmptyState()}
              desktopMinWidth="620px"
              class="border-x-0 sm:border-x"
            />
          </div>
        </Show>
      </SettingsPanel>

      <RolesEditorDialog
        editingRole={state.editingRole()}
        formDescription={state.formDescription()}
        formId={state.formId()}
        formName={state.formName()}
        formPermissions={state.formPermissions()}
        isOpen={state.showModal()}
        saving={state.saving()}
        onAddPermission={state.addPermission}
        onClose={state.closeModal}
        onFormDescriptionInput={state.setFormDescription}
        onFormIdInput={state.setFormId}
        onFormNameInput={state.setFormName}
        onRemovePermission={state.removePermission}
        onSave={state.handleSaveRole}
        onUpdatePermission={state.updatePermission}
      />
    </div>
  );
};

export default RolesPanel;
