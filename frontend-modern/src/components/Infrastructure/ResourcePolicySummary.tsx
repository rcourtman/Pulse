import { For, Show, type Component } from 'solid-js';
import type { IntelligencePolicyPostureSummary } from '@/types/aiIntelligence';
import {
  RESOURCE_POLICY_REDACTION_ORDER,
  RESOURCE_POLICY_ROUTING_ORDER,
  RESOURCE_POLICY_SENSITIVITY_ORDER,
  getResourceRedactionHintLabel,
  getResourceRoutingScopeLabel,
  getResourceSensitivityLabel,
} from '@/utils/resourcePolicyPresentation';

interface ResourcePolicySummaryProps {
  posture?: IntelligencePolicyPostureSummary | null;
  title?: string;
  class?: string;
}

export const ResourcePolicySummary: Component<ResourcePolicySummaryProps> = (props) => {
  const posture = () => props.posture;
  const className = () => props.class?.trim() ?? '';

  return (
    <Show when={posture()}>
      {(value) => (
        <div class={`rounded-md border border-border-subtle bg-base p-4 ${className()}`.trim()}>
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div>
              <h3 class="text-sm font-semibold text-base-content">
                {props.title ?? 'Data Governance'}
              </h3>
              <p class="mt-1 text-xs text-muted">{value().total_resources} governed resources</p>
            </div>
          </div>

          <dl class="mt-3 grid grid-cols-2 gap-2 text-sm">
            <For each={RESOURCE_POLICY_SENSITIVITY_ORDER}>
              {(sensitivity) => {
                const count = () => value().sensitivity_counts?.[sensitivity] ?? 0;
                return (
                  <div class="rounded-md bg-surface px-3 py-2">
                    <dt class="text-xs uppercase tracking-wide text-muted">
                      {getResourceSensitivityLabel(sensitivity)}
                    </dt>
                    <dd class="mt-1 font-semibold text-base-content">{count()}</dd>
                  </div>
                );
              }}
            </For>
            <For each={RESOURCE_POLICY_ROUTING_ORDER}>
              {(scope) => {
                const count = () => value().routing_counts?.[scope] ?? 0;
                return (
                  <div class="rounded-md bg-surface px-3 py-2">
                    <dt class="text-xs uppercase tracking-wide text-muted">
                      {getResourceRoutingScopeLabel(scope)}
                    </dt>
                    <dd class="mt-1 font-semibold text-base-content">{count()}</dd>
                  </div>
                );
              }}
            </For>
          </dl>

          <Show
            when={RESOURCE_POLICY_REDACTION_ORDER.some(
              (hint) => (value().redaction_counts?.[hint] ?? 0) > 0,
            )}
          >
            <div class="mt-2 flex flex-wrap gap-1">
              <For each={RESOURCE_POLICY_REDACTION_ORDER}>
                {(hint) => {
                  const count = value().redaction_counts?.[hint] ?? 0;
                  if (!count) return null;
                  return (
                    <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                      {getResourceRedactionHintLabel(hint)} {count}
                    </span>
                  );
                }}
              </For>
            </div>
          </Show>
        </div>
      )}
    </Show>
  );
};

export default ResourcePolicySummary;
