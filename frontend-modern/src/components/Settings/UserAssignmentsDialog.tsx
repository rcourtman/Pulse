import { Component, createMemo, For, Show } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { InlineNotice } from '@/components/shared/InlineNotice';
import type { Permission, Role, UserRoleAssignment } from '@/types/rbac';
import BadgeCheck from 'lucide-solid/icons/badge-check';
import Shield from 'lucide-solid/icons/shield';
import X from 'lucide-solid/icons/x';
import {
  getRoleIdentityLabel,
  getUserIdentityDisplayName,
  getUserIdentityLabel,
  getUserIdentityProviderLabel,
  sameRoleIDs,
} from '@/utils/rbacPresentation';

/** The saved principal stays readable even when its mutable claims match another user. */
export const UserAccessIdentity: Component<{
  user: UserRoleAssignment;
  showName?: boolean;
}> = (props) => (
  <div class="min-w-0 whitespace-normal [overflow-wrap:anywhere]">
    <Show when={props.showName !== false}>
      <div class="font-medium text-base-content">{getUserIdentityDisplayName(props.user)}</div>
    </Show>
    <Show when={props.user.email && props.user.email !== getUserIdentityDisplayName(props.user)}>
      <div class="text-xs text-muted">{props.user.email}</div>
    </Show>
    <Show when={getUserIdentityProviderLabel(props.user)}>
      <div class="text-xs text-muted">{getUserIdentityProviderLabel(props.user)}</div>
    </Show>
    <Show when={props.user.username !== getUserIdentityDisplayName(props.user)}>
      <div class="font-mono text-[10px] leading-4 text-muted" title={props.user.username}>
        <span class="font-sans">User ID: </span>
        <span>{props.user.username}</span>
      </div>
    </Show>
  </div>
);

/** The inventory and editor describe the same saved key, not a name-derived grant. */
export const RoleAccessIdentity: Component<{
  id: string;
  name?: string;
  nameClass?: string;
  unlisted?: boolean;
}> = (props) => (
  <span class="min-w-0">
    <Show when={props.name?.trim() && props.name !== props.id}>
      <bdi class={`block ${props.nameClass ?? ''}`}>{props.name}</bdi>
    </Show>
    <span class="block text-[10px] leading-4 text-muted">
      Role ID: <bdi class="font-mono whitespace-pre-wrap">{props.id}</bdi>
    </span>
    <Show when={props.unlisted}>
      <span class="block text-[10px] leading-4 text-muted">Not in role list</span>
    </Show>
  </span>
);

interface UserAssignmentsDialogProps {
  actionError?: string | null;
  editingUser: UserRoleAssignment | null;
  formRoleIds: string[];
  isOpen: boolean;
  loadingPermissions: boolean;
  permissionsError: string | null;
  roles: Role[];
  saving: boolean;
  userPermissions: Permission[] | null;
  onClose: () => void;
  onSave: () => void;
  onRetryPermissions: () => void;
  onToggleRole: (roleId: string) => void;
}

export const UserAssignmentsDialog: Component<UserAssignmentsDialogProps> = (props) => {
  const unlistedRoleIds = createMemo(() =>
    props.formRoleIds.filter((id) => !props.roles.some((role) => role.id === id)),
  );
  return (
    <Show when={props.isOpen}>
      <Dialog
        isOpen={true}
        onClose={props.onClose}
        panelClass="max-w-2xl"
        closeOnBackdrop={false}
        ariaLabel={`Manage access: ${props.editingUser ? getUserIdentityLabel(props.editingUser) : 'user'}`}
      >
        <div class="flex min-h-0 w-full flex-col">
          <div class="flex shrink-0 items-start justify-between gap-3 px-4 sm:px-6 py-4 border-b border-border">
            <div class="min-w-0 whitespace-normal [overflow-wrap:anywhere]">
              <h3 class="text-lg font-semibold text-base-content">
                Manage Access:{' '}
                {props.editingUser ? getUserIdentityDisplayName(props.editingUser) : 'User'}
              </h3>
              <p class="text-xs text-muted uppercase tracking-wider font-semibold mt-0.5">
                Role Assignments
              </p>
              <Show when={props.editingUser}>
                {(user) => <UserAccessIdentity user={user()} showName={false} />}
              </Show>
            </div>
            <button
              type="button"
              onClick={props.onClose}
              aria-label="Close user assignments"
              title="Close"
              class="shrink-0 p-1.5 rounded-md hover:text-base-content hover:bg-surface-hover"
            >
              <X class="w-5 h-5" />
            </button>
          </div>

          <div class="min-h-0 overflow-y-auto px-4 sm:px-6 py-4 space-y-4 sm:py-6 sm:space-y-8">
            <Show when={props.actionError}>
              {(message) => (
                <InlineNotice tone="danger" role="alert" aria-live="polite">
                  {message()}
                </InlineNotice>
              )}
            </Show>
            <div class="space-y-2 sm:space-y-4">
              <h4 class="text-sm font-semibold text-base-content flex items-center gap-2">
                <Shield class="w-4 h-4 text-blue-500" />
                Select Roles
              </h4>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <For each={props.roles}>
                  {(role) => (
                    <label
                      class={`flex min-w-0 flex-col p-2 sm:p-3 rounded-md border transition-all cursor-pointer [overflow-wrap:anywhere] ${props.formRoleIds.includes(role.id) ? 'bg-blue-50 border-blue-200 dark:bg-blue-900/25 dark:border-blue-800' : 'bg-surface border-border hover:border-blue-100 dark:hover:border-blue-900/25'}`}
                    >
                      <div class="flex items-start justify-between gap-2 mb-1">
                        <div class="flex min-w-0 items-start gap-2 shadow-xs">
                          <input
                            type="checkbox"
                            aria-label={`${getRoleIdentityLabel(role)} ${role.description}`.trim()}
                            checked={props.formRoleIds.includes(role.id)}
                            onChange={() => props.onToggleRole(role.id)}
                            class="mt-0.5 w-4 h-4 shrink-0 text-blue-600 rounded-sm border-border focus:ring-blue-500"
                          />
                          <RoleAccessIdentity
                            id={role.id}
                            name={role.name}
                            nameClass="text-sm font-semibold text-base-content"
                          />
                        </div>
                        <Show when={role.isBuiltIn}>
                          <BadgeCheck class="w-4 h-4 shrink-0 text-blue-500" />
                        </Show>
                      </div>
                      <p class="text-xs text-muted line-clamp-2 leading-relaxed pl-6">
                        {role.description}
                      </p>
                    </label>
                  )}
                </For>
              </div>
            </div>

            <div class="space-y-4 pt-4 border-t border-border-subtle">
              <div class="flex items-center justify-between">
                <h4 class="text-sm font-semibold text-base-content flex items-center gap-2">
                  <BadgeCheck class="w-4 h-4 text-blue-500" />
                  Saved permissions
                </h4>
                <Show when={props.loadingPermissions}>
                  <LoadingSpinner size="md" tone="info" label="Loading effective permissions" />
                </Show>
              </div>
              <div class="bg-surface-hover rounded-md p-4 border border-border-subtle">
                <Show when={!props.loadingPermissions && props.permissionsError}>
                  {(message) => (
                    <InlineNotice
                      tone="warning"
                      role="alert"
                      aria-live="polite"
                      actionLabel="Retry saved permissions"
                      actionOnClick={props.onRetryPermissions}
                    >
                      {message()}
                    </InlineNotice>
                  )}
                </Show>
                <Show
                  when={
                    !props.loadingPermissions && !props.permissionsError && props.userPermissions
                  }
                >
                  {(permissions) => (
                    <>
                      <Show when={permissions().length === 0}>
                        <p class="text-xs text-muted italic text-center py-2">
                          No permissions were returned for this user's saved roles.
                        </p>
                      </Show>
                      <div class="flex flex-wrap gap-2">
                        <For each={permissions()}>
                          {(permission) => (
                            <span class="inline-flex max-w-full flex-wrap items-center break-all rounded-md bg-surface px-2.5 py-1 text-xs font-semibold text-base-content border border-border shadow-xs">
                              <span class="text-blue-600 dark:text-blue-400">
                                {permission.action}
                              </span>
                              <span class="mx-1 text-slate-400">:</span>
                              <span class="text-blue-600 dark:text-blue-400">
                                {permission.resource}
                              </span>
                            </span>
                          )}
                        </For>
                      </div>
                    </>
                  )}
                </Show>
                <p class="mt-4 text-xs text-muted">
                  This read checks saved roles only. Unsaved role selections are not included.
                </p>
                <Show when={unlistedRoleIds().length > 0}>
                  <div class="mt-3 space-y-1 text-xs text-muted">
                    <p>Saved role IDs not in this list are kept when you save:</p>
                    <ul aria-label="Kept saved role IDs" class="space-y-1">
                      <For each={unlistedRoleIds()}>
                        {(id) => (
                          <li class="[overflow-wrap:anywhere]">
                            Role ID: <bdi class="font-mono whitespace-pre-wrap">{id}</bdi>
                          </li>
                        )}
                      </For>
                    </ul>
                  </div>
                </Show>
              </div>
            </div>
          </div>

          <div class="grid shrink-0 grid-cols-1 sm:flex sm:items-center sm:justify-end gap-3 px-4 sm:px-6 py-5 border-t border-border bg-surface-alt rounded-b-xl">
            <button
              type="button"
              onClick={props.onClose}
              class="w-full sm:w-auto rounded-md px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover transition-colors"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={props.onSave}
              disabled={
                props.saving || sameRoleIDs(props.formRoleIds, props.editingUser?.roleIds ?? [])
              }
              class="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-md bg-blue-600 px-6 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {props.saving ? 'Applying...' : 'Save Changes'}
            </button>
          </div>
        </div>
      </Dialog>
    </Show>
  );
};
