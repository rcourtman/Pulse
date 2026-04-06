export interface SelfHostedProBillingPresentation {
  shellTitle: string;
  shellDescription: string;
  infrastructureRouteReferral: string;
  infrastructureWorkspaceReferral: string;
  sectionSelectorAriaLabel: string;
  refreshLabel: string;
  planTabLabel: string;
  usageTabLabel: string;
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
  sectionSelectorAriaLabel: 'Pulse Pro billing section',
  refreshLabel: 'Refresh',
  planTabLabel: 'Plan',
  usageTabLabel: 'Usage',
  planSectionTitle: 'Plan',
  planSectionDescription: 'Review your active plan, expiry, included limits, and paid capabilities.',
  usageSectionTitle: 'Usage',
};
