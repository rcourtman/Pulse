import { For } from 'solid-js';
import ShieldAlert from 'lucide-solid/icons/shield-alert';

import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

const counterRules = [
  {
    key: 'smartReallocated',
    label: 'Reallocated sectors',
    help: 'Alert when the current counter reaches this value.',
  },
  {
    key: 'smartPending',
    label: 'Pending sectors',
    help: 'Alert when sectors are waiting to be remapped.',
  },
  {
    key: 'smartUncorrectable',
    label: 'Uncorrectable sectors',
    help: 'Alert when offline scans find unreadable sectors.',
  },
  {
    key: 'smartMediaErrors',
    label: 'Media errors',
    help: 'Alert when the drive reports media or data-integrity errors.',
  },
  {
    key: 'smartCrcErrorDelta',
    label: 'CRC increase',
    help: 'Alert when the CRC counter grows by at least this much between reports.',
  },
] as const;

const percentageRules = [
  { key: 'smartLifeWarning', label: 'Life remaining warning', help: 'Warn below this value.' },
  { key: 'smartLifeCritical', label: 'Life remaining critical', help: 'Critical at or below.' },
  { key: 'smartSpareWarning', label: 'NVMe spare warning', help: 'Warn below this value.' },
  { key: 'smartSpareCritical', label: 'NVMe spare critical', help: 'Critical at or below.' },
] as const;

export function ThresholdsTableSMARTDefaultsCard(props: ThresholdsTableSectionProps) {
  const updateValue = (key: string, value: number) => {
    const normalized = Number.isFinite(value) ? Math.max(0, Math.trunc(value)) : 0;
    props.tableProps.setAgentDefaults((previous) => ({ ...previous, [key]: normalized }));
    props.tableProps.setHasUnsavedChanges(true);
  };

  return (
    <section
      class="rounded-lg border border-base-300 bg-surface p-4"
      aria-labelledby="smart-alert-defaults-title"
    >
      <div class="flex items-start gap-3">
        <ShieldAlert class="mt-0.5 h-5 w-5 shrink-0 text-warning" aria-hidden="true" />
        <div>
          <h3 id="smart-alert-defaults-title" class="font-semibold text-base-content">
            SMART alert rules
          </h3>
          <p class="mt-1 text-sm text-muted">
            Defaults for agent-reported disks. Set a numeric rule to 0 to disable it.
          </p>
        </div>
      </div>

      <label class="mt-4 flex min-h-11 items-center justify-between gap-4 rounded-md border border-base-300 px-3 py-2 sm:min-h-0">
        <span>
          <span class="block text-sm font-medium text-base-content">Failed health status</span>
          <span class="block text-xs text-muted">
            Alert when SMART reports a failed health state.
          </span>
        </span>
        <input
          type="checkbox"
          class="toggle toggle-warning toggle-sm"
          checked={(props.tableProps.agentDefaults.smartHealthFailure ?? 1) > 0}
          onChange={(event) =>
            updateValue('smartHealthFailure', event.currentTarget.checked ? 1 : 0)
          }
          aria-label="Alert on failed SMART health status"
        />
      </label>

      <div class="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        <For each={counterRules}>
          {(rule) => (
            <label class="rounded-md border border-base-300 px-3 py-2">
              <span class="block text-sm font-medium text-base-content">{rule.label}</span>
              <span class="block min-h-8 text-xs text-muted">{rule.help}</span>
              <input
                type="number"
                min="0"
                step="1"
                class="input input-bordered input-sm mt-2 w-full"
                value={props.tableProps.agentDefaults[rule.key] ?? 0}
                onInput={(event) => updateValue(rule.key, event.currentTarget.valueAsNumber)}
                aria-label={`${rule.label} threshold`}
              />
            </label>
          )}
        </For>
        <For each={percentageRules}>
          {(rule) => (
            <label class="rounded-md border border-base-300 px-3 py-2">
              <span class="block text-sm font-medium text-base-content">{rule.label}</span>
              <span class="block min-h-8 text-xs text-muted">{rule.help}</span>
              <div class="mt-2 flex items-center gap-2">
                <input
                  type="number"
                  min="0"
                  max="100"
                  step="1"
                  class="input input-bordered input-sm w-full"
                  value={props.tableProps.agentDefaults[rule.key] ?? 0}
                  onInput={(event) =>
                    updateValue(rule.key, Math.min(100, event.currentTarget.valueAsNumber))
                  }
                  aria-label={`${rule.label} percentage`}
                />
                <span class="text-sm text-muted">%</span>
              </div>
            </label>
          )}
        </For>
      </div>
    </section>
  );
}
