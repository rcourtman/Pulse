import { apiFetchJSON } from '@/utils/apiClient';

export type OrganizationRole = 'owner' | 'admin' | 'editor' | 'viewer' | 'member';

export interface OrganizationMember {
  userId: string;
  role: OrganizationRole;
  addedAt: string;
  addedBy?: string;
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
  accessRole: Exclude<OrganizationRole, 'owner' | 'member'> | 'viewer';
  createdAt: string;
  createdBy: string;
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

  inviteMember: (id: string, payload: { userId: string; role: OrganizationRole }) =>
    apiFetchJSON<OrganizationMember>(`/api/orgs/${encodeURIComponent(id)}/members`, {
      method: 'POST',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  updateMemberRole: (id: string, payload: { userId: string; role: OrganizationRole }) =>
    apiFetchJSON<OrganizationMember>(`/api/orgs/${encodeURIComponent(id)}/members`, {
      method: 'POST',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  removeMember: (id: string, userId: string) =>
    apiFetchJSON<void>(`/api/orgs/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`, {
      method: 'DELETE',
      skipOrgContext: true,
    }),

  listShares: (id: string) =>
    apiFetchJSON<OrganizationShare[]>(`/api/orgs/${encodeURIComponent(id)}/shares`, {
      skipOrgContext: true,
    }),

  listIncomingShares: (id: string) =>
    apiFetchJSON<IncomingOrganizationShare[]>(`/api/orgs/${encodeURIComponent(id)}/shares/incoming`, {
      skipOrgContext: true,
    }),

  createShare: (
    id: string,
    payload: {
      targetOrgId: string;
      resourceType: string;
      resourceId: string;
      resourceName?: string;
      accessRole: Exclude<OrganizationRole, 'owner' | 'member'> | 'viewer';
    },
  ) =>
    apiFetchJSON<OrganizationShare>(`/api/orgs/${encodeURIComponent(id)}/shares`, {
      method: 'POST',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),

  deleteShare: (id: string, shareId: string) =>
    apiFetchJSON<void>(`/api/orgs/${encodeURIComponent(id)}/shares/${encodeURIComponent(shareId)}`, {
      method: 'DELETE',
      skipOrgContext: true,
    }),
};
