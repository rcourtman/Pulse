import { Component, createEffect, For, on, onCleanup, Show } from 'solid-js';
import { ActionIconButton, Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { RBACFeatureGateSection } from './RBACFeatureGateSection';
import { RolesEditorDialog } from './RolesEditorDialog';
import { useRolesPanelState, type RolesPanelProps } from './useRolesPanelState';
import { Dialog } from '@/components/shared/Dialog';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { getRoleIdentityLabel, getRolesEmptyState } from '@/utils/rbacPresentation';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Trash2 from 'lucide-solid/icons/trash-2';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { InlineNotice } from '@/components/shared/InlineNotice';
import TriangleAlert from 'lucide-solid/icons/triangle-alert';
import type { Role } from '@/types/rbac';

// Keep the same complete grants in the desktop column and phone identity cell.
const RolePermissions: Component<{ role: Role }> = (props) => (
  <Show
    when={props.role.permissions.length > 0}
    fallback={<span class="text-xs italic text-muted">No permissions</span>}
  >
    <ul
      aria-label={`Permissions for ${getRoleIdentityLabel(props.role)}`}
      class="flex min-w-0 flex-wrap gap-1"
    >
      <For each={props.role.permissions}>
        {(permission) => (
          <li class="max-w-full rounded-md bg-surface-alt px-2 py-0.5 text-xs font-medium text-muted border border-border whitespace-normal [overflow-wrap:anywhere]">
            {permission.action}:{permission.resource}
          </li>
        )}
      </For>
    </ul>
  </Show>
);

export const RolesPanel: Component<RolesPanelProps> = (props) => {
  const state = useRolesPanelState(props);
  let inventoryNotice: HTMLDivElement | undefined;
  let focusTimer: number | undefined;
  createEffect(
    on(state.unconfirmedChange, (change) => {
      window.clearTimeout(focusTimer);
      if (!change) return;
      // The former editor/confirmation is gone; keep keyboard recovery local.
      focusTimer = window.setTimeout(() => {
        if (state.unconfirmedChange() === change) inventoryNotice?.focus();
      }, 0);
    }),
  );
  onCleanup(() => window.clearTimeout(focusTimer));

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
            disabled={!state.canEdit()}
          >
            <Plus class="w-4 h-4" />
            New Role
          </Button>
        }
        noPadding
        bodyClass="divide-y divide-border"
      >
        <Show when={state.successNotice()}>
          {(message) => (
            <InlineNotice
              tone="success"
              role="status"
              aria-live="polite"
              class="[overflow-wrap:anywhere]"
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show when={state.featureGate.paywallVisible()}>
          <RBACFeatureGateSection
            copy={state.featureGate.featureGateCopy()}
            paywallLocation="settings_roles_panel"
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
              actionLabel={state.loading() ? undefined : 'Check saved roles'}
              actionOnClick={() => void state.loadRoles()}
            >
              {message()}
            </InlineNotice>
          )}
        </Show>

        <Show when={state.loading()}>
          <div class="flex items-center justify-center py-8">
            <LoadingSpinner size="xl" tone="info" label="Loading roles" />
          </div>
        </Show>

        <Show when={!state.loading() && state.featureGate.rbacEnabled() && !state.accessAllowed()}>
          <InlineNotice tone="warning" role="status">
            Roles are unavailable for this session.
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
              actionOnClick={() => void state.loadRoles()}
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
            !state.unconfirmedChange()
          }
        >
          <PulseDataGrid
            data={state.roles()}
            columns={[
              {
                key: 'role',
                label: 'Role',
                render: (role) => (
                  <div class="flex min-w-0 flex-col whitespace-normal [overflow-wrap:anywhere]">
                    <span class="font-medium text-base-content flex items-center gap-1">
                      <bdi class="min-w-0">{role.name}</bdi>
                      <Show when={role.isBuiltIn}>
                        <BadgeCheck class="w-4 h-4 shrink-0 text-blue-500" />
                      </Show>
                    </span>
                    <span class="text-[10px] leading-4 text-muted">
                      Role ID: <bdi class="font-mono whitespace-pre-wrap">{role.id}</bdi>
                    </span>
                    <span class="text-xs text-muted">{role.description}</span>
                    <div class="mt-3 space-y-1 sm:hidden">
                      <p class="text-[11px] font-medium text-muted">Permissions</p>
                      <RolePermissions role={role} />
                    </div>
                  </div>
                ),
              },
              {
                key: 'permissions',
                label: 'Permissions',
                hiddenOnMobile: true,
                render: (role) => <RolePermissions role={role} />,
              },
              {
                key: 'actions',
                label: 'Actions',
                align: 'right',
                render: (role) => (
                  <div class="inline-flex items-center gap-1">
                    <Show when={!role.isBuiltIn}>
                      <ActionIconButton
                        label={`Edit role ${getRoleIdentityLabel(role)}`}
                        tone="accent"
                        size="sm"
                        class="-my-2 min-h-11 min-w-11 sm:my-0 sm:min-h-0 sm:min-w-0"
                        disabled={!state.canEdit()}
                        onClick={() => state.openEditRole(role)}
                      >
                        <Pencil class="w-4 h-4" />
                      </ActionIconButton>
                      <ActionIconButton
                        label={`Delete role ${getRoleIdentityLabel(role)}`}
                        tone="danger"
                        size="sm"
                        class="-my-2 min-h-11 min-w-11 sm:my-0 sm:min-h-0 sm:min-w-0"
                        disabled={!state.canEdit()}
                        onClick={() => state.openDeleteRole(role)}
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
            class="[&_table]:table-fixed sm:[&_table]:table-auto [&_th:last-child]:w-28 sm:[&_th:last-child]:w-auto"
            desktopMinWidth="620px"
            frame="flush"
          />
        </Show>
      </SettingsPanel>

      <RolesEditorDialog
        actionError={state.actionError()}
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
      <Show keyed when={state.rolePendingDeletion()}>
        {(role) => (
          <Dialog
            isOpen={true}
            onClose={state.closeDeleteRole}
            closeOnBackdrop={false}
            panelClass="max-w-lg"
            ariaLabel={`Delete role: ${getRoleIdentityLabel(role)}`}
          >
            <div class="space-y-5 p-6">
              <div>
                <h3 class="text-lg font-semibold text-base-content">Delete role?</h3>
                <p class="mt-2 text-sm text-muted [overflow-wrap:anywhere]">
                  Are you sure you want to delete the role "<bdi>{role.name}</bdi>"?
                </p>
                <p class="mt-2 text-xs text-muted [overflow-wrap:anywhere]">
                  Role ID: <bdi class="font-mono whitespace-pre-wrap">{role.id}</bdi>
                </p>
              </div>
              <Show when={state.actionError()}>
                {(message) => (
                  <InlineNotice tone="danger" role="alert" aria-live="polite">
                    {message()}
                  </InlineNotice>
                )}
              </Show>
              <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
                <Button onClick={state.closeDeleteRole} disabled={state.deleting()}>
                  Cancel
                </Button>
                <Button
                  variant="danger"
                  isLoading={state.deleting()}
                  onClick={state.handleDeleteRole}
                >
                  Delete Role
                </Button>
              </div>
            </div>
          </Dialog>
        )}
      </Show>
    </div>
  );
};

export default RolesPanel;
