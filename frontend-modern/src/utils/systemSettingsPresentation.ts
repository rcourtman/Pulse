import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import {
  getActiveLocale,
  normalizeLocale,
  t,
  type I18nMessageKey,
  type SupportedLocale,
} from '@/i18n';

export const PVE_POLLING_MIN_SECONDS = 10;
export const PVE_POLLING_MAX_SECONDS = 3600;

export const PVE_POLLING_PRESETS = [
  { label: 'Realtime (10s)', value: 10 },
  { label: 'Balanced (30s)', value: 30 },
  { label: 'Low (60s)', value: 60 },
  { label: 'Very low (5m)', value: 300 },
] as const;

export const BACKUP_INTERVAL_OPTIONS = [
  { value: 0, label: 'Default (~90s)' },
  { value: 300, label: '5 minutes' },
  { value: 900, label: '15 minutes' },
  { value: 1800, label: '30 minutes' },
  { value: 3600, label: '1 hour' },
  { value: 21600, label: '6 hours' },
  { value: 86400, label: '24 hours' },
] as const;

export const BACKUP_INTERVAL_MAX_MINUTES = 7 * 24 * 60;

export const COMMON_DISCOVERY_SUBNETS = [
  '192.168.1.0/24',
  '192.168.0.0/24',
  '10.0.0.0/24',
  '172.16.0.0/24',
  '192.168.10.0/24',
] as const;

const DOCKER_PODMAN_SOURCE_LABEL = getSourcePlatformLabel('docker');

export const DOCKER_UPDATE_ACTIONS_ENV_VAR = 'PULSE_DISABLE_DOCKER_UPDATE_ACTIONS';
export const DOCKER_UPDATE_ACTIONS_SECTION_TITLE = `${DOCKER_PODMAN_SOURCE_LABEL} updates`;
export const DOCKER_UPDATE_ACTIONS_SECTION_DESCRIPTION = `Control how ${DOCKER_PODMAN_SOURCE_LABEL} update actions appear across Pulse.`;
export const DOCKER_UPDATE_ACTIONS_TOGGLE_LABEL = 'Hide update buttons';
export const DOCKER_UPDATE_ACTIONS_TOGGLE_DESCRIPTION = `When enabled, ${DOCKER_PODMAN_SOURCE_LABEL} "Update" actions are hidden across Pulse. Update detection still runs, so available updates remain visible.`;

const PVE_POLLING_PRESET_LABEL_KEYS = {
  10: 'settings.general.monitoringCadence.preset.realtime',
  30: 'settings.general.monitoringCadence.preset.balanced',
  60: 'settings.general.monitoringCadence.preset.low',
  300: 'settings.general.monitoringCadence.preset.veryLow',
} as const satisfies Record<(typeof PVE_POLLING_PRESETS)[number]['value'], I18nMessageKey>;

const PVE_POLLING_PRESET_DURATIONS = {
  10: '10s',
  30: '30s',
  60: '60s',
  300: '5m',
} as const satisfies Record<(typeof PVE_POLLING_PRESETS)[number]['value'], string>;

function resolvePresentationLocale(locale?: SupportedLocale): SupportedLocale {
  return locale ?? getActiveLocale();
}

export function getPvePollingPresetOptions(locale?: SupportedLocale) {
  const presentationLocale = resolvePresentationLocale(locale);
  return PVE_POLLING_PRESETS.map((option) => ({
    value: option.value,
    label: t(
      PVE_POLLING_PRESET_LABEL_KEYS[option.value],
      { duration: PVE_POLLING_PRESET_DURATIONS[option.value] },
      presentationLocale,
    ),
  }));
}

export function getPvePollingCustomOption(locale?: SupportedLocale) {
  return {
    value: 'custom' as const,
    label: t('settings.general.monitoringCadence.preset.custom', {}, locale),
  };
}

export function formatPvePollingDuration(seconds: number, locale?: SupportedLocale): string {
  const presentationLocale = normalizeLocale(resolvePresentationLocale(locale));
  if (seconds < 60) {
    return t('settings.general.monitoringCadence.duration.underMinute', {}, presentationLocale);
  }

  const minutes = seconds / 60;
  const count = new Intl.NumberFormat(presentationLocale, {
    maximumFractionDigits: minutes % 1 === 0 ? 0 : 1,
  }).format(minutes);

  return t(
    minutes === 1
      ? 'settings.general.monitoringCadence.duration.minute'
      : 'settings.general.monitoringCadence.duration.minutes',
    { count },
    presentationLocale,
  );
}

export function getPvePollingCadenceSummary(seconds: number, locale?: SupportedLocale): string {
  const presentationLocale = resolvePresentationLocale(locale);
  return t(
    'settings.general.monitoringCadence.current',
    { seconds, duration: formatPvePollingDuration(seconds, presentationLocale) },
    presentationLocale,
  );
}

export function getDockerUpdateActionsPresentation(
  locale?: SupportedLocale,
  sourceLabel = DOCKER_PODMAN_SOURCE_LABEL,
) {
  return {
    sectionTitle: t('settings.general.docker.section.title', { sourceLabel }, locale),
    sectionDescription: t('settings.general.docker.section.description', { sourceLabel }, locale),
    toggleLabel: t('settings.general.docker.toggle.title', {}, locale),
    toggleDescription: t('settings.general.docker.toggle.description', { sourceLabel }, locale),
    environmentHint: t('settings.general.docker.envHint', {}, locale),
  } as const;
}

export function getBackupIntervalSelectValue(
  backupPollingUseCustom: boolean,
  backupPollingInterval: number,
): string {
  if (backupPollingUseCustom) {
    return 'custom';
  }
  return BACKUP_INTERVAL_OPTIONS.some((option) => option.value === backupPollingInterval)
    ? String(backupPollingInterval)
    : 'custom';
}

export function getBackupIntervalSummary(
  backupPollingEnabled: boolean,
  backupPollingInterval: number,
): string {
  if (!backupPollingEnabled) {
    return 'Backup polling is disabled.';
  }

  if (backupPollingInterval <= 0) {
    return 'Pulse checks backups and snapshots at the default cadence (~every 90 seconds).';
  }
  if (backupPollingInterval % 86400 === 0) {
    const days = backupPollingInterval / 86400;
    return `Pulse checks backups every ${days === 1 ? 'day' : `${days} days`}.`;
  }
  if (backupPollingInterval % 3600 === 0) {
    const hours = backupPollingInterval / 3600;
    return `Pulse checks backups every ${hours === 1 ? 'hour' : `${hours} hours`}.`;
  }

  const minutes = Math.max(1, Math.round(backupPollingInterval / 60));
  return `Pulse checks backups every ${minutes === 1 ? 'minute' : `${minutes} minutes`}.`;
}

export function getSystemSettingsSaveErrorMessage(message?: string): string {
  return message || 'Unable to save settings.';
}

export function getHideLocalLoginUpdateErrorMessage(message?: string): string {
  return message || 'Unable to update local login visibility.';
}

export function getDockerUpdateActionsUpdateErrorMessage(message?: string): string {
  return message || `Unable to update ${DOCKER_PODMAN_SOURCE_LABEL} update actions.`;
}

export function getTelemetryUpdateErrorMessage(message?: string): string {
  return message || 'Unable to update anonymous telemetry.';
}

export function getTemperatureMonitoringUpdateErrorMessage(message?: string): string {
  return message || 'Unable to update temperature monitoring.';
}

export function getCheckForUpdatesErrorMessage(): string {
  return 'Unable to check for updates.';
}

export function getStartUpdateErrorMessage(): string {
  return 'Unable to start the update. Please try again.';
}
