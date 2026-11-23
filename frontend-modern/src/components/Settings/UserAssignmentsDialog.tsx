import { Component, createMemo, createUniqueId, For, Show } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { Button } from '@/components/shared/Button';
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
  savedRuleQualifier,
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
  actionErrorRef?: (element: HTMLDivElement) => void;
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
  onReloadPermissions: (restart: boolean) => void;
  onToggleRole: (roleId: string) => void;
}

function countRoleEntries(roleIds: readonly string[]) {
  const counts = new Map<string, number>();
  for (const id of roleIds) counts.set(id, (counts.get(id) ?? 0) + 1);
  return counts;
}

export const UserAssignmentsDialog: Component<UserAssignmentsDialogProps> = (props) => {
  const permissionsTitleID = createUniqueId();
  const savedRoleCounts = createMemo(() => countRoleEntries(props.editingUser?.roleIds ?? []));
  const draftRoleCounts = createMemo(() => countRoleEntries(props.formRoleIds));
  let savedPermissions: HTMLDivElement | undefined;
  const reloadSavedPermissions = () => {
    // Both controls withdraw as the read changes state. Keep keyboard review
    // at the stable result region, without refocusing when a response arrives.
    savedPermissions?.focus();
    props.onReloadPermissions(props.loadingPermissions);
  };
  const permissionsReadButton = () => (
    <Button
      variant="ghost"
      size="xs"
      onClick={reloadSavedPermissions}
      disabled={props.saving}
      class="shrink-0 underline underline-offset-2"
    >
      {props.loadingPermissions ? 'Restart saved permissions' : 'Retry saved permissions'}
    </Button>
  );
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
              disabled={props.saving}
              aria-label="Close user assignments"
              title="Close"
              class="shrink-0 p-1.5 rounded-md hover:text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
            >
              <X class="w-5 h-5" />
            </button>
          </div>

          <div class="min-h-0 overflow-y-auto px-4 py-3 space-y-3 sm:px-6 sm:py-6 sm:space-y-8">
            <Show when={props.actionError}>
              {(message) => (
                <InlineNotice
                  ref={props.actionErrorRef}
                  tabindex={-1}
                  tone="danger"
                  role="alert"
                  aria-live="polite"
                  class="focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                >
                  {message()}
                </InlineNotice>
              )}
            </Show>
            <div class="space-y-2 sm:space-y-4">
              <h4 class="text-sm font-semibold text-base-content flex items-center gap-2">
                <Shield class="w-4 h-4 text-blue-500" />
                Select Roles
              </h4>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 sm:gap-3">
                <For each={props.roles}>
                  {(role) => {
                    const changeId = createUniqueId();
                    const draftCount = () => draftRoleCounts().get(role.id) ?? 0;
                    const change = createMemo(() => {
                      const saved = savedRoleCounts().get(role.id) ?? 0;
                      const draft = draftCount();
                      if (draft === saved) return;
                      if (!saved) return 'Add on save';
                      if (!draft) return 'Remove on save';
                      // Turning a duplicated saved role off and back on keeps
                      // its membership but changes the submitted entry count.
                      return `Change role entries from ${saved} to ${draft} on save`;
                    });
                    return (
                      <label
                        class={`grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-start gap-x-2 p-2 sm:p-3 rounded-md border transition-all [overflow-wrap:anywhere] ${props.saving ? 'cursor-not-allowed' : 'cursor-pointer'} ${draftCount() ? 'bg-blue-50 border-blue-200 dark:bg-blue-900/25 dark:border-blue-800' : 'bg-surface border-border hover:border-blue-100 dark:hover:border-blue-900/25'}`}
                      >
                        <input
                          type="checkbox"
                          aria-label={`${getRoleIdentityLabel(role)} ${role.description}`.trim()}
                          aria-describedby={change() ? changeId : undefined}
                          checked={draftCount() > 0}
                          disabled={props.saving}
                          onChange={() => props.onToggleRole(role.id)}
                          class="mt-0.5 w-4 h-4 shrink-0 text-blue-600 rounded-sm border-border focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60"
                        />
                        <div class="min-w-0">
                          <div class="flex flex-wrap items-start justify-between gap-x-2 gap-y-0.5">
                            <RoleAccessIdentity
                              id={role.id}
                              name={role.name}
                              nameClass="text-sm font-semibold text-base-content"
                            />
                            <Show when={role.isBuiltIn || change()}>
                              <div class="flex min-w-0 flex-wrap items-center gap-1">
                                <Show when={role.isBuiltIn}>
                                  <BadgeCheck class="w-4 h-4 shrink-0 text-blue-500" />
                                </Show>
                                <Show when={change()}>
                                  <span
                                    id={changeId}
                                    class="rounded-sm border border-border bg-surface-alt px-1.5 text-xs font-medium text-base-content"
                                  >
                                    {change()}
                                  </span>
                                </Show>
                              </div>
                            </Show>
                          </div>
                          <p class="text-xs leading-4 text-muted sm:mt-1">{role.description}</p>
                        </div>
                      </label>
                    );
                  }}
                </For>
              </div>
            </div>

            <div class="space-y-3 pt-3 border-t border-border-subtle sm:space-y-4 sm:pt-4">
              <div class="flex items-center gap-2">
                <h4
                  id={permissionsTitleID}
                  class="mr-auto text-sm font-semibold text-base-content flex items-center gap-2"
                >
                  <BadgeCheck class="w-4 h-4 text-blue-500" />
                  Saved permissions
                </h4>
                <Show when={props.loadingPermissions}>
                  <LoadingSpinner size="md" tone="info" label="Loading saved permissions" />
                  {permissionsReadButton()}
                </Show>
              </div>
              <div
                ref={savedPermissions}
                role="region"
                aria-labelledby={permissionsTitleID}
                aria-busy={props.loadingPermissions}
                tabIndex={-1}
                class="bg-surface-hover rounded-md p-4 border border-border-subtle focus:outline-hidden focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              >
                <Show when={!props.loadingPermissions && props.permissionsError}>
                  {(message) => (
                    <InlineNotice tone="warning" role="alert" aria-live="polite">
                      {message()}
                      {permissionsReadButton()}
                    </InlineNotice>
                  )}
                </Show>
                <Show
                  when={
                    !props.loadingPermissions && !props.permissionsError && props.userPermissions
                  }
                >
                  {(permissions) => (
                    <Show
                      when={permissions().length}
                      fallback={
                        <p class="text-xs text-muted italic text-center py-2">
                          No permissions were returned for this user's saved roles.
                        </p>
                      }
                    >
                      <ul
                        role="list"
                        aria-label="Returned rules"
                        class="grid gap-0.5 sm:grid-cols-2"
                      >
                        <For each={permissions()}>
                          {(permission) => (
                            <li class="grid grid-cols-[2fr_3fr] gap-x-1 text-xs">
                              <bdi class="break-all text-blue-600 dark:text-blue-400">
                                {permission.action}:<span>{permission.resource}</span>
                              </bdi>{' '}
                              <span class="break-words">{savedRuleQualifier(permission)}</span>
                            </li>
                          )}
                        </For>
                      </ul>
                    </Show>
                  )}
                </Show>
                <p class="mt-3 text-xs text-muted sm:mt-4">
                  This summary can omit duplicate and inherited rules. Review individual rules in
                  Roles. Unsaved role selections are not included. This is not an access check for a
                  specific resource.
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
              disabled={props.saving}
              class="w-full sm:w-auto rounded-md px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover transition-colors disabled:cursor-not-allowed disabled:opacity-60"
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
