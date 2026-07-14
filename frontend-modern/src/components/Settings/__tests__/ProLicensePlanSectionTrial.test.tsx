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

describe('ProLicensePlanSection retired trial acquisition', () => {
  afterEach(() => cleanup());

  it('renders the ordinary plan handoff without a trial action', () => {
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
        }}
      />
    ));

    const plansLink = screen.getByRole('link', {
      name: SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonActionLabel,
    });
    expect(plansLink.getAttribute('href')).toContain('feature=self_hosted_plan');
    expect(plansLink.getAttribute('href')).not.toContain('trial=1');
    expect(screen.queryByText(/free pro trial/i)).toBeNull();
    expect(screen.queryByText(/card required/i)).toBeNull();
  });
});
