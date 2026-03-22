import { createMemo } from 'solid-js';

import {
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  createDefaultResolveNotifications,
  fallbackMaxAlertsPerHour,
} from './helpers';
import type {
  CooldownConfig,
  EscalationConfig,
  EscalationLevel,
  EscalationNotifyTarget,
  GroupingConfig,
  QuietHoursConfig,
} from './types';
import { fallbackCooldownMinutes } from './types';

export interface UseAlertScheduleStateProps {
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

export const ALERT_SCHEDULE_TIMEZONES = [
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
] as const;

export const ALERT_SCHEDULE_DAYS = [
  { id: 'monday', label: 'M', fullLabel: 'Monday' },
  { id: 'tuesday', label: 'T', fullLabel: 'Tuesday' },
  { id: 'wednesday', label: 'W', fullLabel: 'Wednesday' },
  { id: 'thursday', label: 'T', fullLabel: 'Thursday' },
  { id: 'friday', label: 'F', fullLabel: 'Friday' },
  { id: 'saturday', label: 'S', fullLabel: 'Saturday' },
  { id: 'sunday', label: 'S', fullLabel: 'Sunday' },
] as const;

type AlertScheduleDayId = (typeof ALERT_SCHEDULE_DAYS)[number]['id'];
type QuietSuppressKey = keyof QuietHoursConfig['suppress'];

export function useAlertScheduleState(props: UseAlertScheduleStateProps) {
  const markUnsaved = () => {
    props.setHasUnsavedChanges(true);
  };

  const resetToDefaults = () => {
    props.setQuietHours(createDefaultQuietHours());
    props.setCooldown(createDefaultCooldown());
    props.setGrouping(createDefaultGrouping());
    props.setNotifyOnResolve(createDefaultResolveNotifications());
    props.setEscalation(createDefaultEscalation());
    markUnsaved();
  };

  const setQuietHoursEnabled = (enabled: boolean) => {
    props.setQuietHours({
      ...props.quietHours(),
      enabled,
    });
    markUnsaved();
  };

  const setQuietHoursStart = (start: string) => {
    props.setQuietHours({
      ...props.quietHours(),
      start,
    });
    markUnsaved();
  };

  const setQuietHoursEnd = (end: string) => {
    props.setQuietHours({
      ...props.quietHours(),
      end,
    });
    markUnsaved();
  };

  const setQuietHoursTimezone = (timezone: string) => {
    props.setQuietHours({
      ...props.quietHours(),
      timezone,
    });
    markUnsaved();
  };

  const toggleQuietDay = (dayId: AlertScheduleDayId) => {
    const currentQuietHours = props.quietHours();
    props.setQuietHours({
      ...currentQuietHours,
      days: {
        ...currentQuietHours.days,
        [dayId]: !currentQuietHours.days[dayId],
      },
    });
    markUnsaved();
  };

  const setQuietSuppressCategory = (key: QuietSuppressKey, enabled: boolean) => {
    const currentQuietHours = props.quietHours();
    props.setQuietHours({
      ...currentQuietHours,
      suppress: {
        ...currentQuietHours.suppress,
        [key]: enabled,
      },
    });
    markUnsaved();
  };

  const weekdaysOnly = createMemo(() => {
    const currentDays = props.quietHours().days;
    return (
      currentDays.monday &&
      currentDays.tuesday &&
      currentDays.wednesday &&
      currentDays.thursday &&
      currentDays.friday &&
      !currentDays.saturday &&
      !currentDays.sunday
    );
  });

  const weekendsOnly = createMemo(() => {
    const currentDays = props.quietHours().days;
    return (
      !currentDays.monday &&
      !currentDays.tuesday &&
      !currentDays.wednesday &&
      !currentDays.thursday &&
      !currentDays.friday &&
      currentDays.saturday &&
      currentDays.sunday
    );
  });

  const setCooldownEnabled = (enabled: boolean) => {
    const currentCooldown = props.cooldown();
    const nextCooldown: CooldownConfig = {
      ...currentCooldown,
      enabled,
    };
    if (enabled) {
      nextCooldown.minutes = fallbackCooldownMinutes(currentCooldown.minutes);
      nextCooldown.maxAlerts = fallbackMaxAlertsPerHour(currentCooldown.maxAlerts);
    }
    props.setCooldown(nextCooldown);
    markUnsaved();
  };

  const setCooldownMinutes = (rawValue: string) => {
    const parsed = Number.parseInt(rawValue, 10);
    props.setCooldown({
      ...props.cooldown(),
      minutes: Number.isNaN(parsed) ? props.cooldown().minutes : parsed,
    });
    markUnsaved();
  };

  const setCooldownMaxAlerts = (rawValue: string) => {
    const parsed = Number.parseInt(rawValue, 10);
    props.setCooldown({
      ...props.cooldown(),
      maxAlerts: Number.isNaN(parsed) ? props.cooldown().maxAlerts : parsed,
    });
    markUnsaved();
  };

  const setGroupingEnabled = (enabled: boolean) => {
    props.setGrouping({
      ...props.grouping(),
      enabled,
    });
    markUnsaved();
  };

  const setGroupingWindow = (rawValue: string) => {
    props.setGrouping({
      ...props.grouping(),
      window: Number.parseInt(rawValue, 10),
    });
    markUnsaved();
  };

  const setGroupingByNode = (enabled: boolean) => {
    props.setGrouping({
      ...props.grouping(),
      byNode: enabled,
    });
    markUnsaved();
  };

  const setGroupingByGuest = (enabled: boolean) => {
    props.setGrouping({
      ...props.grouping(),
      byGuest: enabled,
    });
    markUnsaved();
  };

  const setNotifyOnResolveEnabled = (enabled: boolean) => {
    props.setNotifyOnResolve(enabled);
    markUnsaved();
  };

  const setEscalationEnabled = (enabled: boolean) => {
    props.setEscalation({
      ...props.escalation(),
      enabled,
    });
    markUnsaved();
  };

  const setEscalationAfter = (index: number, rawValue: string) => {
    const nextLevels = [...props.escalation().levels];
    const currentLevel = nextLevels[index];
    const parsed = Number.parseInt(rawValue, 10);
    nextLevels[index] = {
      ...currentLevel,
      after: Number.isNaN(parsed) ? currentLevel.after : parsed,
    };
    props.setEscalation({
      ...props.escalation(),
      levels: nextLevels,
    });
    markUnsaved();
  };

  const setEscalationNotify = (index: number, notify: EscalationNotifyTarget) => {
    const nextLevels = [...props.escalation().levels];
    nextLevels[index] = {
      ...nextLevels[index],
      notify,
    };
    props.setEscalation({
      ...props.escalation(),
      levels: nextLevels,
    });
    markUnsaved();
  };

  const removeEscalationLevel = (index: number) => {
    const nextLevels = props.escalation().levels.filter(
      (_level: EscalationLevel, currentIndex: number) => currentIndex !== index,
    );
    props.setEscalation({
      ...props.escalation(),
      levels: nextLevels,
    });
    markUnsaved();
  };

  const addEscalationLevel = () => {
    const lastLevel = props.escalation().levels[props.escalation().levels.length - 1];
    const nextAfter = typeof lastLevel?.after === 'number' ? lastLevel.after + 30 : 15;
    props.setEscalation({
      ...props.escalation(),
      levels: [
        ...props.escalation().levels,
        { after: nextAfter, notify: 'all' as EscalationNotifyTarget },
      ],
    });
    markUnsaved();
  };

  return {
    weekdaysOnly,
    weekendsOnly,
    resetToDefaults,
    setQuietHoursEnabled,
    setQuietHoursStart,
    setQuietHoursEnd,
    setQuietHoursTimezone,
    toggleQuietDay,
    setQuietSuppressCategory,
    setCooldownEnabled,
    setCooldownMinutes,
    setCooldownMaxAlerts,
    setGroupingEnabled,
    setGroupingWindow,
    setGroupingByNode,
    setGroupingByGuest,
    setNotifyOnResolveEnabled,
    setEscalationEnabled,
    setEscalationAfter,
    setEscalationNotify,
    removeEscalationLevel,
    addEscalationLevel,
  };
}
