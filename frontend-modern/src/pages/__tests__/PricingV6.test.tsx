import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';

import PricingV6 from '../PricingV6';
import pricingV6Source from '../PricingV6.tsx?raw';

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
  entitlements: () => ({
    subscription_state: 'expired',
    tier: 'free',
    trial_eligible: false,
  }),
  getUpgradeActionUrlOrFallback: (tier: string) => `/${tier}`,
  loadLicenseStatus: vi.fn().mockResolvedValue(undefined),
  startProTrial: vi.fn(),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: vi.fn(),
}));

vi.mock('@/utils/toast', () => ({
  showToast: vi.fn(),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    warn: vi.fn(),
  },
}));

describe('PricingV6', () => {
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
    expect(screen.queryByRole('button', { name: 'What counts?' })).not.toBeInTheDocument();
  });

  it('imports the shared self-hosted pricing model instead of redefining it locally', () => {
    expect(pricingV6Source).toContain("@/utils/selfHostedPlans");
    expect(pricingV6Source).not.toContain('const TIERS =');
    expect(pricingV6Source).not.toContain('const FEATURE_ROWS');
  });
});
