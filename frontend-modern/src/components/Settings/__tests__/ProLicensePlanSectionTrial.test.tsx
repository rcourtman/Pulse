import { describe, expect, it, afterEach } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { Component } from 'solid-js';
import { ProLicensePlanSection } from '../ProLicensePlanSection';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from '../selfHostedBillingPresentation';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';

const baseProps = () => ({
  activationSuccessSummary: null,
  commercialMigrationNotice: null,
  commercialPlanModel: { summary: [], details: [] },
  currentPlanSummary: {
    title: 'Community',
    body: 'Core monitoring',
    badgeClass: '',
    statusLabel: 'Free',
    unlockedFeaturesLabel: 'Included',
    unlockedFeatures: [],
    includedExtras: [],
    supplementalBadges: [],
  },
  entitlements: null,
  grandfatheredPriceNotice: null,
  hasLicenseDetails: false,
  loading: false,
  onReload: () => {},
  planSelectionPrompt: null,
  planStatus: null,
  purchaseActivationNotice: null,
  purchaseActivationAction: null,
});

const renderInRouter = (component: Component) => {
  render(() => (
    <Router>
      <Route path="/" component={component} />
    </Router>
  ));
};

describe('ProLicensePlanSection trial action', () => {
  afterEach(() => cleanup());

  it('renders the trial button with its card-required note and trial link', () => {
    renderInRouter(() => (
      <ProLicensePlanSection
        {...baseProps()}
        planComparisonSummary={{
          cards: [{ title: 'Pro', body: 'Everything', highlights: [] }],
          action: {
            label: SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonActionLabel,
            destination: resolveUpgradeDestination(
              '/auth/license-purchase-start?feature=self_hosted_plan',
            ),
          },
          trialAction: {
            label: SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonTrialActionLabel,
            note: SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonTrialActionNote,
            destination: resolveUpgradeDestination(
              '/auth/license-purchase-start?trial=1&feature=self_hosted_plan',
            ),
          },
        }}
      />
    ));

    const trialLink = screen.getByRole('link', {
      name: SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonTrialActionLabel,
    });
    expect(trialLink.getAttribute('href')).toContain('trial=1');
    expect(trialLink.getAttribute('href')).toContain('feature=self_hosted_plan');
    expect(
      screen.getByText(SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonTrialActionNote),
    ).toBeTruthy();
  });

  it('renders no trial button when trialAction is null', () => {
    renderInRouter(() => (
      <ProLicensePlanSection
        {...baseProps()}
        planComparisonSummary={{
          cards: [{ title: 'Pro', body: 'Everything', highlights: [] }],
          action: {
            label: SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonActionLabel,
            destination: resolveUpgradeDestination(
              '/auth/license-purchase-start?feature=self_hosted_plan',
            ),
          },
          trialAction: null,
        }}
      />
    ));

    expect(
      screen.queryByText(SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonTrialActionLabel),
    ).toBeNull();
    expect(
      screen.queryByText(SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonTrialActionNote),
    ).toBeNull();
  });
});
