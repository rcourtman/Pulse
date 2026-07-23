import { Component, For, Show } from 'solid-js';
import { ActionIconButton, Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACFeatureGateSection } from './RBACFeatureGateSection';
import { RolesEditorDialog } from './RolesEditorDialog';
import { useRolesPanelState } from './useRolesPanelState';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { getRolesEmptyState } from '@/utils/rbacPresentation';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { InlineNotice } from '@/components/shared/InlineNotice';
import TriangleAlert from 'lucide-solid/icons/triangle-alert';

export const RolesPanel: Component = () => {
  const state = useRolesPanelState();

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Roles"
        action={
          <Button
            variant="primary"
            size="settingsAction"
            class="w-full gap-2 sm:w-auto"
            onClick={state.openCreateRole}
            disabled={!state.featureGate.rbacEnabled() || Boolean(state.loadError())}
          >
            <Plus class="w-4 h-4" />
            New Role
          </Button>
        }
        noPadding
        bodyClass="divide-y divide-border"
      >
        <Show when={state.featureGate.paywallVisible()}>
          <RBACFeatureGateSection
            copy={state.featureGate.featureGateCopy()}
            paywallLocation="settings_roles_panel"
            showUpgradePrompts={state.featureGate.showUpgradePrompts()}
          />
        </Show>

        <Show when={state.loading()}>
          <div class="flex items-center justify-center py-8">
            <LoadingSpinner size="xl" tone="info" label="Loading roles" />
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
              actionOnClick={() => void state.loadRoles()}
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show when={!state.loading() && state.featureGate.rbacEnabled() && !state.loadError()}>
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
                      <ActionIconButton
                        label="Edit role"
                        tone="accent"
                        size="sm"
                        onClick={() => state.openEditRole(role)}
                      >
                        <Pencil class="w-4 h-4" />
                      </ActionIconButton>
                      <ActionIconButton
                        label="Delete role"
                        tone="danger"
                        size="sm"
                        onClick={() => state.handleDeleteRole(role)}
                      >
                        <Trash2 class="w-4 h-4" />
                      </ActionIconButton>
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
            frame="flush"
          />
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
