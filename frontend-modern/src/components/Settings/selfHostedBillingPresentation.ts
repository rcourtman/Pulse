export interface SelfHostedProBillingPresentation {
  shellTitle: string;
  shellDescription: string;
  infrastructureRouteReferral: string;
  infrastructureWorkspaceReferral: string;
  refreshLabel: string;
  planSectionTitle: string;
  planSectionDescription: string;
  usageSectionTitle: string;
}

export const SELF_HOSTED_PRO_BILLING_PRESENTATION: SelfHostedProBillingPresentation = {
  shellTitle: 'Pulse Pro',
  shellDescription:
    'Manage self-hosted billing, monitored-system limits, and Pulse Pro license status.',
  infrastructureRouteReferral: 'Billing and monitored-system limits live in Pulse Pro.',
  infrastructureWorkspaceReferral:
    'Billing, monitored-system limits, and Pulse Pro license status live in Pulse Pro, not here.',
  refreshLabel: 'Refresh',
  planSectionTitle: 'Plan',
  planSectionDescription: 'Review your active plan, expiry, included limits, and paid capabilities.',
  usageSectionTitle: 'Usage',
};
