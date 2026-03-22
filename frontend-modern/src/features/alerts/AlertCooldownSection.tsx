import { Show } from 'solid-js';

import {
  controlClass,
  formHelpText,
  formField,
  labelClass,
} from '@/components/shared/Form';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import {
  ALERT_CONFIG_COOLDOWN_DESCRIPTION,
  ALERT_CONFIG_COOLDOWN_MAX_ALERTS_HELP,
  ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL,
  ALERT_CONFIG_COOLDOWN_MAX_ALERTS_SUFFIX,
  ALERT_CONFIG_COOLDOWN_PERIOD_HELP,
  ALERT_CONFIG_COOLDOWN_PERIOD_LABEL,
  ALERT_CONFIG_COOLDOWN_PERIOD_SUFFIX,
  ALERT_CONFIG_COOLDOWN_TITLE,
  getAlertConfigToggleStatusLabel,
} from '@/utils/alertConfigPresentation';

import type { CooldownConfig } from './types';

interface AlertCooldownSectionProps {
  cooldown: CooldownConfig;
  setCooldownEnabled: (value: boolean) => void;
  setCooldownMinutes: (value: string) => void;
  setCooldownMaxAlerts: (value: string) => void;
}

export function AlertCooldownSection(props: AlertCooldownSectionProps) {
  return (
    <SettingsPanel
      title={ALERT_CONFIG_COOLDOWN_TITLE}
      description={ALERT_CONFIG_COOLDOWN_DESCRIPTION}
      action={
        <Toggle
          checked={props.cooldown.enabled}
          onChange={(event) => props.setCooldownEnabled(event.currentTarget.checked)}
          containerClass="sm:self-start"
          label={
            <span class="text-xs font-medium text-muted">
              {getAlertConfigToggleStatusLabel(props.cooldown.enabled)}
            </span>
          }
        />
      }
      class="space-y-4"
    >
      <Show when={props.cooldown.enabled}>
        <div class="space-y-4">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_CONFIG_COOLDOWN_PERIOD_LABEL}
              </label>
              <div class="relative">
                <input
                  type="number"
                  min="5"
                  max="120"
                  value={props.cooldown.minutes}
                  onChange={(event) => props.setCooldownMinutes(event.currentTarget.value)}
                  class={controlClass('pr-16')}
                />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-muted">
                  {ALERT_CONFIG_COOLDOWN_PERIOD_SUFFIX}
                </span>
              </div>
              <p class={`${formHelpText} mt-1`}>{ALERT_CONFIG_COOLDOWN_PERIOD_HELP}</p>
            </div>

            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL}
              </label>
              <div class="relative">
                <input
                  type="number"
                  min="1"
                  max="10"
                  value={props.cooldown.maxAlerts}
                  onChange={(event) => props.setCooldownMaxAlerts(event.currentTarget.value)}
                  class={controlClass('pr-16')}
                />
                <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-muted">
                  {ALERT_CONFIG_COOLDOWN_MAX_ALERTS_SUFFIX}
                </span>
              </div>
              <p class={`${formHelpText} mt-1`}>{ALERT_CONFIG_COOLDOWN_MAX_ALERTS_HELP}</p>
            </div>
          </div>
        </div>
      </Show>
    </SettingsPanel>
  );
}
