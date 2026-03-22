import {
  ALERT_CONFIG_SCHEDULING_DESCRIPTION,
  ALERT_CONFIG_SCHEDULING_TITLE,
  getAlertConfigQuietHourSuppressOptions,
  getAlertConfigResetDefaultsLabel,
  getAlertConfigResetDefaultsTitle,
} from '@/utils/alertConfigPresentation';
import { AlertCooldownSection } from '../AlertCooldownSection';
import { AlertEscalationSection } from '../AlertEscalationSection';
import { AlertGroupingSection } from '../AlertGroupingSection';
import { AlertQuietHoursSection } from '../AlertQuietHoursSection';
import { AlertRecoverySection } from '../AlertRecoverySection';
import { AlertScheduleSummarySection } from '../AlertScheduleSummarySection';
import { useAlertScheduleState } from '../useAlertScheduleState';
import type { CooldownConfig, EscalationConfig, GroupingConfig, QuietHoursConfig } from '../types';

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

export function ScheduleTab(props: ScheduleTabProps) {
  const quietHourSuppressOptions = getAlertConfigQuietHourSuppressOptions();
  const scheduleState = useAlertScheduleState(props);

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
          onClick={scheduleState.resetToDefaults}
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
        <AlertQuietHoursSection
          quietHours={props.quietHours()}
          quietHourSuppressOptions={quietHourSuppressOptions}
          weekdaysOnly={scheduleState.weekdaysOnly()}
          weekendsOnly={scheduleState.weekendsOnly()}
          setQuietHoursEnabled={scheduleState.setQuietHoursEnabled}
          setQuietHoursStart={scheduleState.setQuietHoursStart}
          setQuietHoursEnd={scheduleState.setQuietHoursEnd}
          setQuietHoursTimezone={scheduleState.setQuietHoursTimezone}
          toggleQuietDay={scheduleState.toggleQuietDay}
          setQuietSuppressCategory={scheduleState.setQuietSuppressCategory}
        />

        <AlertCooldownSection
          cooldown={props.cooldown()}
          setCooldownEnabled={scheduleState.setCooldownEnabled}
          setCooldownMinutes={scheduleState.setCooldownMinutes}
          setCooldownMaxAlerts={scheduleState.setCooldownMaxAlerts}
        />

        <AlertGroupingSection
          grouping={props.grouping()}
          setGroupingEnabled={scheduleState.setGroupingEnabled}
          setGroupingWindow={scheduleState.setGroupingWindow}
          setGroupingByNode={scheduleState.setGroupingByNode}
          setGroupingByGuest={scheduleState.setGroupingByGuest}
        />

        <AlertRecoverySection
          notifyOnResolve={props.notifyOnResolve()}
          setNotifyOnResolveEnabled={scheduleState.setNotifyOnResolveEnabled}
        />

        <AlertEscalationSection
          escalation={props.escalation()}
          setEscalationEnabled={scheduleState.setEscalationEnabled}
          setEscalationAfter={scheduleState.setEscalationAfter}
          setEscalationNotify={scheduleState.setEscalationNotify}
          removeEscalationLevel={scheduleState.removeEscalationLevel}
          addEscalationLevel={scheduleState.addEscalationLevel}
        />

        <AlertScheduleSummarySection
          quietHours={props.quietHours()}
          cooldown={props.cooldown()}
          grouping={props.grouping()}
          notifyOnResolve={props.notifyOnResolve()}
          escalation={props.escalation()}
          quietHourSuppressOptions={quietHourSuppressOptions}
        />
      </div>
    </div>
  );
}
