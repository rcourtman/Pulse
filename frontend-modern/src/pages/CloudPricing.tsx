import { For, Show } from 'solid-js';
import { A } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { PageHeader } from '@/components/shared/PageHeader';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import { onMount } from 'solid-js';

// ---------------------------------------------------------------------------
// Cloud tier data (matches guiding-light spec)
// ---------------------------------------------------------------------------

type CloudTier = {
  key: string;
  name: string;
  price: string;
  subline: string;
  hosts: number;
  support: 'Community' | 'Priority';
  founding?: string;
  highlighted?: boolean;
};

const CLOUD_TIERS: CloudTier[] = [
  {
    key: 'starter',
    name: 'Starter',
    price: '$29/month',
    subline: 'or $249/year (save 29%)',
    hosts: 10,
    support: 'Community',
    founding: '$19/mo — Founding Member rate (first 100 signups)',
    highlighted: true,
  },
  {
    key: 'power',
    name: 'Power',
    price: '$49/month',
    subline: 'or $449/year (save 24%)',
    hosts: 30,
    support: 'Priority',
  },
  {
    key: 'max',
    name: 'Max',
    price: '$79/month',
    subline: 'or $699/year (save 26%)',
    hosts: 75,
    support: 'Priority',
  },
];

const INCLUDED_IN_ALL = [
  'All Pro features (AI patrol, auto-fix, RBAC, audit logging, SAML SSO)',
  'Managed hosting — deploy nothing, upgrade nothing',
  'Daily backups',
  'Secure agent connectivity via Relay',
  'Mobile app access + push notifications',
  'Isolated container per workspace',
  'Subdomain at <id>.cloud.pulserelay.pro',
];

// ---------------------------------------------------------------------------
// Components
// ---------------------------------------------------------------------------

function FoundingBanner() {
  return (
    <div class="rounded-lg border border-amber-300 bg-amber-50 px-5 py-4 dark:border-amber-700 dark:bg-amber-950/40">
      <div class="flex items-start gap-3">
        <span class="text-xl leading-tight">🎉</span>
        <div>
          <p class="text-sm font-semibold text-amber-900 dark:text-amber-200">
            Founding Member Pricing — First 100 Signups
          </p>
          <p class="mt-0.5 text-sm text-amber-800 dark:text-amber-300">
            Lock in Starter Cloud at <strong>$19/month</strong> (normally $29/month) forever. This
            rate is exclusively for the first 100 customers and never increases.
          </p>
        </div>
      </div>
    </div>
  );
}

function CloudTierCard(props: { tier: CloudTier }) {
  const t = props.tier;

  return (
    <Card
      padding="lg"
      class={
        t.highlighted
          ? 'relative ring-2 ring-blue-600 dark:ring-blue-500 border-blue-200 dark:border-blue-900'
          : 'relative'
      }
    >
      <Show when={t.highlighted}>
        <div class="absolute right-4 top-4">
          <span class="inline-flex items-center rounded-full bg-amber-500 px-2.5 py-1 text-[11px] font-semibold text-white shadow-sm">
            Founding Rate
          </span>
        </div>
      </Show>

      <h2 class="text-lg font-semibold text-base-content">{t.name}</h2>

      <Show when={t.founding}>
        <div class="mt-2 text-2xl font-semibold tracking-tight text-amber-600 dark:text-amber-400">
          $19<span class="text-base font-normal text-muted">/month</span>
        </div>
        <div class="text-sm text-muted line-through">{t.price}</div>
      </Show>
      <Show when={!t.founding}>
        <div class="mt-2 text-3xl font-semibold tracking-tight text-base-content">
          {t.price.replace('/month', '')}
          <span class="text-base font-normal text-muted">/month</span>
        </div>
      </Show>
      <div class="mt-1 text-sm text-muted">{t.subline}</div>

      <ul class="mt-4 space-y-2 text-sm text-base-content">
        <li class="flex gap-2">
          <span class="shrink-0 text-emerald-600 dark:text-emerald-400 font-bold">
            {t.hosts} agents
          </span>
        </li>
        <li class="flex gap-2">
          <span class="text-blue-700 dark:text-blue-300 shrink-0">•</span>
          <span>{t.support} support</span>
        </li>
        <li class="flex gap-2">
          <span class="text-blue-700 dark:text-blue-300 shrink-0">•</span>
          <span>All Pro features</span>
        </li>
        <li class="flex gap-2">
          <span class="text-blue-700 dark:text-blue-300 shrink-0">•</span>
          <span>Managed hosting + daily backups</span>
        </li>
      </ul>

      <div class="mt-6">
        <A
          href="/cloud/signup"
          class="w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
        >
          {t.founding ? 'Claim Founding Rate' : `Get ${t.name}`}
        </A>
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export default function CloudPricing() {
  onMount(() => {
    trackPaywallViewed('cloud_pricing', 'cloud_pricing_page');
  });

  return (
    <div class="space-y-8">
      <PageHeader
        title="Pulse Cloud"
        description="Managed hosting. Deploy nothing, monitor everything."
      />

      <FoundingBanner />

      {/* Tier cards */}
      <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <For each={CLOUD_TIERS}>{(tier) => <CloudTierCard tier={tier} />}</For>
      </div>

      {/* What's included in all Cloud plans */}
      <Card padding="lg">
        <h2 class="text-base font-semibold text-base-content">Included in every Cloud plan</h2>
        <ul class="mt-3 grid grid-cols-1 gap-x-8 gap-y-2 sm:grid-cols-2 text-sm text-base-content">
          <For each={INCLUDED_IN_ALL}>
            {(item) => (
              <li class="flex gap-2">
                <span class="text-emerald-600 dark:text-emerald-400 shrink-0 font-bold">✓</span>
                <span>{item}</span>
              </li>
            )}
          </For>
        </ul>
      </Card>

      {/* How it works */}
      <Card padding="lg">
        <h2 class="text-base font-semibold text-base-content">How Cloud works</h2>
        <ol class="mt-3 list-decimal space-y-2 pl-5 text-sm text-base-content">
          <li>Sign up with your email. No credit card required for trial.</li>
          <li>Your isolated Pulse workspace is provisioned in under 60 seconds.</li>
          <li>
            Install the Pulse agent on any Linux machine with one command. It connects back securely
            over Relay — no inbound firewall rules needed.
          </li>
          <li>Monitor, get AI findings, and set up alerts. No maintenance ever.</li>
        </ol>
      </Card>

      {/* Footer links */}
      <div class="flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-sm text-muted">
        <span>
          Already have a Cloud account?{' '}
          <A href="/cloud/signup" class="text-blue-600 hover:underline dark:text-blue-400">
            Sign in
          </A>
        </span>
        <span>
          Prefer self-hosting?{' '}
          <A href="/pricing" class="text-blue-600 hover:underline dark:text-blue-400">
            See self-hosted plans
          </A>
        </span>
        <span>
          Managing multiple clients?{' '}
          <a
            href="mailto:hello@pulserelay.pro?subject=Pulse%20MSP%20Inquiry"
            class="text-blue-600 hover:underline dark:text-blue-400"
          >
            Ask about MSP plans
          </a>
        </span>
      </div>
    </div>
  );
}
