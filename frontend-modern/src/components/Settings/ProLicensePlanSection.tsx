import { Component, For, Show } from 'solid-js';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import { getUpgradeActionUrlOrFallback, licenseLoadError } from '@/stores/license';
import {
  getLicenseStatusLoadingState,
  getNoActiveProLicenseState,
} from '@/utils/licensePresentation';
import { CommercialStatGrid } from './CommercialBillingSections';

interface Notice {
  title: string;
  body: string;
  tone: string;
}

interface ProLicensePlanSectionProps {
  commercialMigrationNotice: Notice | null;
  commercialPlanModel: {
    summary: Array<{ label: string; value: string | number }>;
    details: Array<{ label: string; value: string | number }>;
  };
  entitlements: {
    in_grace_period?: boolean;
    grace_period_end?: string | null;
  } | null;
  formattedFeatures: string[];
  grandfatheredPriceNotice: Notice | null;
  hasLicenseDetails: boolean;
  hasPaidFeatures: boolean;
  loading: boolean;
  onReload: () => void;
  statusPresentation: {
    badgeClass: string;
    label: string;
  };
  trialActivationNotice: Notice | null;
  trialEnded: boolean;
}

const formatDate = (value?: string | null) => {
  if (!value) return 'Not available';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
};

export const ProLicensePlanSection: Component<ProLicensePlanSectionProps> = (props) => (
  <>
    <Show when={props.trialActivationNotice}>
      {(notice) => (
        <div class={`mb-4 rounded-md border p-3 text-sm ${notice().tone}`}>
          <p class="font-medium">{notice().title}</p>
          <p class="mt-1 text-xs opacity-90">{notice().body}</p>
        </div>
      )}
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
    <Show when={props.trialEnded && !licenseLoadError()}>
      <div class="mb-4 rounded-md border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 p-3 text-sm text-red-900 dark:text-red-100">
        <p class="font-medium">Your Pro trial has ended</p>
        <p class="text-xs text-red-800 dark:text-red-200 mt-1">Upgrade to keep Pro features.</p>
        <a
          class="inline-flex items-center gap-1 mt-2 text-xs font-medium text-red-900 dark:text-red-100 hover:underline"
          href={getUpgradeActionUrlOrFallback('trial_expired')}
          target="_blank"
          rel="noreferrer"
        >
          View Pro plans
        </a>
      </div>
    </Show>
    <Show when={licenseLoadError()}>
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
    <Show when={!licenseLoadError()}>
      <Show when={!props.loading} fallback={<p class="text-sm ">{getLicenseStatusLoadingState().text}</p>}>
        <div class="flex flex-wrap items-center gap-2">
          <span class={`px-2 py-1 text-xs font-medium rounded-full ${props.statusPresentation.badgeClass}`}>
            {props.statusPresentation.label}
          </span>
          <Show when={props.entitlements?.in_grace_period}>
            <span class="text-xs text-amber-700 dark:text-amber-300">
              Grace until {formatDate(props.entitlements?.grace_period_end)}
            </span>
          </Show>
        </div>

        <Show
          when={props.hasLicenseDetails}
          fallback={<div class="text-sm text-muted">{getNoActiveProLicenseState().text}</div>}
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

          <Show when={props.formattedFeatures.length > 0}>
            <div>
              <p class="text-xs uppercase tracking-wide text-muted mb-2">Features</p>
              <ul class="grid gap-2 sm:grid-cols-2">
                <For each={props.formattedFeatures}>
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
        </Show>

        <Show when={!props.hasPaidFeatures && !props.trialEnded}>
          <div class="rounded-md border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 p-3 text-sm text-amber-800 dark:text-amber-200">
            <p class="font-medium">Upgrade to Pro</p>
            <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
              Unlock Pulse Patrol, alert analysis, auto-fix, and more.
            </p>
            <a
              class="inline-flex items-center gap-1 mt-2 text-xs font-medium text-amber-800 dark:text-amber-200 hover:underline"
              href={getUpgradeActionUrlOrFallback('ai_autofix')}
              target="_blank"
              rel="noreferrer"
            >
              View Pro plans
            </a>
          </div>
        </Show>
      </Show>
    </Show>
  </>
);
