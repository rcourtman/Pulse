import { Show, type Component } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { PulseDataGrid } from '@/components/shared/PulseDataGrid';
import { useBreakpoint } from '@/hooks/useBreakpoint';
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
  billingErrorByOrgID: Record<string, string>;
  canManageBilling: boolean;
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
) => {
  const { isMobile } = useBreakpoint();
  const unreadLabel = (orgID: string) =>
    props.billingLoadingByOrgID[orgID] ? 'Loading' : 'Unavailable';
  const actionsDisabled = (orgID: string) =>
    !props.canManageBilling ||
    !props.billingByOrgID[orgID] ||
    !!props.billingErrorByOrgID[orgID] ||
    props.savingByOrgID[orgID] ||
    props.billingLoadingByOrgID[orgID];
  const subscriptionActions = (orgID: string) => {
    const currentSubState = () =>
      (props.billingByOrgID[orgID]?.subscription_state || '').toLowerCase();
    return (
      <div class="flex flex-col sm:inline-flex sm:flex-row sm:items-center gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => props.onSuspendOrganization(orgID)}
          disabled={actionsDisabled(orgID) || currentSubState() === 'suspended'}
        >
          Suspend Org
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => props.onActivateOrganization(orgID)}
          disabled={actionsDisabled(orgID) || currentSubState() === 'active'}
        >
          Activate Org
        </Button>
      </div>
    );
  };
  return (
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
                class="text-left w-full whitespace-normal [overflow-wrap:anywhere]"
                onClick={() => {
                  props.onToggleOrganization(orgID);
                }}
              >
                <div class="font-medium text-base-content">{org.display_name || org.org_id}</div>
                <div class="text-xs text-muted">
                  <span class="font-mono">{org.org_id}</span>
                  {getBillingAdminOrganizationBadges(org).map((badge) => (
                    <span class={`ml-2 rounded-sm px-1.5 py-0.5 ${badge.badgeClass}`}>
                      {badge.label}
                    </span>
                  ))}
                </div>
              </button>
            );
          },
        },
        {
          key: 'owner',
          label: 'Owner',
          hiddenOnMobile: true,
          render: (org) => (
            <span class="font-mono text-xs text-base-content">{org.owner_user_id || 'N/A'}</span>
          ),
        },
        {
          key: 'subscription',
          label: 'Subscription',
          render: (org) => {
            const billing = props.billingByOrgID[(org.org_id || '').trim()];
            const orgID = (org.org_id || '').trim();
            return (
              <div class="space-y-2">
                <span class="font-mono text-xs text-base-content">
                  {billing
                    ? getLicenseSubscriptionStatusPresentation(billing.subscription_state).label
                    : unreadLabel(orgID)}
                </span>
                <Show when={props.billingErrorByOrgID[orgID]}>
                  <p
                    role="alert"
                    class="max-w-xs whitespace-normal [overflow-wrap:anywhere] text-xs text-red-700 dark:text-red-300"
                  >
                    {props.billingErrorByOrgID[orgID]}
                  </p>
                  <Button
                    variant="secondary"
                    size="xs"
                    aria-label={`Reload billing for ${org.display_name || orgID}`}
                    onClick={() => props.onReloadOrganization(orgID)}
                    disabled={props.billingLoadingByOrgID[orgID] || props.savingByOrgID[orgID]}
                  >
                    Reload billing
                  </Button>
                </Show>
                <Show when={isMobile()}>{subscriptionActions(orgID)}</Show>
              </div>
            );
          },
        },
        {
          key: 'trial',
          label: 'Trial',
          hiddenOnMobile: true,
          render: (org) => {
            const billing = props.billingByOrgID[(org.org_id || '').trim()];
            return (
              <span class="text-xs text-base-content">
                {billing ? getBillingAdminTrialStatus(billing) : unreadLabel(org.org_id)}
              </span>
            );
          },
        },
        {
          key: 'stripeCustomer',
          label: 'Stripe Customer',
          hiddenOnMobile: true,
          render: (org) => {
            const billing = props.billingByOrgID[(org.org_id || '').trim()];
            const cellValue = billing ? stripeCustomerCell(billing) : unreadLabel(org.org_id);
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
          hiddenOnMobile: true,
          align: 'right',
          render: (org) => <Show when={!isMobile()}>{subscriptionActions(org.org_id)}</Show>,
        },
      ]}
      keyExtractor={(org) => org.org_id}
      isRowExpanded={(org) => props.expandedOrgID === (org.org_id || '').trim()}
      expandedRender={(org) => {
        const orgID = (org.org_id || '').trim();
        return (
          <div class="px-3 pb-3 bg-surface-alt whitespace-normal">
            <div class="rounded-md border border-border bg-surface-alt p-3">
              <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-2">
                <div class="text-xs font-semibold text-muted">Billing state JSON</div>
                <Button
                  variant="secondary"
                  size="xs"
                  disabled={props.billingLoadingByOrgID[orgID] || props.savingByOrgID[orgID]}
                  onClick={() => {
                    props.onReloadOrganization(orgID);
                  }}
                >
                  Reload
                </Button>
              </div>
              <Show
                when={props.billingByOrgID[orgID]}
                fallback={
                  <p role="status" class="text-sm text-muted">
                    {props.billingErrorByOrgID[orgID] || 'Loading billing state...'}
                  </p>
                }
              >
                <pre class="text-xs overflow-x-auto whitespace-pre-wrap [overflow-wrap:anywhere] font-mono text-base-content">
                  {JSON.stringify(props.billingByOrgID[orgID], null, 2)}
                </pre>
              </Show>
            </div>
          </div>
        );
      }}
    />
  );
};
