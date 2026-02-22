import { describe, expect, it, vi, beforeEach } from 'vitest';
import { OrgsAPI } from '../orgs';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('OrgsAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('list', () => {
    it('fetches all organizations', async () => {
      const mockOrgs = [{ id: 'org-1', displayName: 'Org 1' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockOrgs);

      const result = await OrgsAPI.list();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/orgs', { skipOrgContext: true });
      expect(result).toEqual(mockOrgs);
    });
  });

  describe('create', () => {
    it('creates a new organization', async () => {
      const newOrg = { id: 'org-new', displayName: 'New Org' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(newOrg);

      const result = await OrgsAPI.create({ id: 'org-new', displayName: 'New Org' });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/orgs',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ id: 'org-new', displayName: 'New Org' }),
          skipOrgContext: true,
        })
      );
      expect(result).toEqual(newOrg);
    });
  });

  describe('get', () => {
    it('fetches a single organization', async () => {
      const mockOrg = { id: 'org-1', displayName: 'Org 1' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockOrg);

      const result = await OrgsAPI.get('org-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/orgs/org-1', { skipOrgContext: true });
      expect(result).toEqual(mockOrg);
    });
  });

  describe('update', () => {
    it('updates organization display name', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({ id: 'org-1', displayName: 'Updated' });

      await OrgsAPI.update('org-1', { displayName: 'Updated' });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/orgs/org-1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ displayName: 'Updated' }),
          skipOrgContext: true,
        })
      );
    });
  });

  describe('remove', () => {
    it('deletes an organization', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await OrgsAPI.remove('org-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/orgs/org-1',
        expect.objectContaining({ method: 'DELETE', skipOrgContext: true })
      );
    });
  });

  describe('listMembers', () => {
    it('fetches organization members', async () => {
      const mockMembers = [{ userId: 'user-1', role: 'admin' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockMembers);

      const result = await OrgsAPI.listMembers('org-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/orgs/org-1/members', { skipOrgContext: true });
      expect(result).toEqual(mockMembers);
    });
  });

  describe('inviteMember', () => {
    it('invites a member to organization', async () => {
      const mockMember = { userId: 'user-1', role: 'viewer' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockMember);

      const result = await OrgsAPI.inviteMember('org-1', { userId: 'user-1', role: 'viewer' });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/orgs/org-1/members',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ userId: 'user-1', role: 'viewer' }),
          skipOrgContext: true,
        })
      );
      expect(result).toEqual(mockMember);
    });
  });

  describe('removeMember', () => {
    it('removes a member from organization', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await OrgsAPI.removeMember('org-1', 'user-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/orgs/org-1/members/user-1',
        expect.objectContaining({ method: 'DELETE', skipOrgContext: true })
      );
    });
  });

  describe('listShares', () => {
    it('fetches organization shares', async () => {
      const mockShares = [{ id: 'share-1', targetOrgId: 'org-2', resourceType: 'alert' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockShares);

      const result = await OrgsAPI.listShares('org-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/orgs/org-1/shares', { skipOrgContext: true });
      expect(result).toEqual(mockShares);
    });
  });

  describe('listIncomingShares', () => {
    it('fetches incoming organization shares', async () => {
      const mockShares = [{ id: 'share-1', sourceOrgId: 'org-2' }];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockShares);

      const result = await OrgsAPI.listIncomingShares('org-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/orgs/org-1/shares/incoming', { skipOrgContext: true });
      expect(result).toEqual(mockShares);
    });
  });

  describe('createShare', () => {
    it('creates a resource share', async () => {
      const mockShare = { id: 'share-1', targetOrgId: 'org-2', resourceType: 'alert' };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockShare);

      const result = await OrgsAPI.createShare('org-1', {
        targetOrgId: 'org-2',
        resourceType: 'alert',
        resourceId: 'resource-1',
        accessRole: 'viewer',
      });

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/orgs/org-1/shares',
        expect.objectContaining({
          method: 'POST',
          skipOrgContext: true,
        })
      );
      expect(result).toEqual(mockShare);
    });
  });

  describe('deleteShare', () => {
    it('deletes a resource share', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(undefined);

      await OrgsAPI.deleteShare('org-1', 'share-1');

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/orgs/org-1/shares/share-1',
        expect.objectContaining({ method: 'DELETE', skipOrgContext: true })
      );
    });
  });
});
