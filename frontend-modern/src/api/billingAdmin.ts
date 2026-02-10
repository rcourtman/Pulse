import { apiFetchJSON } from '@/utils/apiClient';

export interface HostedOrganizationSummary {
  org_id: string;
  display_name: string;
  owner_user_id: string;
  created_at: string;
  suspended: boolean;
  soft_deleted: boolean;
}

export interface BillingState {
  capabilities: string[];
  limits: Record<string, number>;
  meters_enabled?: string[];
  subscription_state: string;
  plan_version?: string;
  trial_started_at?: number | null;
  trial_ends_at?: number | null;
  stripe_customer_id?: string;
  stripe_subscription_id?: string;
  stripe_price_id?: string;
}

export const BillingAdminAPI = {
  listOrganizations: () =>
    apiFetchJSON<HostedOrganizationSummary[]>('/api/hosted/organizations', {
      skipOrgContext: true,
    }),

  getBillingState: (orgID: string) =>
    apiFetchJSON<BillingState>(`/api/admin/orgs/${encodeURIComponent(orgID)}/billing-state`, {
      skipOrgContext: true,
    }),

  putBillingState: (orgID: string, payload: BillingState) =>
    apiFetchJSON<BillingState>(`/api/admin/orgs/${encodeURIComponent(orgID)}/billing-state`, {
      method: 'PUT',
      body: JSON.stringify(payload),
      skipOrgContext: true,
    }),
};

