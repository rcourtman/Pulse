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
  mspReferralBody: string;
  mspReferralActionLabel: string;
  infrastructureMspReferralBody: string;
  infrastructureMspReferralDismissLabel: string;
}

// Pulse MSP is sold from pulserelay.pro, not from this install. The campaign
// tag keeps visits that start here apart in the website funnel.
export const PULSE_MSP_REFERRAL_URL =
  'https://pulserelay.pro/msp?utm_source=pulse&utm_medium=app&utm_campaign=msp';

// The Infrastructure line keeps the same source and campaign, so the funnel
// still counts it as an app referral, and its own medium tells the two apart.
export const PULSE_MSP_INFRASTRUCTURE_REFERRAL_URL =
  'https://pulserelay.pro/msp?utm_source=pulse&utm_medium=app-infrastructure&utm_campaign=msp';

// Closing the Infrastructure line is a per-browser presentation choice, kept
// beside the line it hides like the thresholds help banner key.
export const INFRASTRUCTURE_MSP_REFERRAL_DISMISSED_STORAGE_KEY =
  'pulse-infrastructure-msp-referral-dismissed';

// Settings > Infrastructure offers Pulse MSP only to installs sized like a
// provider's: several separate Proxmox VE or Backup Server connections, or a
// large agent fleet. A home lab with one cluster and one backup server never
// sees it.
export const MSP_REFERRAL_MIN_PROXMOX_CONNECTIONS = 3;
export const MSP_REFERRAL_MIN_AGENTS = 25;

export interface InfrastructureMspReferralRow {
  ownerType: string;
  isAgent: boolean;
}

export function infrastructureLooksProviderSized(
  rows: readonly InfrastructureMspReferralRow[],
): boolean {
  let proxmoxVe = 0;
  let proxmoxBackup = 0;
  let agents = 0;
  for (const row of rows) {
    if (row.isAgent) agents += 1;
    else if (row.ownerType === 'pve') proxmoxVe += 1;
    else if (row.ownerType === 'pbs') proxmoxBackup += 1;
  }
  return (
    proxmoxVe >= MSP_REFERRAL_MIN_PROXMOX_CONNECTIONS ||
    proxmoxBackup >= MSP_REFERRAL_MIN_PROXMOX_CONNECTIONS ||
    agents >= MSP_REFERRAL_MIN_AGENTS
  );
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
  mspReferralBody:
    'Managing infrastructure for clients? Pulse MSP is a separate self-hosted install with an isolated workspace for each client. Evaluate two clients free for 60 days.',
  mspReferralActionLabel: 'See Pulse MSP',
  infrastructureMspReferralBody:
    'Managing infrastructure for clients? Pulse MSP gives each client an isolated workspace. Two clients are free for 60 days.',
  infrastructureMspReferralDismissLabel: 'Hide the Pulse MSP suggestion',
};
