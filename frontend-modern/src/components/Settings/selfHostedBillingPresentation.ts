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
  trialStartTitle: string;
  trialStartBody: string;
  trialStartIdleActionLabel: string;
  trialStartPendingActionLabel: string;
  recoverySectionTitle: string;
  recoverySectionDescription: string;
}

export const SELF_HOSTED_PRO_BILLING_PRESENTATION: SelfHostedProBillingPresentation = {
  navLabel: 'Plans',
  shellTitle: 'Plans & Activation',
  shellDescription:
    'Review your current self-hosted plan, activation status, and unlocked capabilities.',
  infrastructureRouteReferral: 'Billing and self-hosted plan changes live in Plans.',
  infrastructureWorkspaceReferral:
    'Billing, self-hosted plan changes, activation status, and unlocked capabilities live in Plans, not here.',
  sectionSelectorAriaLabel: 'Self-hosted plans section',
  refreshLabel: 'Refresh',
  planTabLabel: 'Plan',
  usageTabLabel: 'Usage',
  planSectionTitle: 'Current plan',
  planSectionDescription:
    'See which self-hosted tier this instance unlocked, what capabilities are active, and how plan status or continuity affects this install.',
  usageSectionTitle: 'Usage',
  hiddenShellTitle: 'Demo mode',
  hiddenShellDescription: 'Commercial settings are hidden for this session.',
  hiddenStateTitle: 'License and billing details are hidden',
  hiddenStateBody:
    'This public demo uses sample infrastructure data, so Pulse hides license identity, billing state, monitored-system usage, and upgrade actions instead of creating a demo license.',
  policyLoadingTitle: 'Loading settings access',
  policyLoadingBody:
    'Pulse waits for the session presentation policy before showing license, billing, or usage details.',
  planSelectionPromptTitle: 'Compare self-hosted plans',
  planSelectionPromptBody:
    'Community keeps monitoring free. Compare Relay when you want secure access from anywhere, or Pro when you want root-cause answers, safe remediation, and 90-day incident history.',
  planSelectionPromptActionLabel: 'Compare plans',
  purchaseActivatedPlanActionLabel: 'Review plan',
  purchaseCancelledActionLabel: 'Compare plans',
  purchaseExpiredActionLabel: 'Restart upgrade',
  purchaseFailedActionLabel: 'Open recovery',
  purchaseUnavailableActionLabel: 'Try again',
  trialStartTitle: 'Try Pro for free',
  trialStartBody:
    'Start a 14-day Pro trial for root-cause analysis, safe auto-fix, and 90-day incident history on this organization.',
  trialStartIdleActionLabel: 'Start 14-day Pro Trial',
  trialStartPendingActionLabel: 'Starting...',
  recoverySectionTitle: 'Activation & Recovery',
  recoverySectionDescription:
    'Activate a purchased key, recover a previous self-hosted purchase, or clear a local key from this instance.',
};
