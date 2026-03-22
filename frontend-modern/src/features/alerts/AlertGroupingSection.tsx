import { Show } from 'solid-js';

import {
  formHelpText,
  formField,
  labelClass,
} from '@/components/shared/Form';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import {
  ALERT_CONFIG_GROUPING_BY_GUEST,
  ALERT_CONFIG_GROUPING_BY_NODE,
  ALERT_CONFIG_GROUPING_DESCRIPTION,
  ALERT_CONFIG_GROUPING_STRATEGY_LABEL,
  ALERT_CONFIG_GROUPING_TITLE,
  ALERT_CONFIG_GROUPING_WINDOW_HELP,
  ALERT_CONFIG_GROUPING_WINDOW_LABEL,
  getAlertConfigToggleStatusLabel,
} from '@/utils/alertConfigPresentation';
import {
  getAlertGroupingCardClass,
  getAlertGroupingCheckboxClass,
} from '@/utils/alertGroupingPresentation';

import type { GroupingConfig } from './types';

interface AlertGroupingSectionProps {
  grouping: GroupingConfig;
  setGroupingEnabled: (value: boolean) => void;
  setGroupingWindow: (value: string) => void;
  setGroupingByNode: (value: boolean) => void;
  setGroupingByGuest: (value: boolean) => void;
}

export function AlertGroupingSection(props: AlertGroupingSectionProps) {
  return (
    <SettingsPanel
      title={ALERT_CONFIG_GROUPING_TITLE}
      description={ALERT_CONFIG_GROUPING_DESCRIPTION}
      action={
        <Toggle
          checked={props.grouping.enabled}
          onChange={(event) => props.setGroupingEnabled(event.currentTarget.checked)}
          containerClass="sm:self-start"
          label={
            <span class="text-xs font-medium text-muted">
              {getAlertConfigToggleStatusLabel(props.grouping.enabled)}
            </span>
          }
        />
      }
      class="space-y-4"
    >
      <Show when={props.grouping.enabled}>
        <div class="space-y-4">
          <div class={formField}>
            <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
              {ALERT_CONFIG_GROUPING_WINDOW_LABEL}
            </label>
            <div class="flex items-center gap-3">
              <input
                type="range"
                min="0"
                max="30"
                value={props.grouping.window}
                onChange={(event) => props.setGroupingWindow(event.currentTarget.value)}
                class="flex-1"
              />
              <div class="w-16 rounded-md bg-surface-alt px-2 py-1 text-center text-sm text-base-content">
                {props.grouping.window} min
              </div>
            </div>
            <p class={`${formHelpText} mt-1`}>{ALERT_CONFIG_GROUPING_WINDOW_HELP}</p>
          </div>

          <div>
            <span class={`${labelClass('text-xs uppercase tracking-[0.08em]')} mb-2 block`}>
              {ALERT_CONFIG_GROUPING_STRATEGY_LABEL}
            </span>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <label class={getAlertGroupingCardClass(props.grouping.byNode ?? false)}>
                <input
                  type="checkbox"
                  checked={props.grouping.byNode}
                  onChange={(event) => props.setGroupingByNode(event.currentTarget.checked)}
                  class="sr-only"
                />
                <div class={getAlertGroupingCheckboxClass(props.grouping.byNode ?? false)}>
                  <Show when={props.grouping.byNode}>
                    <svg
                      class="h-3 w-3 text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="3"
                    >
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                  </Show>
                </div>
                <span class="text-sm font-medium text-base-content">
                  {ALERT_CONFIG_GROUPING_BY_NODE}
                </span>
              </label>

              <label class={getAlertGroupingCardClass(props.grouping.byGuest ?? false)}>
                <input
                  type="checkbox"
                  checked={props.grouping.byGuest}
                  onChange={(event) => props.setGroupingByGuest(event.currentTarget.checked)}
                  class="sr-only"
                />
                <div class={getAlertGroupingCheckboxClass(props.grouping.byGuest ?? false)}>
                  <Show when={props.grouping.byGuest}>
                    <svg
                      class="h-3 w-3 text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="3"
                    >
                      <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                    </svg>
                  </Show>
                </div>
                <span class="text-sm font-medium text-base-content">
                  {ALERT_CONFIG_GROUPING_BY_GUEST}
                </span>
              </label>
            </div>
          </div>
        </div>
      </Show>
    </SettingsPanel>
  );
}
