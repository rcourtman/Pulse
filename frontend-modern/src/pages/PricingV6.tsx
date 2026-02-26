import { For, Show, createMemo, createSignal, onMount } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { PageHeader } from '@/components/shared/PageHeader';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import { getUpgradeActionUrlOrFallback, loadLicenseStatus, entitlements } from '@/stores/license';
import { LicenseAPI } from '@/api/license';
import { showToast } from '@/utils/toast';
import { logger } from '@/utils/logger';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type TierColumn = 'community' | 'relay' | 'pro' | 'proPlus';

type FeatureRow = {
  key: string;
  name: string;
  community: boolean | string;
  relay: boolean | string;
  pro: boolean | string;
  proPlus: boolean | string;
};

// ---------------------------------------------------------------------------
// Tier pricing data
// ---------------------------------------------------------------------------

const TIERS = {
  community: {
    name: 'Community',
    price: 'Free forever',
    subline: '5 agents included',
    highlights: [
      'Real-time monitoring',
      '7-day metric history',
      'AI Patrol (BYOK + 25 quickstart credits)',
      'Update alerts',
      'Basic SSO (OIDC)',
      'Community support',
    ],
  },
  relay: {
    name: 'Relay',
    price: '$6/month',
    subline: 'or $49/year (save 32%)',
    highlights: [
      'Everything in Community',
      'Remote access via Relay',
      'Mobile app access',
      'Push notifications',
      'Custom URL (yourlab.pulserelay.pro)',
      '8 agents · 14-day history',
    ],
  },
  pro: {
    name: 'Pro',
    price: '$12/month',
    subline: 'or $99/year (save 31%)',
    highlights: [
      'Everything in Relay',
      'AI Auto-Fix & investigation',
      'AI alert analysis',
      'Kubernetes AI analysis',
      '90-day metric history',
      'RBAC, audit logging, SAML SSO',
      'Agent profiles · PDF/CSV reports',
      '15 agents',
    ],
  },
  proPlus: {
    name: 'Pro+',
    price: '$18/month',
    subline: 'or $149/year (save 31%)',
    highlights: ['Everything in Pro', '50 agents'],
  },
} as const;

// ---------------------------------------------------------------------------
// Feature comparison matrix for self-hosted tiers (Community → Pro+).
// Based on ENTITLEMENT_MATRIX.md with display-only rows (hosts, history) added
// and enterprise-only rows (multi_user, white_label, multi_tenant, unlimited) omitted.
// ---------------------------------------------------------------------------

const FEATURE_ROWS: FeatureRow[] = [
  // -- Monitoring & basics --
  {
    key: 'monitoring',
    name: 'Real-time Monitoring',
    community: true,
    relay: true,
    pro: true,
    proPlus: true,
  },
  { key: 'hosts', name: 'Agent Limit', community: '5', relay: '8', pro: '15', proPlus: '50' },
  {
    key: 'history',
    name: 'Metric History',
    community: '7 days',
    relay: '14 days',
    pro: '90 days',
    proPlus: '90 days',
  },
  {
    key: 'update_alerts',
    name: 'Update Alerts (Container/Package)',
    community: true,
    relay: true,
    pro: true,
    proPlus: true,
  },
  { key: 'sso', name: 'Basic SSO (OIDC)', community: true, relay: true, pro: true, proPlus: true },

  // -- Relay features --
  {
    key: 'relay',
    name: 'Remote Access (Relay)',
    community: false,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'mobile_app',
    name: 'Mobile App Access',
    community: false,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'push_notifications',
    name: 'Push Notifications',
    community: false,
    relay: true,
    pro: true,
    proPlus: true,
  },

  // -- AI features --
  {
    key: 'ai_patrol',
    name: 'AI Patrol (Background Health Checks)',
    community: true,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'ai_autofix',
    name: 'AI Patrol Auto-Fix',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'ai_alerts',
    name: 'AI Alert Analysis',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'kubernetes_ai',
    name: 'Kubernetes AI Analysis',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },

  // -- Team & compliance --
  {
    key: 'rbac',
    name: 'Role-Based Access Control (RBAC)',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'audit_logging',
    name: 'Audit Logging',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'advanced_sso',
    name: 'Advanced SSO (SAML/Multi-Provider)',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'agent_profiles',
    name: 'Centralized Agent Profiles',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'advanced_reporting',
    name: 'PDF/CSV Reporting',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
];

// ---------------------------------------------------------------------------
// Shared sub-components
// ---------------------------------------------------------------------------

function TierCtaButton(props: { children: string; disabled?: boolean; onClick?: () => void }) {
  return (
    <button
      class={[
        'w-full inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-semibold transition-colors',
        props.disabled
          ? 'bg-surface-hover text-base-content cursor-not-allowed'
          : 'bg-blue-600 text-white hover:bg-blue-700',
      ].join(' ')}
      disabled={props.disabled}
      onClick={props.onClick}
      type="button"
    >
      {props.children}
    </button>
  );
}

function TierCtaLabel(props: { children: string }) {
  return (
    <div class="w-full inline-flex items-center justify-center rounded-md border border-border bg-surface px-4 py-2 text-sm font-semibold text-base-content">
      {props.children}
    </div>
  );
}

function CheckCell(props: { value: boolean | string; tier: TierColumn; featureKey: string }) {
  if (typeof props.value === 'string') {
    const label = `${props.tier} tier: ${props.value}`;
    return (
      <span
        class="inline-flex w-full justify-center text-sm font-semibold text-base-content"
        aria-label={label}
        title={label}
      >
        {props.value}
      </span>
    );
  }

  const label = props.value
    ? `Included in ${props.tier} tier`
    : `Not included in ${props.tier} tier`;
  return (
    <span
      class={[
        'inline-flex w-full justify-center text-sm font-semibold',
        props.value ? 'text-emerald-700 dark:text-emerald-300' : 'text-muted',
      ].join(' ')}
      aria-label={label}
      title={label}
    >
      {props.value ? '✓' : '—'}
    </span>
  );
}

function BulletList(props: { items: readonly string[] }) {
  return (
    <ul class="mt-4 space-y-2 text-sm text-base-content">
      <For each={props.items}>
        {(item) => (
          <li class="flex gap-2">
            <span class="text-blue-700 dark:text-blue-300 shrink-0">•</span>
            <span>{item}</span>
          </li>
        )}
      </For>
    </ul>
  );
}

// ---------------------------------------------------------------------------
// Tier cards
// ---------------------------------------------------------------------------

function TierCard(props: {
  tier: (typeof TIERS)[keyof typeof TIERS];
  cta: () => import('solid-js').JSX.Element;
  highlighted?: boolean;
  badge?: string;
  message?: string | null;
}) {
  return (
    <Card
      padding="lg"
      class={
        props.highlighted
          ? 'relative overflow-hidden ring-2 ring-blue-600 dark:ring-blue-500 border-blue-200 dark:border-blue-900'
          : 'relative'
      }
    >
      <Show when={props.badge}>
        <div class="absolute right-4 top-4">
          <span class="inline-flex items-center rounded-full bg-blue-600 px-2.5 py-1 text-[11px] font-semibold text-white shadow-sm">
            {props.badge}
          </span>
        </div>
      </Show>

      <h2 class="text-lg font-semibold text-base-content">{props.tier.name}</h2>
      <div class="mt-2 text-3xl font-semibold tracking-tight text-base-content">
        {props.tier.price}
      </div>
      <div class="mt-1 text-sm text-muted">{props.tier.subline}</div>

      <BulletList items={props.tier.highlights} />

      <div class="mt-6 space-y-2">
        {props.cta()}
        <Show when={props.message}>
          <div class="text-xs text-amber-700 dark:text-amber-300">{props.message}</div>
        </Show>
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Main page component
// ---------------------------------------------------------------------------

export default function PricingV6() {
  const subscriptionState = createMemo(() => entitlements()?.subscription_state ?? '');
  const currentTier = createMemo(() => entitlements()?.tier ?? 'free');
  const isActiveOrTrial = createMemo(
    () => subscriptionState() === 'active' || subscriptionState() === 'trial',
  );

  const [trialCtaMode, setTrialCtaMode] = createSignal<'trial' | 'upgrade'>('trial');
  const [startingTrial, setStartingTrial] = createSignal(false);
  const [trialMessage, setTrialMessage] = createSignal<string | null>(null);

  onMount(() => {
    trackPaywallViewed('pricing', 'pricing_v6');
    void loadLicenseStatus().catch((error) => {
      logger.debug('[PricingV6] Failed to load license status on mount', error);
    });
  });

  const startTrial = async () => {
    if (startingTrial()) return;
    setTrialMessage(null);
    setStartingTrial(true);

    try {
      const res = await LicenseAPI.startTrial();
      if (res.ok) {
        showToast('success', 'Pro trial started (14 days).');
        await loadLicenseStatus(true);
        return;
      }

      if (res.status === 429) {
        setTrialMessage('Try again later.');
        return;
      }

      if (res.status === 409) {
        setTrialCtaMode('upgrade');
        return;
      }

      let errText = 'Failed to start trial.';
      try {
        const body = (await res.json()) as { error?: string; message?: string };
        errText = body.error || body.message || errText;
      } catch (error) {
        logger.debug('[PricingV6] Failed to parse trial start error payload', error);
      }
      setTrialMessage(errText);
    } catch (error) {
      logger.warn('[PricingV6] Trial start request failed', error);
      setTrialMessage('Failed to start trial.');
    } finally {
      setStartingTrial(false);
    }
  };

  // -- CTAs per tier --

  // Normalize backend tier strings to pricing-page columns.
  // pro_annual and lifetime map to "pro"; expired/no-subscription maps to "free".
  const normalizedTier = createMemo(() => {
    if (!isActiveOrTrial()) return 'free';
    const raw = currentTier();
    if (raw === 'pro_annual' || raw === 'lifetime') return 'pro';
    return raw;
  });

  const isCurrentTier = (tier: string) => normalizedTier() === tier;

  const communityCta = createMemo(() => {
    if (isCurrentTier('free')) {
      return <TierCtaButton disabled>Current Plan</TierCtaButton>;
    }
    return <TierCtaLabel>Free</TierCtaLabel>;
  });

  // Tier rank for comparison — higher rank = higher tier.
  const TIER_RANK: Record<string, number> = {
    free: 0,
    relay: 1,
    pro: 2,
    pro_plus: 3,
    cloud: 4,
    msp: 5,
    enterprise: 6,
  };

  // Whether the user is eligible for a trial — backend provides this directly.
  const canStartTrial = createMemo(() => entitlements()?.trial_eligible === true);

  const relayCta = createMemo(() => {
    if (isCurrentTier('relay')) {
      return <TierCtaButton disabled>Current Plan</TierCtaButton>;
    }
    // Users already on a higher tier don't need a Relay CTA.
    if ((TIER_RANK[normalizedTier()] ?? 0) > (TIER_RANK['relay'] ?? 0)) {
      return <TierCtaLabel>Included</TierCtaLabel>;
    }
    return (
      <a
        class="w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
        href={getUpgradeActionUrlOrFallback('relay')}
        target="_blank"
        rel="noopener noreferrer"
      >
        Buy Relay
      </a>
    );
  });

  const proTrialOrUpgrade = createMemo(() => {
    if (isCurrentTier('pro')) {
      return <TierCtaButton disabled>Current Plan</TierCtaButton>;
    }
    // Users already on a higher tier don't need a Pro CTA.
    if ((TIER_RANK[normalizedTier()] ?? 0) > (TIER_RANK['pro'] ?? 0)) {
      return <TierCtaLabel>Included</TierCtaLabel>;
    }

    if (canStartTrial() && trialCtaMode() !== 'upgrade') {
      return (
        <TierCtaButton disabled={startingTrial()} onClick={startTrial}>
          Start Free 14-day Trial
        </TierCtaButton>
      );
    }

    return (
      <a
        class="w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
        href={getUpgradeActionUrlOrFallback('upgrade')}
        target="_blank"
        rel="noopener noreferrer"
      >
        Upgrade to Pro
      </a>
    );
  });

  const proPlusCta = createMemo(() => {
    if (isCurrentTier('pro_plus')) {
      return <TierCtaButton disabled>Current Plan</TierCtaButton>;
    }
    // Users already on a higher tier don't need a Pro+ CTA.
    if ((TIER_RANK[normalizedTier()] ?? 0) > (TIER_RANK['pro_plus'] ?? 0)) {
      return <TierCtaLabel>Included</TierCtaLabel>;
    }

    if (canStartTrial() && trialCtaMode() !== 'upgrade') {
      return (
        <TierCtaButton disabled={startingTrial()} onClick={startTrial}>
          Start Free 14-day Trial
        </TierCtaButton>
      );
    }

    return (
      <a
        class="w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
        href={getUpgradeActionUrlOrFallback('upgrade')}
        target="_blank"
        rel="noopener noreferrer"
      >
        Upgrade to Pro+
      </a>
    );
  });

  // -- Render --

  return (
    <div class="space-y-6">
      <PageHeader title="Pricing" description="Compare tiers and choose what fits." />

      {/* Tier cards — 4 columns on desktop, 2 on tablet, 1 on mobile */}
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <TierCard tier={TIERS.community} cta={communityCta} />
        <TierCard tier={TIERS.relay} cta={relayCta} />
        <TierCard
          tier={TIERS.pro}
          cta={proTrialOrUpgrade}
          highlighted
          badge="Most Popular"
          message={trialMessage()}
        />
        <TierCard tier={TIERS.proPlus} cta={proPlusCta} />
      </div>

      {/* Upsell links */}
      <div class="flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-sm text-muted">
        <span>
          Need managed hosting?{' '}
          <a href="/cloud" class="text-blue-600 hover:underline dark:text-blue-400">
            See Cloud plans
          </a>
        </span>
        <span>
          Managing clients?{' '}
          <a
            href="mailto:hello@pulserelay.pro?subject=Pulse%20MSP%20Inquiry"
            class="text-blue-600 hover:underline dark:text-blue-400"
          >
            See MSP plans
          </a>
        </span>
        <span>
          Need 50+ agents?{' '}
          <a
            href="mailto:hello@pulserelay.pro?subject=Pulse%20Enterprise%20Inquiry"
            class="text-blue-600 hover:underline dark:text-blue-400"
          >
            Contact us
          </a>
        </span>
      </div>

      {/* Feature comparison table */}
      <Card padding="lg" class="overflow-hidden">
        <h2 class="text-base font-semibold text-base-content">Feature Comparison</h2>
        <div class="mt-4 overflow-x-auto">
          <Table class="min-w-[800px] w-full border-collapse">
            <TableHeader>
              <TableRow class="bg-surface-alt border-b border-border">
                <TableHead class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Feature
                </TableHead>
                <TableHead class="px-3 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted">
                  Community
                </TableHead>
                <TableHead class="px-3 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted">
                  Relay
                </TableHead>
                <TableHead class="px-3 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted">
                  Pro
                </TableHead>
                <TableHead class="px-3 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted">
                  Pro+
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="bg-surface divide-y divide-border-subtle">
              <For each={FEATURE_ROWS}>
                {(row) => (
                  <TableRow>
                    <TableCell class="px-4 py-2">
                      <div class="text-sm font-medium text-base-content">{row.name}</div>
                    </TableCell>
                    <TableCell class="px-3 py-2">
                      <CheckCell value={row.community} tier="community" featureKey={row.key} />
                    </TableCell>
                    <TableCell class="px-3 py-2">
                      <CheckCell value={row.relay} tier="relay" featureKey={row.key} />
                    </TableCell>
                    <TableCell class="px-3 py-2">
                      <CheckCell value={row.pro} tier="pro" featureKey={row.key} />
                    </TableCell>
                    <TableCell class="px-3 py-2">
                      <CheckCell value={row.proPlus} tier="proPlus" featureKey={row.key} />
                    </TableCell>
                  </TableRow>
                )}
              </For>
            </TableBody>
          </Table>
        </div>
      </Card>
    </div>
  );
}
