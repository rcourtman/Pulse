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

type TierColumn = 'community' | 'pro' | 'cloud';

type FeatureRow = {
  key: string;
  name: string;
  community: boolean;
  pro: boolean;
  cloud: boolean;
};

// Feature list copied from docs/architecture/ENTITLEMENT_MATRIX.md (capability keys + display names).
const FEATURE_ROWS: FeatureRow[] = [
  {
    key: 'ai_patrol',
    name: 'Pulse Patrol (Background Health Checks)',
    community: true,
    pro: true,
    cloud: true,
  },
  { key: 'ai_alerts', name: 'Alert Analysis', community: false, pro: true, cloud: true },
  { key: 'ai_autofix', name: 'Pulse Patrol Auto-Fix', community: false, pro: true, cloud: true },
  { key: 'kubernetes_ai', name: 'Kubernetes Analysis', community: false, pro: true, cloud: true },
  {
    key: 'agent_profiles',
    name: 'Centralized Agent Profiles',
    community: false,
    pro: true,
    cloud: true,
  },
  {
    key: 'update_alerts',
    name: 'Update Alerts (Container/Package Updates)',
    community: true,
    pro: true,
    cloud: true,
  },
  {
    key: 'rbac',
    name: 'Role-Based Access Control (RBAC)',
    community: false,
    pro: true,
    cloud: true,
  },
  {
    key: 'audit_logging',
    name: 'Enterprise Audit Logging',
    community: false,
    pro: true,
    cloud: true,
  },
  { key: 'sso', name: 'Basic SSO (OIDC)', community: true, pro: true, cloud: true },
  {
    key: 'advanced_sso',
    name: 'Advanced SSO (SAML/Multi-Provider)',
    community: false,
    pro: true,
    cloud: true,
  },
  {
    key: 'advanced_reporting',
    name: 'Advanced Infrastructure Reporting (PDF/CSV)',
    community: false,
    pro: true,
    cloud: true,
  },
  {
    key: 'long_term_metrics',
    name: '90-Day Metric History',
    community: false,
    pro: true,
    cloud: true,
  },
  {
    key: 'relay',
    name: 'Remote Access (Mobile Relay, App Coming Soon)',
    community: false,
    pro: true,
    cloud: true,
  },
  { key: 'multi_user', name: 'Multi-User Mode', community: false, pro: false, cloud: false },
  { key: 'white_label', name: 'White-Label Branding', community: false, pro: false, cloud: false },
  { key: 'multi_tenant', name: 'Multi-Tenant Mode', community: false, pro: false, cloud: false },
  { key: 'unlimited', name: 'Unlimited Instances', community: false, pro: false, cloud: false },
];

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

function CheckCell(props: { enabled: boolean; tier: TierColumn; featureKey: string }) {
  const label = `${props.featureKey}:${props.tier}:${props.enabled ? 'yes' : 'no'}`;
  return (
    <span
      class={[
        'inline-flex w-full justify-center text-sm font-semibold',
        props.enabled ? 'text-emerald-700 dark:text-emerald-300' : 'text-muted',
      ].join(' ')}
      aria-label={label}
      title={label}
    >
      {props.enabled ? '✓' : '—'}
    </span>
  );
}

export default function Pricing() {
  const subscriptionState = createMemo(() => entitlements()?.subscription_state ?? '');
  const isActiveOrTrial = createMemo(
    () => subscriptionState() === 'active' || subscriptionState() === 'trial',
  );

  const [trialCtaMode, setTrialCtaMode] = createSignal<'trial' | 'upgrade'>('trial');
  const [startingTrial, setStartingTrial] = createSignal(false);
  const [trialMessage, setTrialMessage] = createSignal<string | null>(null);

  onMount(() => {
    trackPaywallViewed('pricing', 'pricing_page');
    void loadLicenseStatus().catch((error) => {
      // Best-effort; public page should still render.
      logger.debug('[Pricing] Failed to load license status on mount', error);
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

      // Best-effort error parsing.
      let errText = 'Failed to start trial.';
      try {
        const body = (await res.json()) as { error?: string; message?: string };
        errText = body.error || body.message || errText;
      } catch (error) {
        logger.debug('[Pricing] Failed to parse trial start error payload', error);
      }
      setTrialMessage(errText);
    } catch (error) {
      logger.warn('[Pricing] Trial start request failed', error);
      setTrialMessage('Failed to start trial.');
    } finally {
      setStartingTrial(false);
    }
  };

  const communityCta = createMemo(() => {
    // Requested behavior:
    // If subscription_state is NOT active/trial, show disabled "Current Plan".
    // Otherwise show a non-button "Free" label.
    if (!isActiveOrTrial()) {
      return <TierCtaButton disabled>Current Plan</TierCtaButton>;
    }
    return <TierCtaLabel>Free</TierCtaLabel>;
  });

  const proCta = createMemo(() => {
    if (isActiveOrTrial()) {
      return <TierCtaButton disabled>Current Plan</TierCtaButton>;
    }

    if (trialCtaMode() === 'upgrade') {
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
    }

    return (
      <TierCtaButton disabled={startingTrial()} onClick={startTrial}>
        Start Free 14-day Trial
      </TierCtaButton>
    );
  });

  return (
    <div class="space-y-6">
      <PageHeader title="Pricing" description="Compare tiers and choose what fits." />

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Card padding="lg" class="relative">
          <h2 class="text-lg font-semibold text-base-content">Community</h2>
          <div class="mt-2 text-3xl font-semibold tracking-tight text-base-content">
            Free forever
          </div>
          <ul class="mt-4 space-y-2 text-sm text-base-content">
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Real-time monitoring</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Update alerts</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Basic SSO (OIDC)</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>AI Patrol (monitor only)</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Community support</span>
            </li>
          </ul>
          <div class="mt-6">{communityCta()}</div>
        </Card>

        <Card
          padding="lg"
          class="relative overflow-hidden ring-2 ring-blue-600 dark:ring-blue-500 border-blue-200 dark:border-blue-900"
        >
          <div class="absolute right-4 top-4">
            <span class="inline-flex items-center rounded-full bg-blue-600 px-2.5 py-1 text-[11px] font-semibold text-white shadow-sm">
              Most Popular
            </span>
          </div>

          <h2 class="text-lg font-semibold text-base-content">Pro</h2>
          <div class="mt-2 text-3xl font-semibold tracking-tight text-base-content">$15/month</div>
          <div class="mt-1 text-sm text-muted">or $129/year (save 28%)</div>

          <ul class="mt-4 space-y-2 text-sm text-base-content">
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Everything in Community</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>AI Auto-Fix &amp; Investigation</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Pulse Relay (mobile app rollout coming soon)</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>90-day metric history</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>RBAC &amp; guest access</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Audit logging</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Advanced SSO</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Agent profiles</span>
            </li>
          </ul>

          <div class="mt-6 space-y-2">
            {proCta()}
            <Show when={trialMessage()}>
              <div class="text-xs text-amber-700 dark:text-amber-300">{trialMessage()}</div>
            </Show>
          </div>
        </Card>

        <Card padding="lg" class="relative">
          <h2 class="text-lg font-semibold text-base-content">Cloud</h2>
          <div class="mt-2 text-3xl font-semibold tracking-tight text-base-content">$29/month</div>
          <div class="mt-1 text-sm text-muted">Founding price: $19/mo for first 100 signups</div>
          <ul class="mt-4 space-y-2 text-sm text-base-content">
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Everything in Pro</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Managed hosting</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Automatic backups</span>
            </li>
            <li class="flex gap-2">
              <span class="text-blue-700 dark:text-blue-300">•</span>
              <span>Priority support</span>
            </li>
          </ul>
          <div class="mt-6">
            <a
              class="w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
              href="/cloud/signup"
            >
              Start Hosted Signup
            </a>
          </div>
        </Card>
      </div>

      <Card padding="lg" class="overflow-hidden">
        <h2 class="text-base font-semibold text-base-content">Feature Comparison</h2>
        <div class="mt-4 overflow-x-auto">
          <Table class="min-w-[720px] w-full border-collapse">
            <TableHeader>
              <TableRow class="bg-surface-alt border-b border-border">
                <TableHead class="px-4 py-2 text-left text-xs font-semibold uppercase tracking-wide text-muted">
                  Feature
                </TableHead>
                <TableHead class="px-3 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted">
                  Community
                </TableHead>
                <TableHead class="px-3 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted">
                  Pro
                </TableHead>
                <TableHead class="px-3 py-2 text-center text-xs font-semibold uppercase tracking-wide text-muted">
                  Cloud
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody class="bg-surface divide-y divide-border-subtle">
              <For each={FEATURE_ROWS}>
                {(row) => (
                  <TableRow>
                    <TableCell class="px-4 py-2">
                      <div class="text-sm font-medium text-base-content">{row.name}</div>
                      <div class="mt-0.5 text-xs font-mono text-muted">{row.key}</div>
                    </TableCell>
                    <TableCell class="px-3 py-2">
                      <CheckCell enabled={row.community} tier="community" featureKey={row.key} />
                    </TableCell>
                    <TableCell class="px-3 py-2">
                      <CheckCell enabled={row.pro} tier="pro" featureKey={row.key} />
                    </TableCell>
                    <TableCell class="px-3 py-2">
                      <CheckCell enabled={row.cloud} tier="cloud" featureKey={row.key} />
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
