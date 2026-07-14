import { Component, For, Show } from 'solid-js';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import { Button, ButtonLink } from '@/components/shared/Button';
import { InlineNotice, type InlineNoticeTone } from '@/components/shared/InlineNotice';
import { UpgradeButtonLink } from '@/components/shared/UpgradeLink';
import { licenseEntitlementsLoadError } from '@/stores/licenseEntitlements';
import {
  getLicenseStatusLoadingState,
  getNoActiveSelfHostedActivationState,
  type SelfHostedPatrolControlActionIntent,
} from '@/utils/licensePresentation';
import { isExternalUpgradeHref, type UpgradeDestination } from '@/utils/upgradeNavigation';
import { SELF_HOSTED_PRO_BILLING_PRESENTATION } from './selfHostedBillingPresentation';
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
    actionIntent?: SelfHostedPatrolControlActionIntent;
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
    patrolControlAction?: {
      actionLabel: string;
      actionUrl: string;
      actionIntent: SelfHostedPatrolControlActionIntent;
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
  planStatus: {
    title: string;
    body: string;
    items: Array<{
      label: string;
      statusLabel: string;
      state: 'active' | 'partial' | 'missing';
      detail: string;
    }>;
  } | null;
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

const commercialMigrationNoticeTone = (notice: Notice): InlineNoticeTone =>
  notice.tone.includes('red-') ? 'danger' : 'warning';

const statusStateClass = (state: 'active' | 'partial' | 'missing') => {
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
  const getCurrentPlanPatrolControlAction = () => {
    const action = props.currentPlanSummary.patrolControlAction;
    if (!action) {
      return null;
    }
    if (props.activationSuccessSummary?.actionIntent === action.actionIntent) {
      return null;
    }
    return action;
  };
  const getPlanStatusSummary = () => {
    const status = props.planStatus;
    if (!status) {
      return '';
    }
    const attentionCount = status.items.filter((item) => item.state !== 'active').length;
    if (attentionCount > 0) {
      return attentionCount === 1
        ? '1 item needs attention'
        : `${attentionCount} items need attention`;
    }
    return status.items.length === 1 ? '1 item ready' : `${status.items.length} items ready`;
  };

  return (
    <>
      <Show when={props.activationSuccessSummary}>
        {(summary) => (
          <div
            role="status"
            aria-live="polite"
            class={`mb-4 rounded-md border p-3 text-sm ${summary().tone}`}
          >
            <p class="font-medium">{summary().title}</p>
            <p class="mt-1 text-xs opacity-90">{summary().body}</p>
            <Show
              when={(() => {
                const actionUrl = summary().actionUrl;
                const actionLabel = summary().actionLabel;
                return actionUrl && actionLabel ? { href: actionUrl, label: actionLabel } : null;
              })()}
            >
              {(action) => (
                <ButtonLink
                  href={action().href}
                  target={isExternalUpgradeHref(action().href) ? '_blank' : undefined}
                  variant="outline"
                  size="settingsActionXs"
                  class="mt-3 gap-1"
                >
                  {action().label}
                </ButtonLink>
              )}
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
                <UpgradeButtonLink
                  variant="outline"
                  size="settingsActionXs"
                  class="mt-3 gap-1"
                  destination={action().destination}
                >
                  {action().label}
                </UpgradeButtonLink>
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
            <UpgradeButtonLink
              variant="outline"
              size="settingsActionXs"
              class="mt-3 gap-1"
              destination={prompt().actionDestination}
            >
              {prompt().actionLabel}
            </UpgradeButtonLink>
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
            <ButtonLink
              href={action().actionUrl}
              target="_blank"
              variant="outline"
              size="settingsActionXs"
              class="mt-3 gap-1"
            >
              {action().actionLabel}
            </ButtonLink>
          )}
        </Show>
        <Show when={getCurrentPlanPatrolControlAction()}>
          {(action) => (
            <ButtonLink
              href={action().actionUrl}
              target={isExternalUpgradeHref(action().actionUrl) ? '_blank' : undefined}
              variant="outline"
              size="settingsActionXs"
              class="mt-3 gap-1"
            >
              {action().actionLabel}
            </ButtonLink>
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
      <Show when={props.planStatus}>
        {(status) => (
          <details class="mb-4 rounded-md border border-border bg-surface p-4">
            <summary class="cursor-pointer text-sm font-medium text-base-content">
              <span>{status().title}</span>{' '}
              <span class="text-xs font-normal text-muted">({getPlanStatusSummary()})</span>
            </summary>
            <div class="mt-3">
              <p class="text-xs text-muted">{status().body}</p>
              <ul class="mt-4 grid gap-3">
                <For each={status().items}>
                  {(item) => (
                    <li class="border-t border-border pt-3 first:border-t-0 first:pt-0">
                      <div class="flex flex-wrap items-center gap-2">
                        <p class="text-sm font-medium text-base-content">{item.label}</p>
                        <span
                          class={`rounded-full px-2 py-1 text-[11px] font-medium ${statusStateClass(
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
          </details>
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
          <div class="flex flex-wrap items-center gap-2">
            <Show when={props.planComparisonSummary.action}>
              {(action) => (
                <UpgradeButtonLink
                  variant="outline"
                  size="settingsActionXs"
                  class="mt-4 gap-1"
                  destination={action().destination}
                >
                  {action().label}
                </UpgradeButtonLink>
              )}
            </Show>
          </div>
        </div>
      </Show>
      <Show when={props.commercialMigrationNotice}>
        {(notice) => (
          <InlineNotice tone={commercialMigrationNoticeTone(notice())} class="mb-4">
            <p class="font-medium">{notice().title}</p>
            <p class="mt-1 text-xs">{notice().body}</p>
          </InlineNotice>
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
          <Button
            type="button"
            variant="warning"
            size="settingsActionXs"
            class="mt-2 gap-2"
            disabled={props.loading}
            onClick={props.onReload}
          >
            <RefreshCw class={`w-3 h-3 ${props.loading ? 'animate-spin' : ''}`} />
            Retry
          </Button>
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
