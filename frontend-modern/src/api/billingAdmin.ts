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

interface BillingAdminRequestOptions {
  signal?: AbortSignal;
}

export const BillingAdminAPI = {
  listOrganizations: (options?: BillingAdminRequestOptions) =>
    apiFetchJSON<HostedOrganizationSummary[]>('/api/hosted/organizations', {
      preferSessionAuth: true,
      skipOrgContext: true,
      ...options,
    }),

  getBillingState: (orgID: string, options?: BillingAdminRequestOptions) =>
    apiFetchJSON<BillingState>(`/api/admin/orgs/${encodeURIComponent(orgID)}/billing-state`, {
      preferSessionAuth: true,
      skipOrgContext: true,
      ...options,
    }),

  putBillingState: (orgID: string, payload: BillingState, options?: BillingAdminRequestOptions) =>
    apiFetchJSON<BillingState>(`/api/admin/orgs/${encodeURIComponent(orgID)}/billing-state`, {
      method: 'PUT',
      body: JSON.stringify(payload),
      preferSessionAuth: true,
      skipOrgContext: true,
      ...options,
      // A failed response does not establish whether a subscription changed.
      // Also recheck cancellation after the client's CSRF preparation wait.
      retry: false,
    }),
};
