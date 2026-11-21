import type { Organization, OrganizationRole } from '@/api/orgs';
import { Component, For, Show, createMemo, createUniqueId } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { FormSelect } from '@/components/shared/FormSelect';
import { ORGANIZATION_MEMBER_ROLE_OPTIONS } from '@/utils/organizationRolePresentation';
import { getOrganizationAccessManageRequiredMessage } from '@/utils/organizationSettingsPresentation';
import { DEFAULT_ORG_SCOPE } from '@/utils/orgScope';
import type { useOrganizationAccessPanelState } from './useOrganizationAccessPanelState';

// Adding someone to the default organization grants access at once, and its
// members are viewers: instance roles decide what they can do.
const isDefaultOrg = (org: Organization) => org.id === DEFAULT_ORG_SCOPE;

interface OrganizationAccessManagementSectionProps {
  currentUser?: string;
  state: ReturnType<typeof useOrganizationAccessPanelState>;
}

export const OrganizationAccessManagementSection: Component<
  OrganizationAccessManagementSectionProps
> = (props) => {
  const effectID = `member-effect-${createUniqueId()}`;
  const memberAction = createMemo(() => {
    const org = props.state.org();
    if (org?.id === DEFAULT_ORG_SCOPE) {
      return {
        name: 'Add member',
        heading: 'Add Member',
        submit: 'Add',
        effect:
          'Adding grants access immediately. Viewer is the organization role. The user’s instance roles determine permissions.',
      };
    }
    const readerIsOwner = !!org?.ownerUserId && props.currentUser === org.ownerUserId;
    const targetIsOwner = props.state.inviteDraftIsOwner();
    const member = props.state.inviteDraftMember();
    if (props.state.inviteRole() === 'owner') {
      return {
        name: 'Review ownership transfer',
        heading: 'Transfer Ownership',
        submit: 'Review transfer',
        effect: !readerIsOwner
          ? 'Only the current owner can transfer ownership.'
          : targetIsOwner
            ? 'This member is already the owner. Enter another existing member’s user ID.'
            : 'Choose another existing member. You’ll review the transfer before it’s saved.',
        blocked: !readerIsOwner || targetIsOwner || !member,
      };
    }
    if (targetIsOwner) {
      return {
        name: 'Current owner',
        heading: 'Current Owner',
        submit: 'Update role',
        effect: readerIsOwner
          ? 'To change your role, transfer ownership to another existing member.'
          : 'Only the current owner can transfer ownership.',
        blocked: true,
      };
    }
    if (member) {
      const savedRole = ORGANIZATION_MEMBER_ROLE_OPTIONS.find(
        (option) => option.value === member.role,
      );
      return {
        name: 'Update member role',
        heading: 'Update Member Role',
        submit: 'Update role',
        effect: `${savedRole ? `Last read: ${savedRole.label}. ` : ''}Role changes take effect immediately, without an invitation.`,
      };
    }
    return {
      name: 'Invite member',
      heading: 'Invite Member',
      submit: 'Invite',
      effect:
        'New users must accept before they get access. Role changes for existing members take effect immediately.',
    };
  });

  return (
    <Show when={props.state.org()}>
      {(currentOrg) => (
        <>
          <Show when={props.state.canManageCurrentOrg()}>
            <form
              aria-label={memberAction().name}
              aria-describedby={effectID}
              onSubmit={(event) => {
                event.preventDefault();
                void props.state.inviteMember();
              }}
              class="rounded-md border border-border p-4 space-y-3"
            >
              <div class="space-y-1">
                <h4 class="text-sm font-semibold text-base-content">{memberAction().heading}</h4>
                <p id={effectID} class="text-xs text-muted" aria-live="polite">
                  {memberAction().effect}
                </p>
              </div>
              <div class="grid grid-cols-2 items-end gap-2 sm:grid-cols-[1fr_auto_auto]">
                <label class="col-span-2 min-w-0 space-y-1 sm:col-span-1">
                  <span class="block text-xs font-medium text-base-content">User ID</span>
                  <input
                    type="text"
                    required
                    data-org-access-action="invite-user"
                    data-org-access-target="draft"
                    aria-label={isDefaultOrg(currentOrg()) ? 'User ID to add' : 'Member user ID'}
                    aria-describedby={effectID}
                    value={props.state.inviteUserID()}
                    disabled={!!props.state.invitationReadIssue()}
                    onInput={(event) => props.state.setInviteUserID(event.currentTarget.value)}
                    placeholder="username"
                    class="w-full rounded-md border px-3 py-2 text-sm shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500"
                  />
                </label>
                <FormSelect
                  label="Role"
                  data-org-access-action="invite-role"
                  data-org-access-target="draft"
                  aria-label="Member role"
                  labelClass="text-xs"
                  fieldBaseClass="min-w-0 space-y-1"
                  value={props.state.inviteRole()}
                  disabled={!!props.state.invitationReadIssue()}
                  onChange={(event) =>
                    props.state.setInviteRole(event.currentTarget.value as OrganizationRole)
                  }
                  selectBaseClass="rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500"
                >
                  <For
                    each={ORGANIZATION_MEMBER_ROLE_OPTIONS.filter((option) =>
                      isDefaultOrg(currentOrg())
                        ? option.value === 'viewer'
                        : option.value !== 'owner' ||
                          currentOrg().ownerUserId === props.currentUser ||
                          props.state.inviteRole() === 'owner',
                    )}
                  >
                    {(option) => (
                      <option
                        value={option.value}
                        disabled={
                          option.value === 'owner' && currentOrg().ownerUserId !== props.currentUser
                        }
                      >
                        {option.value === 'owner' && currentOrg().ownerUserId !== props.currentUser
                          ? 'Owner (current owner only)'
                          : option.label}
                      </option>
                    )}
                  </For>
                </FormSelect>
                <Button
                  type="submit"
                  aria-describedby={effectID}
                  data-org-access-action="invite-submit"
                  data-org-access-target="draft"
                  variant="primary"
                  size="settingsAction"
                  class="w-full sm:w-auto"
                  disabled={!props.state.canChangeAccess() || memberAction().blocked}
                >
                  {props.state.saving() ? 'Saving...' : memberAction().submit}
                </Button>
              </div>
            </form>
          </Show>

          <Show when={!props.state.canManageCurrentOrg()}>
            <div class="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/25 dark:text-amber-300">
              {getOrganizationAccessManageRequiredMessage(props.state.readOnly())}
            </div>
          </Show>
        </>
      )}
    </Show>
  );
};
