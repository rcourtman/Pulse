import { For, Show } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import {
  getResourceRoutingScopeLabel,
  getResourceSensitivityLabel,
} from '@/utils/resourcePolicyPresentation';
import {
  RESOURCE_ANALYSIS_LABEL,
  RESOURCE_SAFE_SUMMARY_LABEL,
} from '@/utils/resourceAnalysisPresentation';
import { ResourceChangeSummary } from './ResourceChangeSummary';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';

interface ResourceInvestigationContextTablesProps {
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
}

export const ResourceInvestigationContextTables: Component<
  ResourceInvestigationContextTablesProps
> = (props) => (
  <div class="overflow-hidden rounded border border-border bg-surface">
    <table class="w-full table-fixed text-[11px]">
      <Show when={props.drawer.resourceIntelligence()}>
        {(intel) => (
          <tbody class="divide-y divide-border">
            <tr class="bg-surface-alt">
              <th
                colspan="2"
                class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
              >
                {RESOURCE_ANALYSIS_LABEL}
              </th>
            </tr>
            <tr>
              <td class="w-[38%] px-2 py-1 text-muted">Health</td>
              <td class="px-2 py-1 text-right font-semibold text-base-content">
                {intel().health.grade} · {Math.round(intel().health.score)}/100
              </td>
            </tr>
            <tr>
              <td class="px-2 py-1 text-muted">Trend</td>
              <td class="px-2 py-1 text-right font-semibold capitalize text-base-content">
                {intel().health.trend}
              </td>
            </tr>
            <tr>
              <td class="px-2 py-1 text-muted">Notes</td>
              <td class="px-2 py-1 text-right font-semibold text-base-content">
                {intel().note_count}
              </td>
            </tr>
            <tr>
              <td colspan="2" class="px-2 py-2">
                <ResourceChangeSummary
                  class="space-y-0"
                  title="Latest canonical change"
                  changes={intel().recent_changes}
                  resolveResourceLabel={props.drawer.resolveResourceLabel}
                  maxChanges={1}
                  compact
                />
              </td>
            </tr>
          </tbody>
        )}
      </Show>

      <Show when={props.drawer.hasGovernanceData()}>
        <tbody class="divide-y divide-border border-t border-border">
          <tr class="bg-surface-alt">
            <th
              colspan="2"
              class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
            >
              Governance
            </th>
          </tr>
          <Show when={props.resource.policy}>
            <tr>
              <td class="w-[38%] px-2 py-1 text-muted">Sensitivity</td>
              <td class="px-2 py-1 text-right font-semibold text-base-content">
                {getResourceSensitivityLabel(props.resource.policy?.sensitivity)}
              </td>
            </tr>
            <tr>
              <td class="px-2 py-1 text-muted">Routing</td>
              <td class="px-2 py-1 text-right font-semibold text-base-content">
                {getResourceRoutingScopeLabel(props.resource.policy?.routing.scope)}
              </td>
            </tr>
          </Show>
          <Show
            when={props.drawer.policyRedactions().length > 0 || props.drawer.governanceSummary()}
          >
            <tr>
              <td class="px-2 py-1 text-muted">Redactions</td>
              <td class="px-2 py-1 text-right font-semibold text-base-content">
                {props.drawer.policyRedactions().length}
              </td>
            </tr>
          </Show>
          <Show when={props.drawer.policyRedactions().length > 0}>
            <tr>
              <td class="px-2 py-1 align-top text-muted">Redaction labels</td>
              <td class="px-2 py-1">
                <div class="flex flex-wrap justify-end gap-1">
                  <For each={props.drawer.policyRedactions()}>
                    {(label) => (
                      <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
                        {label}
                      </span>
                    )}
                  </For>
                </div>
              </td>
            </tr>
          </Show>
          <Show when={props.drawer.governanceSummary()}>
            <tr>
              <td class="px-2 py-1 align-top text-muted">{RESOURCE_SAFE_SUMMARY_LABEL}</td>
              <td
                class="px-2 py-1 text-left font-medium text-base-content"
                title={props.drawer.governanceSummary() ?? undefined}
              >
                <span class="block truncate">{props.drawer.governanceSummary()}</span>
              </td>
            </tr>
          </Show>
        </tbody>
      </Show>
    </table>
  </div>
);
