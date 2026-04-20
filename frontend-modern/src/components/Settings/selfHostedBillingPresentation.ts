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
  shellTitle: 'Plans & Billing',
  shellDescription:
    'Manage self-hosted billing, plan features, and activation status.',
  infrastructureRouteReferral: 'Billing and self-hosted plan features live in Plans & Billing.',
  infrastructureWorkspaceReferral:
    'Billing, self-hosted plan features, and activation status live in Plans & Billing, not here.',
  sectionSelectorAriaLabel: 'Self-hosted billing section',
  refreshLabel: 'Refresh',
  planTabLabel: 'Plan',
  usageTabLabel: 'Usage',
  planSectionTitle: 'Plan',
  planSectionDescription:
    'Review your active plan, expiry, and the paid capabilities that come with it.',
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
    'Community keeps core monitoring free. Compare Relay and Pro in Pulse Account, then return here with the new plan activated automatically.',
  planSelectionPromptActionLabel: 'Compare plans',
  purchaseActivatedPlanActionLabel: 'Review plan',
  purchaseCancelledActionLabel: 'Compare plans',
  purchaseExpiredActionLabel: 'Restart upgrade',
  purchaseFailedActionLabel: 'Open recovery',
  purchaseUnavailableActionLabel: 'Try again',
  trialStartTitle: 'Try Pro for free',
  trialStartBody: 'Start a 14-day Pro trial for this organization.',
  trialStartIdleActionLabel: 'Start 14-day Pro Trial',
  trialStartPendingActionLabel: 'Starting...',
  recoverySectionTitle: 'Recovery',
  recoverySectionDescription:
    'Use recovery tools only when you already have an activation key or need to remove a local key from this instance.',
};
