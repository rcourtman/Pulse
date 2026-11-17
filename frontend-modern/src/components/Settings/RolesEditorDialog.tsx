import { Component, Index, Show, createUniqueId } from 'solid-js';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { Dialog } from '@/components/shared/Dialog';
import { FormSelect } from '@/components/shared/FormSelect';
import type { Permission, Role } from '@/types/rbac';
import { PermissionPolicyDetails } from './PermissionPolicyDetails';
import { RBAC_PERMISSION_ACTIONS, RBAC_PERMISSION_RESOURCES } from '@/utils/rbacPermissions';
import { getRoleIdentityLabel } from '@/utils/rbacPresentation';
import Plus from 'lucide-solid/icons/plus';
import Trash2 from 'lucide-solid/icons/trash-2';
import X from 'lucide-solid/icons/x';

interface RolesEditorDialogProps {
  actionError?: string | null;
  editingRole: Role | null;
  formDescription: string;
  formId: string;
  formName: string;
  formPermissions: Permission[];
  isOpen: boolean;
  saving: boolean;
  onAddPermission: () => void;
  onClose: () => void;
  onFormDescriptionInput: (value: string) => void;
  onFormIdInput: (value: string) => void;
  onFormNameInput: (value: string) => void;
  onRemovePermission: (index: number) => void;
  onSave: () => void;
  onUpdatePermission: (index: number, field: 'action' | 'resource', value: string) => void;
}

// Catalog text is its exact value; share the static option renderer across rows.
const renderCatalogOption = (value: string) => <option>{value}</option>;

export const RolesEditorDialog: Component<RolesEditorDialogProps> = (props) => (
  <Show when={props.isOpen}>
    <Dialog
      isOpen={true}
      onClose={props.onClose}
      panelClass="max-w-2xl"
      closeOnBackdrop={false}
      ariaLabel={
        props.editingRole ? `Edit role: ${getRoleIdentityLabel(props.editingRole)}` : 'New role'
      }
    >
      <form
        class="flex min-h-0 w-full flex-col"
        onSubmit={(event) => {
          event.preventDefault();
          if (!props.saving && props.formName.trim()) props.onSave();
        }}
      >
        <div class="flex shrink-0 items-start justify-between gap-3 px-4 sm:px-6 py-4 border-b border-border">
          <div class="min-w-0 [overflow-wrap:anywhere]">
            <h3 class="text-lg font-semibold text-base-content">
              {props.editingRole ? 'Edit Role' : 'New Role'}
            </h3>
            <Show when={props.editingRole}>
              {(role) => (
                <p class="text-xs text-muted">
                  Role ID: <bdi class="font-mono whitespace-pre-wrap">{role().id}</bdi>
                </p>
              )}
            </Show>
          </div>
          <button
            type="button"
            onClick={props.onClose}
            disabled={props.saving}
            aria-label="Close role editor"
            title="Close"
            class="shrink-0 p-1.5 rounded-md text-slate-500 hover:text-base-content hover:bg-surface-hover"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="min-h-0 overflow-y-auto">
          <fieldset
            disabled={props.saving}
            class="px-4 sm:px-6 py-3 space-y-3 sm:py-4 sm:space-y-4"
          >
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 sm:gap-4">
              <div class="space-y-1">
                <label for="role-editor-id" class="block text-sm font-medium text-base-content">
                  Role ID
                </label>
                <input
                  id="role-editor-id"
                  type="text"
                  value={props.formId}
                  onInput={(event) => props.onFormIdInput(event.currentTarget.value)}
                  placeholder="e.g., custom-auditor"
                  disabled={!!props.editingRole}
                  class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900/25 disabled:opacity-50"
                />
              </div>
              <div class="space-y-1">
                <label for="role-editor-name" class="block text-sm font-medium text-base-content">
                  Role Name
                </label>
                <input
                  id="role-editor-name"
                  type="text"
                  value={props.formName}
                  onInput={(event) => props.onFormNameInput(event.currentTarget.value)}
                  placeholder="e.g., Custom Auditor"
                  class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900/25"
                />
              </div>
            </div>
            <div class="space-y-1">
              <label
                for="role-editor-description"
                class="block text-sm font-medium text-base-content"
              >
                Description
              </label>
              <input
                id="role-editor-description"
                type="text"
                value={props.formDescription}
                onInput={(event) => props.onFormDescriptionInput(event.currentTarget.value)}
                placeholder="Brief description of this role's purpose"
                class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900/25"
              />
            </div>

            <div class="space-y-3 pt-2">
              <div class="flex flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-between">
                <span class="block text-sm font-medium text-base-content">Permissions</span>
                <button
                  type="button"
                  onClick={props.onAddPermission}
                  class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-300 flex items-center gap-1"
                >
                  <Plus class="w-3 h-3" /> Add Permission
                </button>
              </div>

              <p class="text-xs text-muted">
                Effects and conditions are kept when you change action or resource. All conditions
                must match request attributes. A missing attribute skips the rule, including a deny
                rule.
              </p>
              <div class="space-y-2">
                {/* Draft updates replace permission objects. Keep the indexed controls
                    mounted so changing a value does not discard keyboard focus. */}
                <Index each={props.formPermissions}>
                  {(permission, index) => {
                    const policyID = createUniqueId();
                    return (
                      <div role="group" aria-label={`Permission ${index + 1}`} class="space-y-1">
                        <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-center gap-2">
                          <FormSelect
                            label={`Permission ${index + 1} action`}
                            aria-describedby={policyID}
                            labelClass="sr-only"
                            fieldBaseClass="contents"
                            value={permission().action}
                            onChange={(event) =>
                              props.onUpdatePermission(index, 'action', event.currentTarget.value)
                            }
                            selectBaseClass="min-w-0 w-full rounded-md border border-border bg-surface px-2 py-1.5 text-sm text-base-content"
                          >
                            <Show
                              when={
                                !(RBAC_PERMISSION_ACTIONS as readonly string[]).includes(
                                  permission().action,
                                )
                              }
                            >
                              <option value={permission().action}>{permission().action}</option>
                            </Show>
                            {RBAC_PERMISSION_ACTIONS.map(renderCatalogOption)}
                          </FormSelect>
                          <FormSelect
                            label={`Permission ${index + 1} resource`}
                            aria-describedby={policyID}
                            labelClass="sr-only"
                            fieldBaseClass="contents"
                            value={permission().resource}
                            onChange={(event) =>
                              props.onUpdatePermission(index, 'resource', event.currentTarget.value)
                            }
                            selectBaseClass="min-w-0 w-full rounded-md border bg-surface px-2 py-1.5 text-sm text-base-content"
                          >
                            <Show
                              when={
                                !(RBAC_PERMISSION_RESOURCES as readonly string[]).includes(
                                  permission().resource,
                                )
                              }
                            >
                              <option value={permission().resource}>{permission().resource}</option>
                            </Show>
                            {RBAC_PERMISSION_RESOURCES.map(renderCatalogOption)}
                          </FormSelect>
                          <button
                            type="button"
                            onClick={(event) => {
                              const controls = event.currentTarget.form?.querySelectorAll('select');
                              const count = props.formPermissions.length;
                              props.onRemovePermission(index);
                              // Index retains slots, each with action then resource.
                              // Continue at the next action, or previous for the last
                              // slot. A detached editor cannot receive this focus.
                              if (props.formPermissions.length === count - 1) {
                                controls?.[Math.min(index, count - 2) * 2]?.focus();
                              }
                            }}
                            disabled={props.formPermissions.length <= 1}
                            aria-label={`Remove permission ${index + 1}: ${permission().action}:${permission().resource}`}
                            aria-describedby={policyID}
                            title="Remove permission"
                            class="p-1.5 text-slate-400 hover:text-red-500 disabled:opacity-30"
                          >
                            <Trash2 class="w-4 h-4" />
                          </button>
                        </div>
                        <PermissionPolicyDetails
                          id={policyID}
                          permission={permission()}
                          showTarget
                        />
                      </div>
                    );
                  }}
                </Index>
              </div>
            </div>
          </fieldset>
          <Show when={props.actionError}>
            <InlineNotice tone="danger" role="alert" aria-live="polite">
              {props.actionError}
            </InlineNotice>
          </Show>
        </div>

        <div class="grid shrink-0 grid-cols-1 sm:flex sm:items-center sm:justify-end gap-3 px-4 sm:px-6 py-4 border-t border-border">
          <button
            type="button"
            onClick={props.onClose}
            disabled={props.saving}
            class="w-full sm:w-auto rounded-md px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={props.saving || !props.formName.trim()}
            class="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {props.saving ? 'Saving...' : props.editingRole ? 'Update Role' : 'Create Role'}
          </button>
        </div>
      </form>
    </Dialog>
  </Show>
);
