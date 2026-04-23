import { apiFetchJSON } from '@/utils/apiClient';

export type OrganizationRole = 'owner' | 'admin' | 'editor' | 'viewer';
export type OrganizationShareStatus = 'pending' | 'accepted';

export interface OrganizationMember {
  userId: string;
  role: OrganizationRole;
  addedAt: string;
  addedBy?: string;
}

export interface OrganizationInvitation {
  userId: string;
  role: Exclude<OrganizationRole, 'owner'>;
  invitedAt: string;
  invitedBy: string;
}

export type OrganizationAccessMutationResult =
  | {
      kind: 'member';
      member: OrganizationMember;
      invitation?: never;
    }
  | {
      kind: 'invitation';
      invitation: OrganizationInvitation;
      member?: never;
    };

export interface UserOrganizationInvitation extends OrganizationInvitation {
  orgId: string;
  orgDisplayName: string;
}

export interface Organization {
  id: string;
  displayName: string;
  createdAt?: string;
  ownerUserId?: string;
  members?: OrganizationMember[];
}

export interface OrganizationShare {
  id: string;
  targetOrgId: string;
  resourceType: string;
  resourceId: string;
  resourceName?: string;
  accessRole: Exclude<OrganizationRole, 'owner'> | 'viewer';
  status: OrganizationShareStatus;
  createdAt: string;
  createdBy: string;
  acceptedAt?: string;
  acceptedBy?: string;
}

export interface IncomingOrganizationShare extends OrganizationShare {
  sourceOrgId: string;
  sourceOrgName: string;
}

export const OrgsAPI = {
  list: () =>
    apiFetchJSON<Organization[]>('/api/orgs', {
      skipOrgContext: true,
    }),

  create: (payload: { id: string; displayName: string }) =>
    apiFetchJSON<Organization>('/api/orgs', {
      method: 'POST',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  get: (id: string) =>
    apiFetchJSON<Organization>(`/api/orgs/${encodeURIComponent(id)}`, {
      skipOrgContext: true,
    }),

  update: (id: string, payload: { displayName: string }) =>
    apiFetchJSON<Organization>(`/api/orgs/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  remove: (id: string) =>
    apiFetchJSON<void>(`/api/orgs/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      skipOrgContext: true,
    }),

  listMembers: (id: string) =>
    apiFetchJSON<OrganizationMember[]>(`/api/orgs/${encodeURIComponent(id)}/members`, {
      skipOrgContext: true,
    }),

  listPendingInvitations: (id: string) =>
    apiFetchJSON<OrganizationInvitation[]>(`/api/orgs/${encodeURIComponent(id)}/invitations`, {
      skipOrgContext: true,
    }),

  listMyInvitations: () =>
    apiFetchJSON<UserOrganizationInvitation[]>('/api/org-invitations', {
      skipOrgContext: true,
    }),

  inviteMember: (id: string, payload: { userId: string; role: OrganizationRole }) =>
    apiFetchJSON<OrganizationAccessMutationResult>(`/api/orgs/${encodeURIComponent(id)}/members`, {
      method: 'POST',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  acceptMyInvitation: (id: string) =>
    apiFetchJSON<OrganizationAccessMutationResult>(
      `/api/org-invitations/${encodeURIComponent(id)}/accept`,
      {
        method: 'POST',
        skipOrgContext: true,
      },
    ),

  declineMyInvitation: (id: string) =>
    apiFetchJSON<void>(`/api/org-invitations/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      skipOrgContext: true,
    }),

  updateMemberRole: (id: string, payload: { userId: string; role: OrganizationRole }) =>
    apiFetchJSON<OrganizationAccessMutationResult>(`/api/orgs/${encodeURIComponent(id)}/members`, {
      method: 'POST',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  revokeInvitation: (id: string, userId: string) =>
    apiFetchJSON<void>(
      `/api/orgs/${encodeURIComponent(id)}/invitations/${encodeURIComponent(userId)}`,
      {
        method: 'DELETE',
        skipOrgContext: true,
      },
    ),

  removeMember: (id: string, userId: string) =>
    apiFetchJSON<void>(
      `/api/orgs/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`,
      {
        method: 'DELETE',
        skipOrgContext: true,
      },
    ),

  listShares: (id: string) =>
    apiFetchJSON<OrganizationShare[]>(`/api/orgs/${encodeURIComponent(id)}/shares`, {
      skipOrgContext: true,
    }),

  listIncomingShares: (id: string) =>
    apiFetchJSON<IncomingOrganizationShare[]>(
      `/api/orgs/${encodeURIComponent(id)}/shares/incoming`,
      {
        skipOrgContext: true,
      },
    ),

  acceptIncomingShare: (id: string, shareId: string) =>
    apiFetchJSON<OrganizationShare>(
      `/api/orgs/${encodeURIComponent(id)}/shares/incoming/${encodeURIComponent(shareId)}/accept`,
      {
        method: 'POST',
        skipOrgContext: true,
      },
    ),

  declineIncomingShare: (id: string, shareId: string) =>
    apiFetchJSON<void>(
      `/api/orgs/${encodeURIComponent(id)}/shares/incoming/${encodeURIComponent(shareId)}`,
      {
        method: 'DELETE',
        skipOrgContext: true,
      },
    ),

  createShare: (
    id: string,
    payload: {
      targetOrgId: string;
      resourceType: string;
      resourceId: string;
      resourceName?: string;
      accessRole: Exclude<OrganizationRole, 'owner'> | 'viewer';
    },
  ) =>
    apiFetchJSON<OrganizationShare>(`/api/orgs/${encodeURIComponent(id)}/shares`, {
      method: 'POST',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  deleteShare: (id: string, shareId: string) =>
    apiFetchJSON<void>(
      `/api/orgs/${encodeURIComponent(id)}/shares/${encodeURIComponent(shareId)}`,
      {
        method: 'DELETE',
        skipOrgContext: true,
      },
    ),
};
