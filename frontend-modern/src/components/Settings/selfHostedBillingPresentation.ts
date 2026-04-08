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
  monitoredSystemUpgradeArrivalTitle: string;
  monitoredSystemUpgradeArrivalBody: string;
  monitoredSystemUpgradeArrivalActionLabel: string;
  purchaseActivatedUsageActionLabel: string;
  purchaseCancelledActionLabel: string;
  purchaseExpiredActionLabel: string;
  purchaseFailedActionLabel: string;
  trialStartTitle: string;
  trialStartBody: string;
  trialStartIdleActionLabel: string;
  trialStartPendingActionLabel: string;
  recoverySectionTitle: string;
  recoverySectionDescription: string;
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
  monitoredSystemUpgradeArrivalTitle: 'Need a higher monitored-system cap?',
  monitoredSystemUpgradeArrivalBody:
    'Open Pulse Account to compare self-hosted plans, complete checkout, and return here with Pulse Pro activated automatically.',
  monitoredSystemUpgradeArrivalActionLabel: 'Compare plans',
  purchaseActivatedUsageActionLabel: 'Review usage',
  purchaseCancelledActionLabel: 'Compare plans',
  purchaseExpiredActionLabel: 'Restart upgrade',
  purchaseFailedActionLabel: 'Open recovery',
  trialStartTitle: 'Try Pro for free',
  trialStartBody: 'Start a 14-day Pro trial for this organization.',
  trialStartIdleActionLabel: 'Start 14-day Pro Trial',
  trialStartPendingActionLabel: 'Starting...',
  recoverySectionTitle: 'Recovery',
  recoverySectionDescription:
    'Use recovery tools only when you already have a Pulse Pro key or need to remove a local key from this instance.',
};
