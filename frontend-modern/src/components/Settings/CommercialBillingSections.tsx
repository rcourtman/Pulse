import { Component, For, JSX, Show } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';

export type CommercialStatItem = {
  label: string;
  value: JSX.Element | string | number;
};

export type CommercialUsageMeterItem = {
  label: string;
  current: number;
  limit?: number;
  accentClass: string;
};

interface CommercialSectionProps {
  title: string;
  description: string;
  children: JSX.Element;
}

interface CommercialStatGridProps {
  items: readonly CommercialStatItem[];
  columns?: 'three' | 'four';
}

interface CommercialUsageMetersProps {
  title: string;
  items: readonly CommercialUsageMeterItem[];
}

interface CommercialBillingShellProps {
  title: string;
  description: string;
  icon: JSX.Element;
  action?: JSX.Element;
  loading?: boolean;
  loadingFallback?: JSX.Element;
  children: JSX.Element;
}

const usageRatio = (current: number, limit?: number) => {
  if (!limit || limit <= 0 || current <= 0) return 0;
  return Math.min(100, Math.round((current / limit) * 100));
};

export const CommercialSection: Component<CommercialSectionProps> = (props) => (
  <div class="space-y-2 border-t border-border pt-4">
    <h3 class="text-sm font-semibold text-base-content">{props.title}</h3>
    <p class="text-xs text-muted">{props.description}</p>
    <div class="space-y-4">{props.children}</div>
  </div>
);

export const CommercialStatGrid: Component<CommercialStatGridProps> = (props) => (
  <div
    classList={{
      'grid gap-3 sm:grid-cols-3': props.columns !== 'four',
      'grid gap-3 sm:grid-cols-2 lg:grid-cols-4': props.columns === 'four',
    }}
  >
    <For each={props.items}>
      {(item) => (
        <div class="rounded-md border border-border p-3">
          <p class="text-xs uppercase tracking-wide text-muted">{item.label}</p>
          <p class="mt-1 text-sm font-medium text-base-content">{item.value}</p>
        </div>
      )}
    </For>
  </div>
);

export const CommercialUsageMeters: Component<CommercialUsageMetersProps> = (props) => (
  <div class="space-y-3 rounded-md border border-border p-4">
    <h4 class="text-sm font-semibold text-base-content">{props.title}</h4>

    <For each={props.items}>
      {(item) => (
        <div class="space-y-1">
          <div class="flex items-center justify-between text-xs text-muted">
            <span>{item.label}</span>
            <span>
              {item.current}
              {typeof item.limit === 'number' ? ` / ${item.limit}` : ' / Unlimited'}
            </span>
          </div>
          <Show when={typeof item.limit === 'number'}>
            <div class="h-2 w-full rounded bg-surface-hover">
              <div
                class={`h-2 rounded ${item.accentClass}`}
                style={{ width: `${usageRatio(item.current, item.limit)}%` }}
              />
            </div>
          </Show>
        </div>
      )}
    </For>
  </div>
);

export const CommercialBillingShell: Component<CommercialBillingShellProps> = (props) => (
  <SettingsPanel
    title={props.title}
    description={props.description}
    icon={props.icon}
    action={props.action}
    bodyClass="space-y-5"
  >
    <Show when={!props.loading} fallback={props.loadingFallback}>
      {props.children}
    </Show>
  </SettingsPanel>
);
