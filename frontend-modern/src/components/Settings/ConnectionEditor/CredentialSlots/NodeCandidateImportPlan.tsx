import { For, Show, type Component } from 'solid-js';
import CheckCircle2 from 'lucide-solid/icons/check-circle-2';
import ClipboardCheck from 'lucide-solid/icons/clipboard-check';
import KeyRound from 'lucide-solid/icons/key-round';
import Search from 'lucide-solid/icons/search';
import ShieldCheck from 'lucide-solid/icons/shield-check';

import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';
import { Button } from '@/components/shared/Button';
import { formCheckbox } from '@/components/shared/Form';
import type { NodeImportPlan, NodeImportPlanStep } from '../../infrastructureImportPlanModel';
import { MonitoredSystemImpactPreview } from '../../MonitoredSystemImpactPreview';

interface NodeCandidateImportPlanProps {
  plan: NodeImportPlan;
  approved: boolean;
  onApprovedChange: (approved: boolean) => void;
  onPreviewImpact: () => void;
  preview: MonitoredSystemLedgerPreviewResponse | null;
  previewing: boolean;
  previewError: string | null;
}

const stepIcon = (step: NodeImportPlanStep) => {
  switch (step.id) {
    case 'candidate':
      return Search;
    case 'credentials':
      return KeyRound;
    case 'dry-run':
      return ClipboardCheck;
    case 'approval':
      return ShieldCheck;
    case 'verification':
      return CheckCircle2;
  }
};

export const NodeCandidateImportPlan: Component<NodeCandidateImportPlanProps> = (props) => {
  return (
    <section
      aria-label="Candidate import plan"
      class="rounded-md border border-blue-200 bg-blue-50/70 px-4 py-4 dark:border-blue-900/60 dark:bg-blue-950/30"
    >
      <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div class="space-y-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-sm font-semibold text-base-content">Candidate import plan</h3>
            <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-800 dark:bg-blue-900/70 dark:text-blue-200">
              {props.plan.sourceLabel}
            </span>
          </div>
          <p class="text-xs leading-5 text-muted">
            {props.plan.name} · {props.plan.endpoint}
          </p>
          <Show when={props.plan.detectedVersion}>
            {(version) => <p class="text-xs text-muted">{version()}</p>}
          </Show>
        </div>
        <Button
          type="button"
          variant="secondary"
          size="settingsAction"
          onClick={props.onPreviewImpact}
          disabled={!props.plan.previewRequest || props.previewing}
          class="self-start gap-2"
        >
          <ClipboardCheck class="h-4 w-4" aria-hidden="true" />
          {props.previewing ? 'Previewing...' : 'Preview impact'}
        </Button>
      </div>

      <div class="mt-4 grid gap-3 md:grid-cols-5">
        <For each={props.plan.steps}>
          {(step) => {
            const Icon = stepIcon(step);
            return (
              <div class="rounded-md border border-blue-200 bg-surface px-3 py-3 text-xs dark:border-blue-900/70">
                <div class="flex items-center gap-2 font-medium text-base-content">
                  <Icon class="h-3.5 w-3.5 text-blue-600 dark:text-blue-300" aria-hidden="true" />
                  {step.title}
                </div>
                <p class="mt-1 leading-5 text-muted">{step.detail}</p>
              </div>
            );
          }}
        </For>
      </div>

      <div class="mt-4">
        <MonitoredSystemImpactPreview
          preview={props.preview}
          loading={props.previewing}
          error={props.previewError}
          errorTitle="Could not preview import impact"
        />
      </div>

      <label class="mt-4 flex items-start gap-3 rounded-md border border-blue-200 bg-surface px-3 py-3 text-sm dark:border-blue-900/70">
        <input
          type="checkbox"
          checked={props.approved}
          onChange={(event) => props.onApprovedChange(event.currentTarget.checked)}
          class={`${formCheckbox} mt-0.5`}
        />
        <span>
          <span class="block font-medium text-base-content">Approve this import plan</span>
          <span class="mt-1 block text-xs leading-5 text-muted">
            Approving allows this {props.plan.sourceLabel} source to be added with the current
            endpoint, credential path, and collection scope: {props.plan.coverageLabel}.
          </span>
        </span>
      </label>
    </section>
  );
};

export default NodeCandidateImportPlan;
