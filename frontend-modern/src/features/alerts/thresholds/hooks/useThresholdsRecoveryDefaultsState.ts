import { createMemo } from 'solid-js';

import type { BackupAlertConfig, SnapshotAlertConfig } from '@/types/alerts';
import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import {
  DEFAULT_BACKUP_CRITICAL,
  DEFAULT_BACKUP_FRESH_HOURS,
  DEFAULT_BACKUP_STALE_HOURS,
  DEFAULT_BACKUP_WARNING,
  DEFAULT_SNAPSHOT_CRITICAL,
  DEFAULT_SNAPSHOT_CRITICAL_SIZE,
  DEFAULT_SNAPSHOT_WARNING,
  DEFAULT_SNAPSHOT_WARNING_SIZE,
} from '../constants';

export function useThresholdsRecoveryDefaultsState(props: ThresholdsTableProps) {
  const snapshotFactoryConfig = () =>
    props.snapshotFactoryDefaults ?? {
      enabled: false,
      warningDays: DEFAULT_SNAPSHOT_WARNING,
      criticalDays: DEFAULT_SNAPSHOT_CRITICAL,
      warningSizeGiB: DEFAULT_SNAPSHOT_WARNING_SIZE,
      criticalSizeGiB: DEFAULT_SNAPSHOT_CRITICAL_SIZE,
    };

  const sanitizeSnapshotConfig = (config: SnapshotAlertConfig): SnapshotAlertConfig => {
    let warning = Math.max(0, Math.round(config.warningDays ?? 0));
    let critical = Math.max(0, Math.round(config.criticalDays ?? 0));

    if (critical > 0 && warning > critical) {
      warning = critical;
    }
    if (critical === 0 && warning > 0) {
      critical = warning;
    }

    const rawWarningSize = Number.isFinite(config.warningSizeGiB)
      ? Number(config.warningSizeGiB)
      : DEFAULT_SNAPSHOT_WARNING_SIZE;
    const rawCriticalSize = Number.isFinite(config.criticalSizeGiB)
      ? Number(config.criticalSizeGiB)
      : DEFAULT_SNAPSHOT_CRITICAL_SIZE;

    const roundSize = (value: number) => Math.round(Math.max(0, value) * 10) / 10;

    let warningSize = roundSize(rawWarningSize);
    let criticalSize = roundSize(rawCriticalSize);

    if (criticalSize > 0 && warningSize > criticalSize) {
      warningSize = criticalSize;
    }
    if (criticalSize === 0 && warningSize > 0) {
      criticalSize = warningSize;
    }

    return {
      enabled: !!config.enabled,
      warningDays: warning,
      criticalDays: critical,
      warningSizeGiB: warningSize,
      criticalSizeGiB: criticalSize,
    };
  };

  const backupFactoryConfig = () =>
    props.backupFactoryDefaults ?? {
      enabled: false,
      warningDays: DEFAULT_BACKUP_WARNING,
      criticalDays: DEFAULT_BACKUP_CRITICAL,
      freshHours: DEFAULT_BACKUP_FRESH_HOURS,
      staleHours: DEFAULT_BACKUP_STALE_HOURS,
      alertOrphaned: true,
      ignoreVMIDs: [],
    };

  const sanitizeBackupConfig = (config: BackupAlertConfig): BackupAlertConfig => {
    let warning = Math.max(0, Math.round(config.warningDays ?? 0));
    let critical = Math.max(0, Math.round(config.criticalDays ?? 0));
    let fresh = Math.max(0, Math.round(config.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS));
    let stale = Math.max(0, Math.round(config.staleHours ?? DEFAULT_BACKUP_STALE_HOURS));
    const alertOrphaned = config.alertOrphaned ?? true;
    const ignoreVMIDs = Array.from(
      new Set(
        (config.ignoreVMIDs ?? []).map((value) => value.trim()).filter((value) => value.length > 0),
      ),
    );

    if (critical > 0 && warning > critical) {
      warning = critical;
    }
    if (critical === 0 && warning > 0) {
      critical = warning;
    }
    if (stale < fresh) {
      stale = fresh;
    }

    return {
      enabled: !!config.enabled,
      warningDays: warning,
      criticalDays: critical,
      freshHours: fresh,
      staleHours: stale,
      alertOrphaned,
      ignoreVMIDs,
    };
  };

  const snapshotDefaultsRecord = createMemo(() => {
    const current = props.snapshotDefaults();
    return {
      'warning days': current.warningDays ?? 0,
      'critical days': current.criticalDays ?? 0,
      'warning size (gib)': current.warningSizeGiB ?? 0,
      'critical size (gib)': current.criticalSizeGiB ?? 0,
    };
  });

  const snapshotFactoryDefaultsRecord = createMemo(() => {
    const factory = snapshotFactoryConfig();
    return {
      'warning days': factory.warningDays ?? DEFAULT_SNAPSHOT_WARNING,
      'critical days': factory.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL,
      'warning size (gib)': factory.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE,
      'critical size (gib)': factory.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE,
    };
  });

  const backupDefaultsRecord = createMemo(() => {
    const current = props.backupDefaults();
    return {
      'fresh hours': current.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
      'stale hours': current.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
      'warning days': current.warningDays ?? 0,
      'critical days': current.criticalDays ?? 0,
    };
  });

  const backupFactoryDefaultsRecord = createMemo(() => {
    const factory = backupFactoryConfig();
    return {
      'fresh hours': factory.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS,
      'stale hours': factory.staleHours ?? DEFAULT_BACKUP_STALE_HOURS,
      'warning days': factory.warningDays ?? DEFAULT_BACKUP_WARNING,
      'critical days': factory.criticalDays ?? DEFAULT_BACKUP_CRITICAL,
    };
  });

  const snapshotOverridesCount = createMemo(() => {
    const current = props.snapshotDefaults();
    const factory = snapshotFactoryConfig();
    const differs =
      current.enabled !== factory.enabled ||
      (current.warningDays ?? DEFAULT_SNAPSHOT_WARNING) !==
        (factory.warningDays ?? DEFAULT_SNAPSHOT_WARNING) ||
      (current.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL) !==
        (factory.criticalDays ?? DEFAULT_SNAPSHOT_CRITICAL) ||
      (current.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE) !==
        (factory.warningSizeGiB ?? DEFAULT_SNAPSHOT_WARNING_SIZE) ||
      (current.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE) !==
        (factory.criticalSizeGiB ?? DEFAULT_SNAPSHOT_CRITICAL_SIZE);
    return differs ? 1 : 0;
  });

  const backupOverridesCount = createMemo(() => {
    const current = props.backupDefaults();
    const factory = backupFactoryConfig();
    const currentIgnore = current.ignoreVMIDs ?? [];
    const factoryIgnore = factory.ignoreVMIDs ?? [];
    const ignoreDiff =
      currentIgnore.length !== factoryIgnore.length ||
      currentIgnore.some((value, index) => value !== factoryIgnore[index]);

    return current.enabled !== factory.enabled ||
      (current.warningDays ?? DEFAULT_BACKUP_WARNING) !==
        (factory.warningDays ?? DEFAULT_BACKUP_WARNING) ||
      (current.criticalDays ?? DEFAULT_BACKUP_CRITICAL) !==
        (factory.criticalDays ?? DEFAULT_BACKUP_CRITICAL) ||
      (current.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS) !==
        (factory.freshHours ?? DEFAULT_BACKUP_FRESH_HOURS) ||
      (current.staleHours ?? DEFAULT_BACKUP_STALE_HOURS) !==
        (factory.staleHours ?? DEFAULT_BACKUP_STALE_HOURS) ||
      (current.alertOrphaned ?? true) !== (factory.alertOrphaned ?? true) ||
      ignoreDiff
      ? 1
      : 0;
  });

  return {
    backupDefaultsRecord,
    backupFactoryConfig,
    backupFactoryDefaultsRecord,
    backupOverridesCount,
    sanitizeBackupConfig,
    sanitizeSnapshotConfig,
    snapshotDefaultsRecord,
    snapshotFactoryConfig,
    snapshotFactoryDefaultsRecord,
    snapshotOverridesCount,
  };
}
