import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';

import PricingV6 from '../PricingV6';
import pricingV6Source from '../PricingV6.tsx?raw';

const entitlementsState = {
  subscription_state: 'expired',
  tier: 'free',
  trial_eligible: false,
};
const startProTrialMock = vi.fn();
const loadLicenseStatusMock = vi.fn().mockResolvedValue(undefined);
const showToastMock = vi.fn();

vi.mock('@/components/shared/Card', () => ({
  Card: (props: { children?: JSX.Element }) => <div>{props.children}</div>,
}));

vi.mock('@/components/shared/Table', () => ({
  Table: (props: { children?: JSX.Element }) => <table>{props.children}</table>,
  TableHeader: (props: { children?: JSX.Element }) => <thead>{props.children}</thead>,
  TableBody: (props: { children?: JSX.Element }) => <tbody>{props.children}</tbody>,
  TableRow: (props: { children?: JSX.Element }) => <tr>{props.children}</tr>,
  TableHead: (props: { children?: JSX.Element }) => <th>{props.children}</th>,
  TableCell: (props: { children?: JSX.Element }) => <td>{props.children}</td>,
}));

vi.mock('@/components/shared/PageHeader', () => ({
  PageHeader: (props: { title: string; description: string }) => (
    <header>
      <h1>{props.title}</h1>
      <p>{props.description}</p>
    </header>
  ),
}));

vi.mock('@/stores/license', () => ({
  entitlements: () => entitlementsState,
  getUpgradeActionUrlOrFallback: (tier: string) => `/${tier}`,
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: vi.fn(),
}));

vi.mock('@/utils/toast', () => ({
  showToast: (...args: unknown[]) => showToastMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    warn: vi.fn(),
  },
}));

describe('PricingV6', () => {
  beforeEach(() => {
    entitlementsState.subscription_state = 'expired';
    entitlementsState.tier = 'free';
    entitlementsState.trial_eligible = false;
    startProTrialMock.mockReset();
    loadLicenseStatusMock.mockReset();
    loadLicenseStatusMock.mockResolvedValue(undefined);
    showToastMock.mockReset();
  });

  it('renders self-hosted plan tiers from the shared pricing model', () => {
    render(() => <PricingV6 />);

    expect(screen.getAllByText('Community').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Relay').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Pro').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Pro+').length).toBeGreaterThan(0);
    expect(screen.getByText('8 monitored systems · 14-day history')).toBeInTheDocument();
    expect(screen.getByText('15 monitored systems')).toBeInTheDocument();
    expect(screen.getByText('50 monitored systems')).toBeInTheDocument();
    expect(screen.getByText('Feature Comparison')).toBeInTheDocument();
    expect(
      screen.queryByText('Billing is based on monitored systems. Child resources are included.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'View counting rules' })).not.toBeInTheDocument();
  });

  it('imports the shared self-hosted pricing model instead of redefining it locally', () => {
    expect(pricingV6Source).toContain("@/utils/selfHostedPlans");
    expect(pricingV6Source).toContain("@/utils/upgradePresentation");
    expect(pricingV6Source).toContain("@/utils/trialStartAction");
    expect(pricingV6Source).not.toContain('const TIERS =');
    expect(pricingV6Source).not.toContain('const FEATURE_ROWS');
    expect(pricingV6Source).not.toContain("setTrialMessage('Trial already used.')");
    expect(pricingV6Source).not.toContain('startProTrial()');
  });

  it('switches Pro pricing CTA to upgrade when trial is already used', async () => {
    entitlementsState.trial_eligible = true;
    startProTrialMock.mockRejectedValue({
      status: 409,
      code: 'trial_already_used',
      message: 'Trial already used',
    });

    render(() => <PricingV6 />);

    fireEvent.click(screen.getAllByRole('button', { name: 'Start Free 14-day Trial' })[0]);

    await waitFor(() => {
      expect(screen.getByText('Trial already used')).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: 'Upgrade to Pro' })).toHaveAttribute(
      'href',
      '/upgrade',
    );
  });
});
