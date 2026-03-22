import { For, Show } from 'solid-js';

import { controlClass, formField, labelClass } from '@/components/shared/Form';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import {
  ALERT_CONFIG_QUIET_HOURS_DESCRIPTION,
  ALERT_CONFIG_QUIET_HOURS_END_TIME_LABEL,
  ALERT_CONFIG_QUIET_HOURS_START_TIME_LABEL,
  ALERT_CONFIG_QUIET_HOURS_TIMEZONE_LABEL,
  ALERT_CONFIG_QUIET_HOURS_TITLE,
} from '@/utils/alertConfigPresentation';
import {
  getAlertQuietDayButtonClass,
  getAlertQuietSuppressCardClass,
  getAlertQuietSuppressCheckboxClass,
} from '@/utils/alertSchedulePresentation';

import { ALERT_SCHEDULE_DAYS, ALERT_SCHEDULE_TIMEZONES } from './useAlertScheduleState';
import type { QuietHoursConfig } from './types';

interface QuietSuppressOption {
  key: keyof QuietHoursConfig['suppress'];
  label: string;
  description: string;
}

interface AlertQuietHoursSectionProps {
  quietHours: QuietHoursConfig;
  quietHourSuppressOptions: QuietSuppressOption[];
  weekdaysOnly: boolean;
  weekendsOnly: boolean;
  setQuietHoursEnabled: (value: boolean) => void;
  setQuietHoursStart: (value: string) => void;
  setQuietHoursEnd: (value: string) => void;
  setQuietHoursTimezone: (value: string) => void;
  toggleQuietDay: (day: keyof QuietHoursConfig['days']) => void;
  setQuietSuppressCategory: (category: keyof QuietHoursConfig['suppress'], value: boolean) => void;
}

export function AlertQuietHoursSection(props: AlertQuietHoursSectionProps) {
  return (
    <SettingsPanel
      title={ALERT_CONFIG_QUIET_HOURS_TITLE}
      description={ALERT_CONFIG_QUIET_HOURS_DESCRIPTION}
      action={
        <Toggle
          checked={props.quietHours.enabled}
          onChange={(event) => props.setQuietHoursEnabled(event.currentTarget.checked)}
          containerClass="sm:self-start"
          label={
            <span class="text-xs font-medium text-muted">
              {props.quietHours.enabled ? 'Enabled' : 'Disabled'}
            </span>
          }
        />
      }
      class="space-y-4"
    >
      <Show when={props.quietHours.enabled}>
        <div class="space-y-4">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_CONFIG_QUIET_HOURS_START_TIME_LABEL}
              </label>
              <input
                type="time"
                value={props.quietHours.start}
                onChange={(event) => props.setQuietHoursStart(event.currentTarget.value)}
                class={controlClass('font-mono')}
              />
            </div>
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_CONFIG_QUIET_HOURS_END_TIME_LABEL}
              </label>
              <input
                type="time"
                value={props.quietHours.end}
                onChange={(event) => props.setQuietHoursEnd(event.currentTarget.value)}
                class={controlClass('font-mono')}
              />
            </div>
            <div class={formField}>
              <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                {ALERT_CONFIG_QUIET_HOURS_TIMEZONE_LABEL}
              </label>
              <select
                value={props.quietHours.timezone}
                onChange={(event) => props.setQuietHoursTimezone(event.currentTarget.value)}
                class={controlClass('pr-8')}
              >
                <For each={ALERT_SCHEDULE_TIMEZONES}>
                  {(timezone) => <option value={timezone}>{timezone}</option>}
                </For>
              </select>
            </div>
          </div>

          <div>
            <span class={`${labelClass('text-xs uppercase tracking-[0.08em]')} mb-2 block`}>
              Quiet days
            </span>
            <div class="grid grid-cols-7 gap-1">
              <For each={ALERT_SCHEDULE_DAYS}>
                {(day) => (
                  <button
                    type="button"
                    onClick={() => props.toggleQuietDay(day.id)}
                    title={day.fullLabel}
                    class={getAlertQuietDayButtonClass(props.quietHours.days[day.id])}
                  >
                    {day.label}
                  </button>
                )}
              </For>
            </div>
            <p class="mt-2 text-xs text-muted">
              <Show when={props.weekdaysOnly}>Weekdays only</Show>
              <Show when={props.weekendsOnly}>Weekends only</Show>
            </p>
          </div>

          <div class="space-y-3 border-t border-border pt-4">
            <span class={`${labelClass('text-xs uppercase tracking-[0.08em]')} block`}>
              Suppress categories
            </span>
            <p class="text-xs text-muted">
              Critical alerts in selected categories will stay silent during quiet hours.
            </p>
            <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:gap-3">
              <For each={props.quietHourSuppressOptions}>
                {(option) => (
                  <label
                    class={getAlertQuietSuppressCardClass(
                      props.quietHours.suppress[option.key],
                    )}
                  >
                    <input
                      type="checkbox"
                      checked={props.quietHours.suppress[option.key]}
                      onChange={(event) =>
                        props.setQuietSuppressCategory(option.key, event.currentTarget.checked)
                      }
                      class="sr-only"
                    />
                    <div
                      class={getAlertQuietSuppressCheckboxClass(
                        props.quietHours.suppress[option.key],
                      )}
                    >
                      <Show when={props.quietHours.suppress[option.key]}>
                        <svg
                          class="h-3 w-3 text-white"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                          stroke-width="3"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M5 13l4 4L19 7"
                          />
                        </svg>
                      </Show>
                    </div>
                    <div>
                      <p class="text-sm font-medium text-base-content">{option.label}</p>
                      <p class="text-xs text-muted">{option.description}</p>
                    </div>
                  </label>
                )}
              </For>
            </div>
          </div>
        </div>
      </Show>
    </SettingsPanel>
  );
}
