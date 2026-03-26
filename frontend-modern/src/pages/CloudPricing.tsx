import { For, Show } from 'solid-js';
import { A } from '@solidjs/router';
import { Card } from '@/components/shared/Card';
import { PageHeader } from '@/components/shared/PageHeader';
import { trackPaywallViewed } from '@/utils/upgradeMetrics';
import { onMount } from 'solid-js';
import {
  CLOUD_PLAN_DEFINITIONS,
  getCloudPlanPricePresentation,
  type CloudPlanDefinition,
} from '@/utils/cloudPlans';

const INCLUDED_IN_ALL = [
  'All Pro features',
  'Managed hosting',
  'Daily backups',
  'Secure agent connectivity via Relay',
  'Mobile app access and push notifications',
  'Dedicated workspace URL',
];

function CloudTierCard(props: { tier: CloudPlanDefinition }) {
  const t = props.tier;
  const price = getCloudPlanPricePresentation(t);

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
            Founding rate
          </span>
        </div>
      </Show>

      <h2 class="text-lg font-semibold text-base-content">{t.name}</h2>

      <Show when={price.compareAtMonthlyPrice}>
        <div class="mt-2 text-2xl font-semibold tracking-tight text-amber-600 dark:text-amber-400">
          {price.monthlyPrice}
          <span class="text-base font-normal text-muted">{price.cadence}</span>
        </div>
        <div class="text-sm text-muted line-through">
          {price.compareAtMonthlyPrice}
          {price.cadence}
        </div>
      </Show>
      <Show when={!price.compareAtMonthlyPrice}>
        <div class="mt-2 text-3xl font-semibold tracking-tight text-base-content">
          {price.monthlyPrice}
          <span class="text-base font-normal text-muted">{price.cadence}</span>
        </div>
      </Show>
      <div class="mt-1 text-sm text-muted">{price.annualSummary}</div>

      <dl class="mt-4 space-y-3 text-sm text-base-content">
        <div class="flex items-center justify-between gap-3">
          <dt class="text-muted">Monitored systems</dt>
          <dd class="font-semibold">{t.monitoredSystems}</dd>
        </div>
        <div class="flex items-center justify-between gap-3">
          <dt class="text-muted">Support</dt>
          <dd class="font-semibold">{t.support}</dd>
        </div>
      </dl>

      <div class="mt-6">
        <A
          href={`/cloud/signup?tier=${t.tier}`}
          class="w-full inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-blue-700"
        >
          {`Choose ${t.name}`}
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
        description="Managed Pulse hosting with Pro features included."
      />

      {/* Tier cards */}
      <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <For each={CLOUD_PLAN_DEFINITIONS}>{(tier) => <CloudTierCard tier={tier} />}</For>
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
        <h2 class="text-base font-semibold text-base-content">Setup</h2>
        <ol class="mt-3 list-decimal space-y-2 pl-5 text-sm text-base-content">
          <li>Create your workspace. No credit card is required for the trial.</li>
          <li>Install the Pulse agent on any Linux machine.</li>
          <li>Connect systems, review findings, and configure alerts.</li>
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
