import type { Component } from 'solid-js';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import {
  BILLING_ADMIN_EMPTY_STATE,
  getBillingAdminOrganizationBadges,
  getBillingAdminTrialStatus,
  getLicenseSubscriptionStatusPresentation,
} from '@/utils/licensePresentation';
import type { BillingState, HostedOrganizationSummary } from '@/api/billingAdmin';
import type { BillingStateCache } from './useBillingAdminPanelState';

interface BillingAdminOrganizationsTableProps {
  billingByOrgID: BillingStateCache;
  billingLoadingByOrgID: Record<string, boolean>;
  expandedOrgID: string | null;
  isLoading: boolean;
  orgs: HostedOrganizationSummary[];
  onActivateOrganization: (orgID: string) => void;
  onReloadOrganization: (orgID: string) => void;
  onSuspendOrganization: (orgID: string) => void;
  onToggleOrganization: (orgID: string) => void;
  savingByOrgID: Record<string, boolean>;
}

const stripeCustomerCell = (state?: BillingState) => {
  const value = (state?.stripe_customer_id || '').trim();
  if (!value) return 'N/A';
  return value;
};

export const BillingAdminOrganizationsTable: Component<BillingAdminOrganizationsTableProps> = (
  props,
) => (
  <PulseDataGrid
    data={props.orgs}
    isLoading={props.isLoading}
    emptyState={BILLING_ADMIN_EMPTY_STATE}
    desktopMinWidth="920px"
    columns={[
      {
        key: 'organization',
        label: 'Organization',
        render: (org) => {
          const orgID = (org.org_id || '').trim();
          return (
            <button
              type="button"
              class="text-left w-full"
              onClick={() => {
                props.onToggleOrganization(orgID);
              }}
            >
              <div class="font-medium text-base-content">{org.display_name || org.org_id}</div>
              <div class="text-xs text-muted">
                <span class="font-mono">{org.org_id}</span>
                {getBillingAdminOrganizationBadges(org).map((badge) => (
                  <span class={`ml-2 rounded px-1.5 py-0.5 ${badge.badgeClass}`}>{badge.label}</span>
                ))}
              </div>
            </button>
          );
        },
      },
      {
        key: 'owner',
        label: 'Owner',
        render: (org) => (
          <span class="font-mono text-xs text-base-content">{org.owner_user_id || 'N/A'}</span>
        ),
      },
      {
        key: 'subscription',
        label: 'Subscription',
        render: (org) => {
          const billing = props.billingByOrgID[(org.org_id || '').trim()];
          return (
            <span class="font-mono text-xs text-base-content">
              {getLicenseSubscriptionStatusPresentation(billing?.subscription_state).label}
            </span>
          );
        },
      },
      {
        key: 'trial',
        label: 'Trial',
        render: (org) => {
          const billing = props.billingByOrgID[(org.org_id || '').trim()];
          return <span class="text-xs text-base-content">{getBillingAdminTrialStatus(billing)}</span>;
        },
      },
      {
        key: 'stripeCustomer',
        label: 'Stripe Customer',
        render: (org) => {
          const billing = props.billingByOrgID[(org.org_id || '').trim()];
          const cellValue = stripeCustomerCell(billing);
          return (
            <span class="font-mono text-xs text-base-content" title={cellValue}>
              {cellValue}
            </span>
          );
        },
      },
      {
        key: 'actions',
        label: 'Actions',
        align: 'right',
        render: (org) => {
          const orgID = (org.org_id || '').trim();
          const billing = props.billingByOrgID[orgID];
          const currentSubState = (billing?.subscription_state || '').toLowerCase() || 'unknown';
          return (
            <div class="inline-flex flex-col sm:flex-row sm:items-center gap-2">
              <button
                type="button"
                onClick={() => {
                  props.onSuspendOrganization(orgID);
                }}
                disabled={
                  props.savingByOrgID[orgID] ||
                  props.billingLoadingByOrgID[orgID] ||
                  currentSubState === 'suspended'
                }
                class="px-2.5 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50"
              >
                Suspend Org
              </button>
              <button
                type="button"
                onClick={() => {
                  props.onActivateOrganization(orgID);
                }}
                disabled={
                  props.savingByOrgID[orgID] ||
                  props.billingLoadingByOrgID[orgID] ||
                  currentSubState === 'active'
                }
                class="px-2.5 py-1.5 text-xs font-medium rounded-md border border-border bg-surface hover:bg-surface-hover disabled:opacity-50"
              >
                Activate Org
              </button>
            </div>
          );
        },
      },
    ]}
    keyExtractor={(org) => org.org_id}
    isRowExpanded={(org) => props.expandedOrgID === (org.org_id || '').trim()}
    expandedRender={(org) => {
      const orgID = (org.org_id || '').trim();
      return (
        <div class="px-3 pb-3 bg-surface-alt">
          <div class="rounded-md border border-border bg-surface-alt p-3">
            <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-2">
              <div class="text-xs font-semibold text-muted">Billing state JSON</div>
              <button
                type="button"
                onClick={() => {
                  props.onReloadOrganization(orgID);
                }}
                class="px-2 py-1 text-xs rounded-md border border-border bg-surface hover:bg-surface-hover"
              >
                Reload
              </button>
            </div>
            <pre class="text-xs overflow-x-auto whitespace-pre-wrap font-mono text-base-content">
              {JSON.stringify(props.billingByOrgID[orgID] ?? { loading: true }, null, 2)}
            </pre>
          </div>
        </div>
      );
    }}
  />
);
