import { For, Show } from 'solid-js';

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
  ALERT_CONFIG_ESCALATION_ADD_LABEL,
  ALERT_CONFIG_ESCALATION_AFTER_LABEL,
  ALERT_CONFIG_ESCALATION_DESCRIPTION,
  ALERT_CONFIG_ESCALATION_MINUTES_SUFFIX,
  ALERT_CONFIG_ESCALATION_NOTIFY_LABEL,
  ALERT_CONFIG_ESCALATION_REMOVE_TITLE,
  ALERT_CONFIG_ESCALATION_TITLE,
  ALERT_CONFIG_GROUPING_BY_GUEST,
  ALERT_CONFIG_GROUPING_BY_NODE,
  ALERT_CONFIG_GROUPING_DESCRIPTION,
  ALERT_CONFIG_GROUPING_STRATEGY_LABEL,
  ALERT_CONFIG_GROUPING_TITLE,
  ALERT_CONFIG_GROUPING_WINDOW_HELP,
  ALERT_CONFIG_GROUPING_WINDOW_LABEL,
  ALERT_CONFIG_QUIET_HOURS_DESCRIPTION,
  ALERT_CONFIG_QUIET_HOURS_END_TIME_LABEL,
  ALERT_CONFIG_QUIET_HOURS_START_TIME_LABEL,
  ALERT_CONFIG_QUIET_HOURS_TIMEZONE_LABEL,
  ALERT_CONFIG_QUIET_HOURS_TITLE,
  ALERT_CONFIG_RECOVERY_DESCRIPTION,
  ALERT_CONFIG_RECOVERY_TITLE,
  ALERT_CONFIG_SCHEDULING_DESCRIPTION,
  ALERT_CONFIG_SCHEDULING_TITLE,
  ALERT_CONFIG_SUMMARY_DESCRIPTION,
  ALERT_CONFIG_SUMMARY_TITLE,
  getAlertConfigEscalationHelp,
  getAlertConfigEscalationNotifyLabel,
  getAlertConfigQuietHourSuppressOptions,
  getAlertConfigRecoveryHelp,
  getAlertConfigResetDefaultsLabel,
  getAlertConfigResetDefaultsTitle,
  getAlertConfigSummaryAllDisabled,
  getAlertConfigSummaryCooldown,
  getAlertConfigSummaryEscalation,
  getAlertConfigSummaryGrouping,
  getAlertConfigSummaryQuietHours,
  getAlertConfigSummaryRecoveryEnabled,
  getAlertConfigSummarySuppressing,
  getAlertConfigToggleStatusLabel,
} from '@/utils/alertConfigPresentation';
import {
  getAlertGroupingCardClass,
  getAlertGroupingCheckboxClass,
} from '@/utils/alertGroupingPresentation';
import {
  getAlertQuietDayButtonClass,
  getAlertQuietSuppressCardClass,
  getAlertQuietSuppressCheckboxClass,
} from '@/utils/alertSchedulePresentation';

import {
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  createDefaultResolveNotifications,
  fallbackMaxAlertsPerHour,
} from '../helpers';
import type {
  CooldownConfig,
  EscalationConfig,
  EscalationLevel,
  EscalationNotifyTarget,
  GroupingConfig,
  QuietHoursConfig,
} from '../types';
import { fallbackCooldownMinutes } from '../types';

export interface ScheduleTabProps {
  setHasUnsavedChanges: (value: boolean) => void;
  quietHours: () => QuietHoursConfig;
  setQuietHours: (value: QuietHoursConfig) => void;
  cooldown: () => CooldownConfig;
  setCooldown: (value: CooldownConfig) => void;
  grouping: () => GroupingConfig;
  setGrouping: (value: GroupingConfig) => void;
  notifyOnResolve: () => boolean;
  setNotifyOnResolve: (value: boolean) => void;
  escalation: () => EscalationConfig;
  setEscalation: (value: EscalationConfig) => void;
}

const timezones = [
  'UTC',
  'Africa/Cairo',
  'Africa/Johannesburg',
  'Africa/Lagos',
  'Africa/Nairobi',
  'America/Anchorage',
  'America/Argentina/Buenos_Aires',
  'America/Bogota',
  'America/Caracas',
  'America/Chicago',
  'America/Denver',
  'America/Halifax',
  'America/Lima',
  'America/Los_Angeles',
  'America/Mexico_City',
  'America/New_York',
  'America/Phoenix',
  'America/Santiago',
  'America/Sao_Paulo',
  'America/St_Johns',
  'America/Toronto',
  'America/Vancouver',
  'Asia/Bangkok',
  'Asia/Dhaka',
  'Asia/Dubai',
  'Asia/Hong_Kong',
  'Asia/Jakarta',
  'Asia/Jerusalem',
  'Asia/Karachi',
  'Asia/Kolkata',
  'Asia/Kuala_Lumpur',
  'Asia/Manila',
  'Asia/Riyadh',
  'Asia/Seoul',
  'Asia/Shanghai',
  'Asia/Singapore',
  'Asia/Taipei',
  'Asia/Tehran',
  'Asia/Tokyo',
  'Australia/Adelaide',
  'Australia/Brisbane',
  'Australia/Melbourne',
  'Australia/Perth',
  'Australia/Sydney',
  'Europe/Amsterdam',
  'Europe/Athens',
  'Europe/Berlin',
  'Europe/Brussels',
  'Europe/Budapest',
  'Europe/Copenhagen',
  'Europe/Dublin',
  'Europe/Helsinki',
  'Europe/Istanbul',
  'Europe/Lisbon',
  'Europe/London',
  'Europe/Madrid',
  'Europe/Moscow',
  'Europe/Oslo',
  'Europe/Paris',
  'Europe/Prague',
  'Europe/Rome',
  'Europe/Stockholm',
  'Europe/Vienna',
  'Europe/Warsaw',
  'Europe/Zurich',
  'Pacific/Auckland',
  'Pacific/Fiji',
  'Pacific/Guam',
  'Pacific/Honolulu',
];

const days = [
  { id: 'monday', label: 'M', fullLabel: 'Monday' },
  { id: 'tuesday', label: 'T', fullLabel: 'Tuesday' },
  { id: 'wednesday', label: 'W', fullLabel: 'Wednesday' },
  { id: 'thursday', label: 'T', fullLabel: 'Thursday' },
  { id: 'friday', label: 'F', fullLabel: 'Friday' },
  { id: 'saturday', label: 'S', fullLabel: 'Saturday' },
  { id: 'sunday', label: 'S', fullLabel: 'Sunday' },
];

export function ScheduleTab(props: ScheduleTabProps) {
  const quietHourSuppressOptions = getAlertConfigQuietHourSuppressOptions();

  const resetToDefaults = () => {
    props.setQuietHours(createDefaultQuietHours());
    props.setCooldown(createDefaultCooldown());
    props.setGrouping(createDefaultGrouping());
    props.setNotifyOnResolve(createDefaultResolveNotifications());
    props.setEscalation(createDefaultEscalation());
    props.setHasUnsavedChanges(true);
  };

  return (
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h3 class="text-base font-semibold text-base-content">
            {ALERT_CONFIG_SCHEDULING_TITLE}
          </h3>
          <p class="mt-1 text-sm text-muted">{ALERT_CONFIG_SCHEDULING_DESCRIPTION}</p>
        </div>
        <button
          type="button"
          onClick={resetToDefaults}
          class="inline-flex items-center gap-2 self-start rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium shadow-sm transition-colors hover:bg-surface-hover"
          title={getAlertConfigResetDefaultsTitle()}
        >
          <svg
            class="h-4 w-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          {getAlertConfigResetDefaultsLabel()}
        </button>
      </div>

      <div class="grid gap-6 xl:grid-cols-2">
        <SettingsPanel
          title={ALERT_CONFIG_QUIET_HOURS_TITLE}
          description={ALERT_CONFIG_QUIET_HOURS_DESCRIPTION}
          action={
            <Toggle
              checked={props.quietHours().enabled}
              onChange={(event) => {
                props.setQuietHours({
                  ...props.quietHours(),
                  enabled: event.currentTarget.checked,
                });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-muted">
                  {props.quietHours().enabled ? 'Enabled' : 'Disabled'}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={props.quietHours().enabled}>
            <div class="space-y-4">
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_CONFIG_QUIET_HOURS_START_TIME_LABEL}
                  </label>
                  <input
                    type="time"
                    value={props.quietHours().start}
                    onChange={(event) => {
                      props.setQuietHours({
                        ...props.quietHours(),
                        start: event.currentTarget.value,
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class={controlClass('font-mono')}
                  />
                </div>
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_CONFIG_QUIET_HOURS_END_TIME_LABEL}
                  </label>
                  <input
                    type="time"
                    value={props.quietHours().end}
                    onChange={(event) => {
                      props.setQuietHours({
                        ...props.quietHours(),
                        end: event.currentTarget.value,
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class={controlClass('font-mono')}
                  />
                </div>
                <div class={formField}>
                  <label class={labelClass('text-xs uppercase tracking-[0.08em]')}>
                    {ALERT_CONFIG_QUIET_HOURS_TIMEZONE_LABEL}
                  </label>
                  <select
                    value={props.quietHours().timezone}
                    onChange={(event) => {
                      props.setQuietHours({
                        ...props.quietHours(),
                        timezone: event.currentTarget.value,
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class={controlClass('pr-8')}
                  >
                    <For each={timezones}>{(timezone) => <option value={timezone}>{timezone}</option>}</For>
                  </select>
                </div>
              </div>

              <div>
                <span class={`${labelClass('text-xs uppercase tracking-[0.08em]')} mb-2 block`}>
                  Quiet days
                </span>
                <div class="grid grid-cols-7 gap-1">
                  <For each={days}>
                    {(day) => (
                      <button
                        type="button"
                        onClick={() => {
                          const currentDays = props.quietHours().days;
                          props.setQuietHours({
                            ...props.quietHours(),
                            days: { ...currentDays, [day.id]: !currentDays[day.id] },
                          });
                          props.setHasUnsavedChanges(true);
                        }}
                        title={day.fullLabel}
                        class={getAlertQuietDayButtonClass(props.quietHours().days[day.id])}
                      >
                        {day.label}
                      </button>
                    )}
                  </For>
                </div>
                <p class="mt-2 text-xs text-muted">
                  <Show
                    when={
                      props.quietHours().days.monday &&
                      props.quietHours().days.tuesday &&
                      props.quietHours().days.wednesday &&
                      props.quietHours().days.thursday &&
                      props.quietHours().days.friday &&
                      !props.quietHours().days.saturday &&
                      !props.quietHours().days.sunday
                    }
                  >
                    Weekdays only
                  </Show>
                  <Show
                    when={
                      !props.quietHours().days.monday &&
                      !props.quietHours().days.tuesday &&
                      !props.quietHours().days.wednesday &&
                      !props.quietHours().days.thursday &&
                      !props.quietHours().days.friday &&
                      props.quietHours().days.saturday &&
                      props.quietHours().days.sunday
                    }
                  >
                    Weekends only
                  </Show>
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
                  <For each={quietHourSuppressOptions}>
                    {(option) => (
                      <label
                        class={getAlertQuietSuppressCardClass(
                          props.quietHours().suppress[option.key],
                        )}
                      >
                        <input
                          type="checkbox"
                          checked={props.quietHours().suppress[option.key]}
                          onChange={(event) => {
                            props.setQuietHours({
                              ...props.quietHours(),
                              suppress: {
                                ...props.quietHours().suppress,
                                [option.key]: event.currentTarget.checked,
                              },
                            });
                            props.setHasUnsavedChanges(true);
                          }}
                          class="sr-only"
                        />
                        <div
                          class={getAlertQuietSuppressCheckboxClass(
                            props.quietHours().suppress[option.key],
                          )}
                        >
                          <Show when={props.quietHours().suppress[option.key]}>
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

        <SettingsPanel
          title={ALERT_CONFIG_COOLDOWN_TITLE}
          description={ALERT_CONFIG_COOLDOWN_DESCRIPTION}
          action={
            <Toggle
              checked={props.cooldown().enabled}
              onChange={(event) => {
                const enabled = event.currentTarget.checked;
                const current = props.cooldown();
                const next: CooldownConfig = {
                  ...current,
                  enabled,
                };
                if (enabled) {
                  next.minutes = fallbackCooldownMinutes(current.minutes);
                  next.maxAlerts = fallbackMaxAlertsPerHour(current.maxAlerts);
                }
                props.setCooldown(next);
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-muted">
                  {getAlertConfigToggleStatusLabel(props.cooldown().enabled)}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={props.cooldown().enabled}>
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
                      value={props.cooldown().minutes}
                      onChange={(event) => {
                        const value = Number.parseInt(event.currentTarget.value, 10);
                        props.setCooldown({
                          ...props.cooldown(),
                          minutes: Number.isNaN(value) ? props.cooldown().minutes : value,
                        });
                        props.setHasUnsavedChanges(true);
                      }}
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
                      value={props.cooldown().maxAlerts}
                      onChange={(event) => {
                        const value = Number.parseInt(event.currentTarget.value, 10);
                        props.setCooldown({
                          ...props.cooldown(),
                          maxAlerts: Number.isNaN(value) ? props.cooldown().maxAlerts : value,
                        });
                        props.setHasUnsavedChanges(true);
                      }}
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

        <SettingsPanel
          title={ALERT_CONFIG_GROUPING_TITLE}
          description={ALERT_CONFIG_GROUPING_DESCRIPTION}
          action={
            <Toggle
              checked={props.grouping().enabled}
              onChange={(event) => {
                props.setGrouping({
                  ...props.grouping(),
                  enabled: event.currentTarget.checked,
                });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-muted">
                  {getAlertConfigToggleStatusLabel(props.grouping().enabled)}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={props.grouping().enabled}>
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
                    value={props.grouping().window}
                    onChange={(event) => {
                      props.setGrouping({
                        ...props.grouping(),
                        window: Number.parseInt(event.currentTarget.value, 10),
                      });
                      props.setHasUnsavedChanges(true);
                    }}
                    class="flex-1"
                  />
                  <div class="w-16 rounded-md bg-surface-alt px-2 py-1 text-center text-sm text-base-content">
                    {props.grouping().window} min
                  </div>
                </div>
                <p class={`${formHelpText} mt-1`}>{ALERT_CONFIG_GROUPING_WINDOW_HELP}</p>
              </div>

              <div>
                <span class={`${labelClass('text-xs uppercase tracking-[0.08em]')} mb-2 block`}>
                  {ALERT_CONFIG_GROUPING_STRATEGY_LABEL}
                </span>
                <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
                  <label class={getAlertGroupingCardClass(props.grouping().byNode ?? false)}>
                    <input
                      type="checkbox"
                      checked={props.grouping().byNode}
                      onChange={(event) => {
                        props.setGrouping({
                          ...props.grouping(),
                          byNode: event.currentTarget.checked,
                        });
                        props.setHasUnsavedChanges(true);
                      }}
                      class="sr-only"
                    />
                    <div class={getAlertGroupingCheckboxClass(props.grouping().byNode ?? false)}>
                      <Show when={props.grouping().byNode}>
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

                  <label class={getAlertGroupingCardClass(props.grouping().byGuest ?? false)}>
                    <input
                      type="checkbox"
                      checked={props.grouping().byGuest}
                      onChange={(event) => {
                        props.setGrouping({
                          ...props.grouping(),
                          byGuest: event.currentTarget.checked,
                        });
                        props.setHasUnsavedChanges(true);
                      }}
                      class="sr-only"
                    />
                    <div class={getAlertGroupingCheckboxClass(props.grouping().byGuest ?? false)}>
                      <Show when={props.grouping().byGuest}>
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

        <SettingsPanel
          title={ALERT_CONFIG_RECOVERY_TITLE}
          description={ALERT_CONFIG_RECOVERY_DESCRIPTION}
          action={
            <Toggle
              checked={props.notifyOnResolve()}
              onChange={(event) => {
                props.setNotifyOnResolve(event.currentTarget.checked);
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-muted">
                  {getAlertConfigToggleStatusLabel(props.notifyOnResolve())}
                </span>
              }
            />
          }
          class="space-y-3"
        >
          <p class={formHelpText}>{getAlertConfigRecoveryHelp()}</p>
        </SettingsPanel>

        <SettingsPanel
          title={ALERT_CONFIG_ESCALATION_TITLE}
          description={ALERT_CONFIG_ESCALATION_DESCRIPTION}
          action={
            <Toggle
              checked={props.escalation().enabled}
              onChange={(event) => {
                props.setEscalation({
                  ...props.escalation(),
                  enabled: event.currentTarget.checked,
                });
                props.setHasUnsavedChanges(true);
              }}
              containerClass="sm:self-start"
              label={
                <span class="text-xs font-medium text-muted">
                  {getAlertConfigToggleStatusLabel(props.escalation().enabled)}
                </span>
              }
            />
          }
          class="space-y-4"
        >
          <Show when={props.escalation().enabled}>
            <div class="space-y-3">
              <p class={formHelpText}>{getAlertConfigEscalationHelp()}</p>
              <For each={props.escalation().levels}>
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
                          onChange={(event) => {
                            const nextLevels = [...props.escalation().levels];
                            const parsed = Number.parseInt(event.currentTarget.value, 10);
                            nextLevels[index()] = {
                              ...level,
                              after: Number.isNaN(parsed) ? level.after : parsed,
                            };
                            props.setEscalation({
                              ...props.escalation(),
                              levels: nextLevels,
                            });
                            props.setHasUnsavedChanges(true);
                          }}
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
                          onChange={(event) => {
                            const nextLevels = [...props.escalation().levels];
                            nextLevels[index()] = {
                              ...level,
                              notify: event.currentTarget.value as EscalationNotifyTarget,
                            };
                            props.setEscalation({
                              ...props.escalation(),
                              levels: nextLevels,
                            });
                            props.setHasUnsavedChanges(true);
                          }}
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
                      onClick={() => {
                        const nextLevels = props.escalation().levels.filter(
                          (_level: EscalationLevel, currentIndex: number) =>
                            currentIndex !== index(),
                        );
                        props.setEscalation({
                          ...props.escalation(),
                          levels: nextLevels,
                        });
                        props.setHasUnsavedChanges(true);
                      }}
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
                onClick={() => {
                  const lastLevel = props.escalation().levels[props.escalation().levels.length - 1];
                  const nextAfter =
                    typeof lastLevel?.after === 'number' ? lastLevel.after + 30 : 15;
                  props.setEscalation({
                    ...props.escalation(),
                    levels: [
                      ...props.escalation().levels,
                      { after: nextAfter, notify: 'all' as EscalationNotifyTarget },
                    ],
                  });
                  props.setHasUnsavedChanges(true);
                }}
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

        <SettingsPanel
          title={ALERT_CONFIG_SUMMARY_TITLE}
          description={ALERT_CONFIG_SUMMARY_DESCRIPTION}
          tone="muted"
          padding="lg"
          bodyClass="space-y-1 text-sm text-blue-800 dark:text-blue-300"
          class="lg:col-span-2"
        >
          <Show when={props.quietHours().enabled}>
            <p>
              {getAlertConfigSummaryQuietHours(
                props.quietHours().start,
                props.quietHours().end,
                props.quietHours().timezone,
              )}
            </p>
          </Show>
          <Show
            when={
              props.quietHours().enabled &&
              (props.quietHours().suppress.performance ||
                props.quietHours().suppress.storage ||
                props.quietHours().suppress.offline)
            }
          >
            <p>
              {getAlertConfigSummarySuppressing(
                quietHourSuppressOptions
                  .filter((option) => props.quietHours().suppress[option.key])
                  .map((option) => option.label),
              )}
            </p>
          </Show>
          <Show when={props.cooldown().enabled}>
            <p>
              {getAlertConfigSummaryCooldown(
                props.cooldown().minutes,
                props.cooldown().maxAlerts,
              )}
            </p>
          </Show>
          <Show when={props.grouping().enabled}>
            <p>
              {getAlertConfigSummaryGrouping(
                props.grouping().window,
                props.grouping().byNode ?? false,
                props.grouping().byGuest ?? false,
              )}
            </p>
          </Show>
          <Show when={props.notifyOnResolve()}>
            <p>{getAlertConfigSummaryRecoveryEnabled()}</p>
          </Show>
          <Show when={props.escalation().enabled && props.escalation().levels.length > 0}>
            <p>{getAlertConfigSummaryEscalation(props.escalation().levels.length)}</p>
          </Show>
          <Show
            when={
              !props.quietHours().enabled &&
              !props.cooldown().enabled &&
              !props.grouping().enabled &&
              !props.escalation().enabled
            }
          >
            <p>{getAlertConfigSummaryAllDisabled()}</p>
          </Show>
        </SettingsPanel>
      </div>
    </div>
  );
}
