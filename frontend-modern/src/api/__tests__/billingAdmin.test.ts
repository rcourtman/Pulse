import { describe, expect, it, vi, beforeEach } from 'vitest';
import { BillingAdminAPI, type HostedOrganizationSummary, type BillingState } from '../billingAdmin';
import { apiFetchJSON } from '@/utils/apiClient';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

describe('BillingAdminAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('listOrganizations', () => {
    it('fetches hosted organizations', async () => {
      const mockOrgs: HostedOrganizationSummary[] = [
        { org_id: 'org-1', display_name: 'Org 1', owner_user_id: 'user-1', created_at: '2024-01-01', suspended: false, soft_deleted: false },
      ];
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockOrgs);

      const result = await BillingAdminAPI.listOrganizations();

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/hosted/organizations', { skipOrgContext: true });
      expect(result).toEqual(mockOrgs);
    });
  });

  describe('getBillingState', () => {
    it('fetches billing state for organization', async () => {
      const mockState: BillingState = {
        capabilities: ['feature-a'],
        limits: { users: 10 },
        subscription_state: 'active',
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockState);

      const result = await BillingAdminAPI.getBillingState('org-1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/orgs/org-1/billing-state', { skipOrgContext: true });
      expect(result).toEqual(mockState);
    });

    it('encodes special characters in org ID', async () => {
      vi.mocked(apiFetchJSON).mockResolvedValueOnce({} as BillingState);

      await BillingAdminAPI.getBillingState('org/1');

      expect(apiFetchJSON).toHaveBeenCalledWith('/api/admin/orgs/org%2F1/billing-state', { skipOrgContext: true });
    });
  });

  describe('putBillingState', () => {
    it('updates billing state for organization', async () => {
      const mockState: BillingState = {
        capabilities: ['feature-a'],
        limits: { users: 20 },
        subscription_state: 'active',
      };
      vi.mocked(apiFetchJSON).mockResolvedValueOnce(mockState);

      const result = await BillingAdminAPI.putBillingState('org-1', mockState);

      expect(apiFetchJSON).toHaveBeenCalledWith(
        '/api/admin/orgs/org-1/billing-state',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify(mockState),
          skipOrgContext: true,
        })
      );
      expect(result).toEqual(mockState);
    });
  });
});
