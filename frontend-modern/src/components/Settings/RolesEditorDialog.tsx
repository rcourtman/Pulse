import { Component, For, Show } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import type { Permission, Role } from '@/types/rbac';
import { RBAC_PERMISSION_ACTIONS, RBAC_PERMISSION_RESOURCES } from '@/utils/rbacPermissions';
import Plus from 'lucide-solid/icons/plus';
import Trash2 from 'lucide-solid/icons/trash-2';
import X from 'lucide-solid/icons/x';

interface RolesEditorDialogProps {
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
  onUpdatePermission: (index: number, field: keyof Permission, value: string) => void;
}

export const RolesEditorDialog: Component<RolesEditorDialogProps> = (props) => (
  <Show when={props.isOpen}>
    <Dialog
      isOpen={true}
      onClose={props.onClose}
      panelClass="max-w-2xl"
      closeOnBackdrop={false}
      ariaLabel={props.editingRole ? 'Edit role' : 'New role'}
    >
      <div class="w-full max-h-[92vh] overflow-hidden">
        <div class="flex items-start justify-between gap-3 px-4 sm:px-6 py-4 border-b border-border">
          <h3 class="text-lg font-semibold text-base-content">
            {props.editingRole ? 'Edit Role' : 'New Role'}
          </h3>
          <button
            type="button"
            onClick={props.onClose}
            class="p-1.5 rounded-md text-slate-500 hover:text-base-content hover:bg-surface-hover"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="px-4 sm:px-6 py-4 space-y-4 max-h-[70vh] overflow-y-auto">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div class="space-y-1">
              <label class="block text-sm font-medium text-base-content">Role ID</label>
              <input
                type="text"
                value={props.formId}
                onInput={(event) => props.onFormIdInput(event.currentTarget.value)}
                placeholder="e.g., custom-auditor"
                disabled={!!props.editingRole}
                class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900 disabled:opacity-50"
              />
            </div>
            <div class="space-y-1">
              <label class="block text-sm font-medium text-base-content">Role Name</label>
              <input
                type="text"
                value={props.formName}
                onInput={(event) => props.onFormNameInput(event.currentTarget.value)}
                placeholder="e.g., Custom Auditor"
                class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900"
              />
            </div>
          </div>
          <div class="space-y-1">
            <label class="block text-sm font-medium text-base-content">Description</label>
            <input
              type="text"
              value={props.formDescription}
              onInput={(event) => props.onFormDescriptionInput(event.currentTarget.value)}
              placeholder="Brief description of this role's purpose"
              class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:focus:border-blue-400 dark:focus:ring-blue-900"
            />
          </div>

          <div class="space-y-3 pt-2">
            <div class="flex flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-between">
              <label class="block text-sm font-medium text-base-content">Permissions</label>
              <button
                type="button"
                onClick={props.onAddPermission}
                class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-300 flex items-center gap-1"
              >
                <Plus class="w-3 h-3" /> Add Permission
              </button>
            </div>

            <div class="space-y-2">
              <For each={props.formPermissions}>
                {(permission, index) => (
                  <div class="flex flex-col sm:flex-row sm:items-center gap-2">
                    <select
                      value={permission.action}
                      onChange={(event) =>
                        props.onUpdatePermission(index(), 'action', event.currentTarget.value)
                      }
                      class="w-full sm:flex-1 rounded-md border border-border bg-surface px-2 py-1.5 text-sm text-base-content"
                    >
                      <For each={RBAC_PERMISSION_ACTIONS}>
                        {(action) => <option value={action}>{action}</option>}
                      </For>
                    </select>
                    <span class="hidden sm:inline text-slate-400 text-sm">:</span>
                    <select
                      value={permission.resource}
                      onChange={(event) =>
                        props.onUpdatePermission(index(), 'resource', event.currentTarget.value)
                      }
                      class="w-full sm:flex-1 rounded-md border bg-surface px-2 py-1.5 text-sm text-base-content"
                    >
                      <For each={RBAC_PERMISSION_RESOURCES}>
                        {(resource) => <option value={resource}>{resource}</option>}
                      </For>
                    </select>
                    <button
                      type="button"
                      onClick={() => props.onRemovePermission(index())}
                      disabled={props.formPermissions.length <= 1}
                      class="self-end sm:self-auto p-1.5 text-slate-400 hover:text-red-500 disabled:opacity-30"
                    >
                      <Trash2 class="w-4 h-4" />
                    </button>
                  </div>
                )}
              </For>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:flex sm:items-center sm:justify-end gap-3 px-4 sm:px-6 py-4 border-t border-border">
          <button
            type="button"
            onClick={props.onClose}
            class="w-full sm:w-auto rounded-md px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={props.onSave}
            disabled={props.saving || !props.formName.trim()}
            class="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {props.saving ? 'Saving...' : props.editingRole ? 'Update Role' : 'Create Role'}
          </button>
        </div>
      </div>
    </Dialog>
  </Show>
);
