import { describe, expect, it } from 'vitest';
import type { OrganizationRole, OrganizationShareStatus } from '@/api/orgs';
import {
  getOrganizationAccessInvitationAcceptedMessage,
  getOrganizationAccessInvitationDeclinedMessage,
  getOrganizationAccessInvitationRevokedMessage,
  getOrganizationAccessInvitationSentMessage,
  getOrganizationAccessOwnerTransferMemberRequiredMessage,
  getOrganizationAccessPendingInvitationsEmptyState,
  getOrganizationAccessRoleUpdatedMessage,
  getOrganizationAccessYourInvitationsEmptyState,
  getOrganizationAddMemberErrorMessage,
  getOrganizationDisplayNameUpdateErrorMessage,
  getOrganizationIncomingShareAcceptErrorMessage,
  getOrganizationIncomingShareDeclineConfirmMessage,
  getOrganizationIncomingShareDeclineErrorMessage,
  getOrganizationIncomingShareDeclineSuccessMessage,
  getOrganizationInvitationActionErrorMessage,
  getOrganizationMemberRoleUpdateErrorMessage,
  getOrganizationRemoveMemberErrorMessage,
  getOrganizationSettingsLoadErrorMessage,
  getOrganizationShareCreateErrorMessage,
  getOrganizationShareDeleteErrorMessage,
  getOrganizationShareStatusDescription,
  getOrganizationShareStatusLabel,
} from '@/utils/organizationSettingsPresentation';

// Supplemental branch coverage. The primary test exercises the happy path for
// most helpers but leaves the arms below open: the two-`if` precedence in the
// load-error helper, every fully-uncovered invitation helper, the `message ||`
// truthy/falsy arms of several error helpers, and the acceptedAt/acceptedBy
// branch matrix of getOrganizationShareStatusDescription.

describe('organizationSettingsPresentation branch coverage (supplemental)', () => {
  describe('getOrganizationSettingsLoadErrorMessage', () => {
    it('honours the 402 check before the 501 check when a message carries both tokens', () => {
      // Two sequential `if (message.includes(...))` guards: the 402 arm is checked
      // first, so a message containing both substrings must resolve to the license
      // message, never the not-enabled message.
      expect(
        getOrganizationSettingsLoadErrorMessage('gateway returned 402 then 501', 'sharing'),
      ).toBe('Organization settings require an Enterprise license.');
    });
  });

  describe('getOrganizationAccessPendingInvitationsEmptyState', () => {
    it('returns the canonical pending-invitations empty state', () => {
      expect(getOrganizationAccessPendingInvitationsEmptyState()).toBe(
        'No pending invitations for this organization.',
      );
    });
  });

  describe('getOrganizationAccessYourInvitationsEmptyState', () => {
    it('returns the canonical your-invitations empty state', () => {
      expect(getOrganizationAccessYourInvitationsEmptyState()).toBe(
        'No invitations are waiting for you.',
      );
    });
  });

  describe('getOrganizationAccessOwnerTransferMemberRequiredMessage', () => {
    it('returns the canonical owner-transfer validation message', () => {
      expect(getOrganizationAccessOwnerTransferMemberRequiredMessage()).toBe(
        'Ownership can only be transferred to an existing member.',
      );
    });
  });

  describe('getOrganizationAccessInvitationSentMessage', () => {
    it('formats the invitation-sent copy for every non-owner role', () => {
      // role is typed `Exclude<OrganizationRole, 'owner'>`; exercise each arm.
      const roles: Exclude<OrganizationRole, 'owner'>[] = ['admin', 'editor', 'viewer'];
      for (const role of roles) {
        expect(getOrganizationAccessInvitationSentMessage('bob', role)).toBe(
          `Sent bob an invitation for the ${role} role.`,
        );
      }
    });
  });

  describe('getOrganizationAccessInvitationAcceptedMessage', () => {
    it('formats the invitation-accepted copy', () => {
      expect(getOrganizationAccessInvitationAcceptedMessage('carol')).toBe(
        'carol joined the organization.',
      );
    });
  });

  describe('getOrganizationAccessInvitationDeclinedMessage', () => {
    it('formats the invitation-declined copy with the org id', () => {
      expect(getOrganizationAccessInvitationDeclinedMessage('org-42')).toBe(
        'Declined the invitation for org-42.',
      );
    });
  });

  describe('getOrganizationAccessInvitationRevokedMessage', () => {
    it('formats the invitation-revoked copy with the user id', () => {
      expect(getOrganizationAccessInvitationRevokedMessage('dave')).toBe(
        "Revoked dave's pending invitation.",
      );
    });
  });

  describe('getOrganizationAccessRoleUpdatedMessage', () => {
    it('formats the role-updated copy for the owner and editor roles the primary test omits', () => {
      expect(getOrganizationAccessRoleUpdatedMessage('alice', 'owner')).toBe(
        'Updated alice to the owner role.',
      );
      expect(getOrganizationAccessRoleUpdatedMessage('alice', 'editor')).toBe(
        'Updated alice to the editor role.',
      );
    });
  });

  describe('getOrganizationInvitationActionErrorMessage', () => {
    it('returns the provided message on the truthy arm of `message ||`', () => {
      expect(getOrganizationInvitationActionErrorMessage('invitation already revoked')).toBe(
        'invitation already revoked',
      );
    });

    it('returns the fallback on the falsy arm when message is undefined', () => {
      expect(getOrganizationInvitationActionErrorMessage()).toBe(
        'Unable to update the invitation.',
      );
    });

    it('treats an empty-string message as falsy and falls back', () => {
      expect(getOrganizationInvitationActionErrorMessage('')).toBe(
        'Unable to update the invitation.',
      );
    });
  });

  describe('getOrganizationIncomingShareAcceptErrorMessage', () => {
    it('returns the provided message on the truthy arm of `message ||`', () => {
      expect(getOrganizationIncomingShareAcceptErrorMessage('target rejected')).toBe(
        'target rejected',
      );
    });

    it('treats an empty-string message as falsy and falls back', () => {
      expect(getOrganizationIncomingShareAcceptErrorMessage('')).toBe(
        'Unable to accept the incoming share.',
      );
    });
  });

  describe('getOrganizationIncomingShareDeclineErrorMessage', () => {
    it('returns the provided message on the truthy arm of `message ||`', () => {
      expect(getOrganizationIncomingShareDeclineErrorMessage('still in use')).toBe(
        'still in use',
      );
    });

    it('treats an empty-string message as falsy and falls back', () => {
      expect(getOrganizationIncomingShareDeclineErrorMessage('')).toBe(
        'Unable to remove the incoming share.',
      );
    });
  });

  describe('error helpers — empty-string falsy arm of `message || fallback`', () => {
    // The primary test only feeds `undefined` for these, hitting the same falsy
    // arm; exercising the empty-string variant pins that any falsy message
    // (not only undefined) routes to the fallback.
    it('getOrganizationDisplayNameUpdateErrorMessage falls back for an empty string', () => {
      expect(getOrganizationDisplayNameUpdateErrorMessage('')).toBe(
        'Unable to update the organization name.',
      );
    });

    it('getOrganizationMemberRoleUpdateErrorMessage falls back for an empty string', () => {
      expect(getOrganizationMemberRoleUpdateErrorMessage('')).toBe(
        'Unable to update the member role.',
      );
    });

    it('getOrganizationAddMemberErrorMessage falls back for an empty string', () => {
      expect(getOrganizationAddMemberErrorMessage('')).toBe('Unable to add the member.');
    });

    it('getOrganizationRemoveMemberErrorMessage falls back for an empty string', () => {
      expect(getOrganizationRemoveMemberErrorMessage('')).toBe('Unable to remove the member.');
    });

    it('getOrganizationShareCreateErrorMessage falls back for an empty string', () => {
      expect(getOrganizationShareCreateErrorMessage('')).toBe('Unable to create the share.');
    });

    it('getOrganizationShareDeleteErrorMessage falls back for an empty string', () => {
      expect(getOrganizationShareDeleteErrorMessage('')).toBe('Unable to remove the share.');
    });
  });

  describe('getOrganizationShareStatusLabel', () => {
    it('routes any non-pending status (including an unrecognised value) to the else arm "Active"', () => {
      // Ternary `status === 'pending' ? 'Pending approval' : 'Active'`: a
      // deliberately invalid status must take the else arm.
      expect(
        getOrganizationShareStatusLabel('rejected' as unknown as OrganizationShareStatus),
      ).toBe('Active');
    });
  });

  describe('getOrganizationShareStatusDescription', () => {
    it('keeps the pending copy when status is pending even if accepted metadata is supplied', () => {
      // status === 'pending' short-circuits before the acceptedAt/acceptedBy logic.
      expect(
        getOrganizationShareStatusDescription('pending', '2026-04-22T10:30:00Z', 'alice'),
      ).toBe('Waiting for a target organization admin to accept.');
    });

    it('uses formatOrgDate(acceptedAt) and omits the "by ..." segment when acceptedBy is absent', () => {
      // acceptedAt truthy arm + acceptedBy falsy arm. Use a value formatOrgDate
      // returns verbatim (Invalid Date) so the assertion is deterministic.
      expect(getOrganizationShareStatusDescription('accepted', 'not-a-real-date')).toBe(
        'Accepted not-a-real-date.',
      );
    });

    it('uses formatOrgDate(acceptedAt) and appends "by <acceptedBy>" when acceptedBy is set', () => {
      // acceptedAt truthy arm + acceptedBy truthy arm with a real (locale-formatted)
      // date: assert structure rather than the locale-dependent date text.
      const description = getOrganizationShareStatusDescription(
        'accepted',
        '2026-04-22T10:30:00Z',
        'alice',
      );
      expect(description).toMatch(/^Accepted .+ by alice\.$/);
      expect(description).not.toContain('the target organization');
    });

    it('falls back to the generic "Accepted by the target organization" when acceptedAt is undefined', () => {
      // acceptedAt falsy arm + acceptedBy truthy arm.
      expect(getOrganizationShareStatusDescription('accepted', undefined, 'alice')).toBe(
        'Accepted by the target organization by alice.',
      );
    });

    it('omits the "by ..." segment entirely when both acceptedAt and acceptedBy are undefined', () => {
      // acceptedAt falsy arm + acceptedBy falsy arm.
      expect(getOrganizationShareStatusDescription('accepted')).toBe(
        'Accepted by the target organization.',
      );
    });

    it('treats an empty-string acceptedAt as falsy', () => {
      // acceptedAt is `''` (falsy) -> generic accepted copy, no "by" segment.
      expect(getOrganizationShareStatusDescription('accepted', '')).toBe(
        'Accepted by the target organization.',
      );
    });
  });

  describe('getOrganizationIncomingShareDeclineConfirmMessage', () => {
    it('routes any non-pending status (including an unrecognised value) to the remove arm', () => {
      // `status === 'pending' ? ... : ...` else arm via an invalid status.
      expect(
        getOrganizationIncomingShareDeclineConfirmMessage(
          'High CPU',
          'expired' as unknown as OrganizationShareStatus,
        ),
      ).toBe('Remove shared access to High CPU?');
    });
  });

  describe('getOrganizationIncomingShareDeclineSuccessMessage', () => {
    it('routes any non-pending status (including an unrecognised value) to the removed arm', () => {
      // `status === 'pending' ? ... : ...` else arm via an invalid status.
      expect(
        getOrganizationIncomingShareDeclineSuccessMessage(
          'High CPU',
          'expired' as unknown as OrganizationShareStatus,
        ),
      ).toBe('Removed shared access to High CPU.');
    });
  });
});
