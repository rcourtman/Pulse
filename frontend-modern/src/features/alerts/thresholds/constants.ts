import { pmgColumn } from './helpers';

export const PMG_THRESHOLD_COLUMNS = [
  pmgColumn('queueTotalWarning', 'Queue Warn'),
  pmgColumn('queueTotalCritical', 'Queue Crit'),
  pmgColumn('deferredQueueWarn', 'Deferred Warn'),
  pmgColumn('deferredQueueCritical', 'Deferred Crit'),
  pmgColumn('holdQueueWarn', 'Hold Warn'),
  pmgColumn('holdQueueCritical', 'Hold Crit'),
  pmgColumn('oldestMessageWarnMins', 'Oldest Warn (min)'),
  pmgColumn('oldestMessageCritMins', 'Oldest Crit (min)'),
  pmgColumn('quarantineSpamWarn', 'Spam Warn'),
  pmgColumn('quarantineSpamCritical', 'Spam Crit'),
  pmgColumn('quarantineVirusWarn', 'Virus Warn'),
  pmgColumn('quarantineVirusCritical', 'Virus Crit'),
  pmgColumn('quarantineGrowthWarnPct', 'Growth Warn %'),
  pmgColumn('quarantineGrowthWarnMin', 'Growth Warn Min'),
  pmgColumn('quarantineGrowthCritPct', 'Growth Crit %'),
  pmgColumn('quarantineGrowthCritMin', 'Growth Crit Min'),
] as const;

export const PMG_NORMALIZED_TO_KEY = new Map(
  PMG_THRESHOLD_COLUMNS.map((column) => [column.normalized, column.key]),
);

export const PMG_KEY_TO_NORMALIZED = new Map(
  PMG_THRESHOLD_COLUMNS.map((column) => [column.key, column.normalized]),
);

export const DEFAULT_SNAPSHOT_WARNING = 30;
export const DEFAULT_SNAPSHOT_CRITICAL = 45;
export const DEFAULT_SNAPSHOT_WARNING_SIZE = 0;
export const DEFAULT_SNAPSHOT_CRITICAL_SIZE = 0;
export const DEFAULT_BACKUP_WARNING = 7;
export const DEFAULT_BACKUP_CRITICAL = 14;
export const DEFAULT_BACKUP_FRESH_HOURS = 24;
export const DEFAULT_BACKUP_STALE_HOURS = 72;
