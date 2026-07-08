export interface SelfHostedProBillingPresentation {
  navLabel: string;
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
  planComparisonSectionTitle: string;
  planComparisonActionLabel: string;
  planComparisonTrialActionLabel: string;
  planComparisonTrialActionNote: string;
  usageSectionTitle: string;
  hiddenShellTitle: string;
  hiddenShellDescription: string;
  hiddenStateTitle: string;
  hiddenStateBody: string;
  policyLoadingTitle: string;
  policyLoadingBody: string;
  planSelectionPromptTitle: string;
  planSelectionPromptBody: string;
  planSelectionPromptActionLabel: string;
  purchaseActivatedPlanActionLabel: string;
  purchaseCancelledActionLabel: string;
  purchaseExpiredActionLabel: string;
  purchaseFailedActionLabel: string;
  purchaseUnavailableActionLabel: string;
  recoverySectionTitle: string;
  recoverySectionDescription: string;
}

export const SELF_HOSTED_PRO_BILLING_PRESENTATION: SelfHostedProBillingPresentation = {
  navLabel: 'Plans & Billing',
  shellTitle: 'Plans & Billing',
  shellDescription: 'Plan, license, and Patrol mode for this instance.',
  infrastructureRouteReferral: 'Billing and self-hosted plan changes live in Plans & Billing.',
  infrastructureWorkspaceReferral:
    'Self-hosted plan status, Patrol mode, and available capabilities live in Plans & Billing, not here.',
  sectionSelectorAriaLabel: 'Plans and billing section',
  refreshLabel: 'Refresh',
  planTabLabel: 'Plan',
  usageTabLabel: 'Usage',
  planSectionTitle: 'Current plan',
  planSectionDescription: 'Current tier and enabled capabilities.',
  planComparisonSectionTitle: 'Available plans',
  planComparisonActionLabel: 'View plans',
  planComparisonTrialActionLabel: 'Start 14-day free Pro trial',
  planComparisonTrialActionNote:
    'Card required. You will not be charged if you cancel during the trial.',
  usageSectionTitle: 'Usage',
  hiddenShellTitle: 'Demo mode',
  hiddenShellDescription: 'Commercial settings are hidden for this session.',
  hiddenStateTitle: 'License and billing details are hidden',
  hiddenStateBody:
    'This public demo uses sample infrastructure data, so Pulse hides license identity, billing state, monitored-system usage, and upgrade actions instead of creating a demo license.',
  policyLoadingTitle: 'Loading settings access',
  policyLoadingBody:
    'Pulse waits for the session presentation policy before showing license, billing, or usage details.',
  planSelectionPromptTitle: 'Select a plan',
  planSelectionPromptBody: 'Choose the plan for this install.',
  planSelectionPromptActionLabel: 'View plans',
  purchaseActivatedPlanActionLabel: 'Review plan',
  purchaseCancelledActionLabel: 'View plans',
  purchaseExpiredActionLabel: 'View plans',
  purchaseFailedActionLabel: 'Open recovery',
  purchaseUnavailableActionLabel: 'Try again',
  recoverySectionTitle: 'License recovery',
  recoverySectionDescription: 'Paste a license key or clear the license on this install.',
};
