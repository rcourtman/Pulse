import { Component, For, Show } from 'solid-js';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import { UpgradeLink } from '@/components/shared/UpgradeLink';
import { licenseEntitlementsLoadError } from '@/stores/licenseEntitlements';
import {
  getLicenseStatusLoadingState,
  getNoActiveSelfHostedActivationState,
} from '@/utils/licensePresentation';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';
import { CommercialStatGrid } from './CommercialBillingSections';

interface Notice {
  title: string;
  body: string;
  tone: string;
}

interface ActionNotice extends Notice {
  actionLabel: string;
  actionDestination: UpgradeDestination;
}

interface ProLicensePlanSectionProps {
  activationSuccessSummary: {
    title: string;
    body: string;
    tone: string;
    highlightsLabel: string;
    highlights: string[];
    actionLabel?: string;
    actionUrl?: string;
  } | null;
  activationProof: {
    title: string;
    body: string;
    items: Array<{
      label: string;
      statusLabel: string;
      state: 'active' | 'partial' | 'missing';
      detail: string;
    }>;
  } | null;
  commercialMigrationNotice: Notice | null;
  commercialPlanModel: {
    summary: Array<{ label: string; value: string | number }>;
    details: Array<{ label: string; value: string | number }>;
  };
  currentPlanSummary: {
    title: string;
    body: string;
    badgeClass: string;
    statusLabel: string;
    unlockedFeaturesLabel: string;
    unlockedFeatures: string[];
    includedExtrasLabel?: string;
    includedExtras: string[];
    supplementalBadges: string[];
    supplementalSummary?: string;
    privateRuntimeAction?: {
      actionLabel: string;
      actionUrl: string;
    };
  };
  entitlements: {
    in_grace_period?: boolean;
    grace_period_end?: string | null;
  } | null;
  grandfatheredPriceNotice: Notice | null;
  hasLicenseDetails: boolean;
  loading: boolean;
  onReload: () => void;
  planComparisonSummary: {
    cards: Array<{
      title: string;
      body: string;
      highlights: string[];
    }>;
    action: {
      label: string;
      destination: UpgradeDestination;
    } | null;
  };
  planSelectionPrompt: ActionNotice | null;
  purchaseActivationNotice: Notice | null;
  purchaseActivationAction: {
    label: string;
    destination: UpgradeDestination;
  } | null;
}

const formatDate = (value?: string | null) => {
  if (!value) return 'Not available';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

const proofStateClass = (state: 'active' | 'partial' | 'missing') => {
  switch (state) {
    case 'active':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-200';
    case 'partial':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200';
    case 'missing':
      return 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-200';
  }
};

export const ProLicensePlanSection: Component<ProLicensePlanSectionProps> = (props) => {
  return (
    <>
      <Show when={props.activationSuccessSummary}>
        {(summary) => (
          <div class={`mb-4 rounded-md border p-3 text-sm ${summary().tone}`}>
            <p class="font-medium">{summary().title}</p>
            <p class="mt-1 text-xs opacity-90">{summary().body}</p>
            <Show when={summary().actionUrl && summary().actionLabel}>
              <a
                href={summary().actionUrl}
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-1 mt-3 min-h-10 sm:min-h-9 rounded-md border border-current/20 px-3 py-2 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5"
              >
                {summary().actionLabel}
              </a>
            </Show>
            <Show when={summary().highlights.length > 0}>
              <div class="mt-3">
                <p class="text-[11px] uppercase tracking-wide opacity-80">
                  {summary().highlightsLabel}
                </p>
                <ul class="mt-2 grid gap-2 sm:grid-cols-2">
                  <For each={summary().highlights}>
                    {(feature) => (
                      <li class="text-xs flex items-center gap-2">
                        <span class="w-1.5 h-1.5 rounded-full bg-current"></span>
                        {feature}
                      </li>
                    )}
                  </For>
                </ul>
              </div>
            </Show>
          </div>
        )}
      </Show>
      <Show when={props.purchaseActivationNotice}>
        {(notice) => (
          <div class={`mb-4 rounded-md border p-3 text-sm ${notice().tone}`}>
            <p class="font-medium">{notice().title}</p>
            <p class="mt-1 text-xs opacity-90">{notice().body}</p>
            <Show when={props.purchaseActivationAction}>
              {(action) => (
                <UpgradeLink
                  class="inline-flex items-center gap-1 mt-3 min-h-10 sm:min-h-9 rounded-md border border-current/20 px-3 py-2 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5"
                  destination={action().destination}
                >
                  {action().label}
                </UpgradeLink>
              )}
            </Show>
          </div>
        )}
      </Show>
      <Show when={props.planSelectionPrompt}>
        {(prompt) => (
          <div class={`mb-4 rounded-md border p-3 text-sm ${prompt().tone}`}>
            <p class="font-medium">{prompt().title}</p>
            <p class="mt-1 text-xs opacity-90">{prompt().body}</p>
            <UpgradeLink
              class="inline-flex items-center gap-1 mt-3 min-h-10 sm:min-h-9 rounded-md border border-current/20 px-3 py-2 text-xs font-medium hover:bg-black/5 dark:hover:bg-white/5"
              destination={prompt().actionDestination}
            >
              {prompt().actionLabel}
            </UpgradeLink>
          </div>
        )}
      </Show>
      <div class="mb-4 rounded-md border border-border bg-surface-alt p-4">
        <div class="flex flex-wrap items-center gap-2">
          <span
            class={`px-2 py-1 text-xs font-medium rounded-full ${props.currentPlanSummary.badgeClass}`}
          >
            {props.currentPlanSummary.statusLabel}
          </span>
          <For each={props.currentPlanSummary.supplementalBadges}>
            {(badge) => (
              <span class="px-2 py-1 text-xs font-medium rounded-full bg-surface text-base-content border border-border">
                {badge}
              </span>
            )}
          </For>
          <Show when={props.entitlements?.in_grace_period}>
            <span class="text-xs text-amber-700 dark:text-amber-300">
              Grace until {formatDate(props.entitlements?.grace_period_end)}
            </span>
          </Show>
        </div>
        <p class="mt-3 text-lg font-semibold text-base-content">{props.currentPlanSummary.title}</p>
        <p class="mt-1 text-sm text-muted">{props.currentPlanSummary.body}</p>
        <Show when={props.currentPlanSummary.supplementalSummary}>
          {(summary) => <p class="mt-2 text-xs text-muted">{summary()}</p>}
        </Show>
        <Show when={props.currentPlanSummary.privateRuntimeAction}>
          {(action) => (
            <a
              href={action().actionUrl}
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1 mt-3 min-h-10 sm:min-h-9 rounded-md border border-border px-3 py-2 text-xs font-medium text-base-content hover:bg-surface-hover"
            >
              {action().actionLabel}
            </a>
          )}
        </Show>
        <Show when={props.currentPlanSummary.unlockedFeatures.length > 0}>
          <div class="mt-4">
            <p class="text-xs uppercase tracking-wide text-muted mb-2">
              {props.currentPlanSummary.unlockedFeaturesLabel}
            </p>
            <ul class="grid gap-2 sm:grid-cols-2">
              <For each={props.currentPlanSummary.unlockedFeatures}>
                {(feature) => (
                  <li class="text-sm text-base-content flex items-center gap-2">
                    <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                    {feature}
                  </li>
                )}
              </For>
            </ul>
          </div>
        </Show>
        <Show when={props.currentPlanSummary.includedExtras.length > 0}>
          <div class="mt-4">
            <p class="text-xs uppercase tracking-wide text-muted mb-2">
              {props.currentPlanSummary.includedExtrasLabel || 'Included extras'}
            </p>
            <ul class="grid gap-2 sm:grid-cols-2">
              <For each={props.currentPlanSummary.includedExtras}>
                {(feature) => (
                  <li class="text-sm text-base-content flex items-center gap-2">
                    <span class="w-1.5 h-1.5 rounded-full bg-primary"></span>
                    {feature}
                  </li>
                )}
              </For>
            </ul>
          </div>
        </Show>
      </div>
      <Show when={props.activationProof}>
        {(proof) => (
          <div class="mb-4 rounded-md border border-border bg-surface p-4">
            <p class="text-sm font-medium text-base-content">{proof().title}</p>
            <p class="mt-1 text-xs text-muted">{proof().body}</p>
            <ul class="mt-4 grid gap-3">
              <For each={proof().items}>
                {(item) => (
                  <li class="border-t border-border pt-3 first:border-t-0 first:pt-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <p class="text-sm font-medium text-base-content">{item.label}</p>
                      <span
                        class={`rounded-full px-2 py-1 text-[11px] font-medium ${proofStateClass(
                          item.state,
                        )}`}
                      >
                        {item.statusLabel}
                      </span>
                    </div>
                    <p class="mt-1 text-xs text-muted">{item.detail}</p>
                  </li>
                )}
              </For>
            </ul>
          </div>
        )}
      </Show>
      <Show when={props.planComparisonSummary.cards.length > 0}>
        <div class="mb-4 rounded-md border border-border bg-surface p-4">
          <p class="text-sm font-medium text-base-content">
            {SELF_HOSTED_PRO_BILLING_PRESENTATION.planComparisonSectionTitle}
          </p>
          <div class="mt-3 grid gap-3 lg:grid-cols-2">
            <For each={props.planComparisonSummary.cards}>
              {(card) => (
                <div class="rounded-md border border-border bg-surface-alt p-3">
                  <p class="text-sm font-medium text-base-content">{card.title}</p>
                  <p class="mt-1 text-xs text-muted">{card.body}</p>
                  <ul class="mt-3 grid gap-2">
                    <For each={card.highlights}>
                      {(feature) => (
                        <li class="text-xs text-base-content flex items-center gap-2">
                          <span class="w-1.5 h-1.5 rounded-full bg-primary"></span>
                          {feature}
                        </li>
                      )}
                    </For>
                  </ul>
                </div>
              )}
            </For>
          </div>
          <Show when={props.planComparisonSummary.action}>
            {(action) => (
              <UpgradeLink
                class="inline-flex items-center gap-1 mt-4 min-h-10 sm:min-h-9 rounded-md border border-border px-3 py-2 text-xs font-medium text-base-content hover:bg-surface-hover"
                destination={action().destination}
              >
                {action().label}
              </UpgradeLink>
            )}
          </Show>
        </div>
      </Show>
      <Show when={props.commercialMigrationNotice}>
        {(notice) => (
          <div class={`mb-4 rounded-md border p-3 text-sm ${notice().tone}`}>
            <p class="font-medium">{notice().title}</p>
            <p class="mt-1 text-xs opacity-90">{notice().body}</p>
          </div>
        )}
      </Show>
      <Show when={props.grandfatheredPriceNotice}>
        {(notice) => (
          <div class={`mb-4 rounded-md border p-3 text-sm ${notice().tone}`}>
            <p class="font-medium">{notice().title}</p>
            <p class="mt-1 text-xs opacity-90">{notice().body}</p>
          </div>
        )}
      </Show>
      <Show when={licenseEntitlementsLoadError()}>
        <div class="rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 text-sm text-amber-800 dark:text-amber-200">
          <p class="font-medium">Could not load license status</p>
          <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
            The license server could not be reached. Some features may be temporarily restricted.
          </p>
          <button
            type="button"
            class="mt-2 inline-flex min-h-10 sm:min-h-9 items-center gap-2 px-3 py-2 text-xs font-medium rounded-md border border-amber-300 dark:border-amber-700 text-amber-800 dark:text-amber-200 hover:bg-amber-100 dark:hover:bg-amber-800 transition-colors disabled:opacity-60"
            disabled={props.loading}
            onClick={props.onReload}
          >
            <RefreshCw class={`w-3 h-3 ${props.loading ? 'animate-spin' : ''}`} />
            Retry
          </button>
        </div>
      </Show>
      <Show when={!licenseEntitlementsLoadError()}>
        <Show
          when={!props.loading}
          fallback={<p class="text-sm ">{getLicenseStatusLoadingState().text}</p>}
        >
          <Show
            when={props.hasLicenseDetails}
            fallback={
              <div class="text-sm text-muted">{getNoActiveSelfHostedActivationState().text}</div>
            }
          >
            <CommercialStatGrid items={props.commercialPlanModel.summary} />

            <div class="grid gap-4 sm:grid-cols-2">
              <For each={props.commercialPlanModel.details}>
                {(item) => (
                  <div>
                    <p class="text-xs uppercase tracking-wide text-muted">{item.label}</p>
                    <p class="text-sm font-medium text-base-content">{item.value}</p>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </Show>
      </Show>
    </>
  );
};
