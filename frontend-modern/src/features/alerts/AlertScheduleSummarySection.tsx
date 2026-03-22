import { Show } from 'solid-js';

import { SettingsPanel } from '@/components/shared/SettingsPanel';
import {
  ALERT_CONFIG_SUMMARY_DESCRIPTION,
  ALERT_CONFIG_SUMMARY_TITLE,
  getAlertConfigSummaryAllDisabled,
  getAlertConfigSummaryCooldown,
  getAlertConfigSummaryEscalation,
  getAlertConfigSummaryGrouping,
  getAlertConfigSummaryQuietHours,
  getAlertConfigSummaryRecoveryEnabled,
  getAlertConfigSummarySuppressing,
} from '@/utils/alertConfigPresentation';

import type {
  CooldownConfig,
  EscalationConfig,
  GroupingConfig,
  QuietHoursConfig,
} from './types';

interface QuietSuppressOption {
  key: keyof QuietHoursConfig['suppress'];
  label: string;
  description: string;
}

interface AlertScheduleSummarySectionProps {
  quietHours: QuietHoursConfig;
  cooldown: CooldownConfig;
  grouping: GroupingConfig;
  notifyOnResolve: boolean;
  escalation: EscalationConfig;
  quietHourSuppressOptions: QuietSuppressOption[];
}

export function AlertScheduleSummarySection(props: AlertScheduleSummarySectionProps) {
  return (
    <SettingsPanel
      title={ALERT_CONFIG_SUMMARY_TITLE}
      description={ALERT_CONFIG_SUMMARY_DESCRIPTION}
      tone="muted"
      padding="lg"
      bodyClass="space-y-1 text-sm text-blue-800 dark:text-blue-300"
      class="lg:col-span-2"
    >
      <Show when={props.quietHours.enabled}>
        <p>
          {getAlertConfigSummaryQuietHours(
            props.quietHours.start,
            props.quietHours.end,
            props.quietHours.timezone,
          )}
        </p>
      </Show>
      <Show
        when={
          props.quietHours.enabled &&
          (props.quietHours.suppress.performance ||
            props.quietHours.suppress.storage ||
            props.quietHours.suppress.offline)
        }
      >
        <p>
          {getAlertConfigSummarySuppressing(
            props.quietHourSuppressOptions
              .filter((option) => props.quietHours.suppress[option.key])
              .map((option) => option.label),
          )}
        </p>
      </Show>
      <Show when={props.cooldown.enabled}>
        <p>{getAlertConfigSummaryCooldown(props.cooldown.minutes, props.cooldown.maxAlerts)}</p>
      </Show>
      <Show when={props.grouping.enabled}>
        <p>
          {getAlertConfigSummaryGrouping(
            props.grouping.window,
            props.grouping.byNode ?? false,
            props.grouping.byGuest ?? false,
          )}
        </p>
      </Show>
      <Show when={props.notifyOnResolve}>
        <p>{getAlertConfigSummaryRecoveryEnabled()}</p>
      </Show>
      <Show when={props.escalation.enabled && props.escalation.levels.length > 0}>
        <p>{getAlertConfigSummaryEscalation(props.escalation.levels.length)}</p>
      </Show>
      <Show
        when={
          !props.quietHours.enabled &&
          !props.cooldown.enabled &&
          !props.grouping.enabled &&
          !props.escalation.enabled
        }
      >
        <p>{getAlertConfigSummaryAllDisabled()}</p>
      </Show>
    </SettingsPanel>
  );
}
