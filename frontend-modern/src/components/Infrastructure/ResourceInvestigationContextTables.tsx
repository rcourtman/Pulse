import { For, type Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { DetailSectionTable } from '@/components/shared/DetailSectionTable';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailSection,
} from '@/components/shared/detailSectionModel';
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

const redactionLabelsRow = (labels: string[]): DetailRow | null => {
  if (labels.length === 0) return null;
  return {
    label: 'Redaction labels',
    value: labels.join(', '),
    valueContent: (
      <div class="flex flex-wrap gap-1">
        <For each={labels}>
          {(label) => (
            <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]">
              {label}
            </span>
          )}
        </For>
      </div>
    ),
  };
};

export const ResourceInvestigationContextTables: Component<
  ResourceInvestigationContextTablesProps
> = (props) => {
  const sections = (): DetailSection[] => {
    const intelligence = props.drawer.resourceIntelligence();
    const redactions = props.drawer.policyRedactions();

    return compactDetailSections([
      intelligence
        ? {
            label: RESOURCE_ANALYSIS_LABEL,
            rows: compactDetailRows([
              makeDetailRow(
                'Health',
                `${intelligence.health.grade} · ${Math.round(intelligence.health.score)}/100`,
              ),
              makeDetailRow('Trend', intelligence.health.trend, { valueClass: 'capitalize' }),
              makeDetailRow('Notes', String(intelligence.note_count)),
            ]),
            footerContent: (
              <ResourceChangeSummary
                class="space-y-0"
                title="Latest canonical change"
                changes={intelligence.recent_changes}
                resolveResourceLabel={props.drawer.resolveResourceLabel}
                maxChanges={1}
                compact
              />
            ),
          }
        : null,
      props.drawer.hasGovernanceData()
        ? {
            label: 'Governance',
            rows: compactDetailRows([
              props.resource.policy
                ? makeDetailRow(
                    'Sensitivity',
                    getResourceSensitivityLabel(props.resource.policy.sensitivity),
                  )
                : null,
              props.resource.policy
                ? makeDetailRow(
                    'Routing',
                    getResourceRoutingScopeLabel(props.resource.policy.routing.scope),
                  )
                : null,
              redactions.length > 0 || props.drawer.governanceSummary()
                ? makeDetailRow('Redactions', String(redactions.length))
                : null,
              redactionLabelsRow(redactions),
              makeDetailRow(RESOURCE_SAFE_SUMMARY_LABEL, props.drawer.governanceSummary(), {
                wrap: true,
              }),
            ]),
          }
        : null,
    ]);
  };

  return <DetailSectionTable sections={sections()} />;
};
