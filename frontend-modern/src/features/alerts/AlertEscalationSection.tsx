import { For, Show } from 'solid-js';

import { controlClass, formHelpText } from '@/components/shared/Form';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import {
  ALERT_CONFIG_ESCALATION_ADD_LABEL,
  ALERT_CONFIG_ESCALATION_AFTER_LABEL,
  ALERT_CONFIG_ESCALATION_DESCRIPTION,
  ALERT_CONFIG_ESCALATION_MINUTES_SUFFIX,
  ALERT_CONFIG_ESCALATION_NOTIFY_LABEL,
  ALERT_CONFIG_ESCALATION_REMOVE_TITLE,
  ALERT_CONFIG_ESCALATION_TITLE,
  getAlertConfigEscalationHelp,
  getAlertConfigEscalationNotifyLabel,
  getAlertConfigToggleStatusLabel,
} from '@/utils/alertConfigPresentation';

import type { EscalationConfig, EscalationNotifyTarget } from './types';

interface AlertEscalationSectionProps {
  escalation: EscalationConfig;
  setEscalationEnabled: (value: boolean) => void;
  setEscalationAfter: (index: number, value: string) => void;
  setEscalationNotify: (index: number, value: EscalationNotifyTarget) => void;
  removeEscalationLevel: (index: number) => void;
  addEscalationLevel: () => void;
}

export function AlertEscalationSection(props: AlertEscalationSectionProps) {
  return (
    <SettingsPanel
      title={ALERT_CONFIG_ESCALATION_TITLE}
      description={ALERT_CONFIG_ESCALATION_DESCRIPTION}
      action={
        <Toggle
          checked={props.escalation.enabled}
          onChange={(event) => props.setEscalationEnabled(event.currentTarget.checked)}
          containerClass="sm:self-start"
          label={
            <span class="text-xs font-medium text-muted">
              {getAlertConfigToggleStatusLabel(props.escalation.enabled)}
            </span>
          }
        />
      }
      class="space-y-4"
    >
      <Show when={props.escalation.enabled}>
        <div class="space-y-3">
          <p class={formHelpText}>{getAlertConfigEscalationHelp()}</p>
          <For each={props.escalation.levels}>
            {(level, index) => (
              <div class="flex items-center gap-3 rounded-md border border-border bg-surface-hover p-3">
                <div class="flex flex-1 flex-col gap-3 sm:grid sm:grid-cols-2 sm:items-center sm:gap-2">
                  <div class="flex items-center gap-2">
                    <span class="text-xs font-medium text-muted">
                      {ALERT_CONFIG_ESCALATION_AFTER_LABEL}
                    </span>
                    <input
                      type="number"
                      min="5"
                      max="180"
                      value={level.after}
                      onChange={(event) => props.setEscalationAfter(index(), event.currentTarget.value)}
                      class={`${controlClass('px-2 py-1 text-sm')} w-20`}
                    />
                    <span class="text-xs text-muted">
                      {ALERT_CONFIG_ESCALATION_MINUTES_SUFFIX}
                    </span>
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="text-xs font-medium text-muted">
                      {ALERT_CONFIG_ESCALATION_NOTIFY_LABEL}
                    </span>
                    <select
                      value={level.notify}
                      onChange={(event) =>
                        props.setEscalationNotify(
                          index(),
                          event.currentTarget.value as EscalationNotifyTarget,
                        )
                      }
                      class={`${controlClass('px-2 py-1 text-sm')} flex-1`}
                    >
                      <option value="email">{getAlertConfigEscalationNotifyLabel('email')}</option>
                      <option value="webhook">
                        {getAlertConfigEscalationNotifyLabel('webhook')}
                      </option>
                      <option value="all">{getAlertConfigEscalationNotifyLabel('all')}</option>
                    </select>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => props.removeEscalationLevel(index())}
                  class="rounded-md p-1.5 text-red-600 transition-colors hover:bg-red-100 dark:hover:bg-red-900"
                  title={ALERT_CONFIG_ESCALATION_REMOVE_TITLE}
                >
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                </button>
              </div>
            )}
          </For>

          <button
            type="button"
            onClick={props.addEscalationLevel}
            class="flex w-full items-center justify-center gap-2 rounded-md border-2 border-dashed border-border py-2 text-sm text-muted transition-all hover:border-slate-400 hover:bg-surface-hover"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 6v6m0 0v6m0-6h6m-6 0H6"
              />
            </svg>
            {ALERT_CONFIG_ESCALATION_ADD_LABEL}
          </button>
        </div>
      </Show>
    </SettingsPanel>
  );
}
