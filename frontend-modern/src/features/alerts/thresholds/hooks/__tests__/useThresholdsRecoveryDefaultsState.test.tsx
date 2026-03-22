import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';

import type { ThresholdsTableProps } from '@/features/alerts/thresholds/types';
import { useThresholdsRecoveryDefaultsState } from '../useThresholdsRecoveryDefaultsState';

const buildProps = (): ThresholdsTableProps =>
  ({
    backupDefaults: () => ({
      alertOrphaned: false,
      criticalDays: 8,
      enabled: true,
      freshHours: 24,
      ignoreVMIDs: ['100', '200'],
      staleHours: 72,
      warningDays: 4,
    }),
    backupFactoryDefaults: {
      alertOrphaned: true,
      criticalDays: 7,
      enabled: false,
      freshHours: 12,
      ignoreVMIDs: ['100'],
      staleHours: 48,
      warningDays: 3,
    },
    snapshotDefaults: () => ({
      criticalDays: 10,
      criticalSizeGiB: 18,
      enabled: true,
      warningDays: 6,
      warningSizeGiB: 12,
    }),
    snapshotFactoryDefaults: {
      criticalDays: 7,
      criticalSizeGiB: 16,
      enabled: false,
      warningDays: 3,
      warningSizeGiB: 8,
    },
  }) as ThresholdsTableProps;

afterEach(() => {
  cleanup();
});

describe('useThresholdsRecoveryDefaultsState', () => {
  it('owns backup and snapshot default-policy sanitization', () => {
    let captured: ReturnType<typeof useThresholdsRecoveryDefaultsState> | undefined;

    const Harness = () => {
      captured = useThresholdsRecoveryDefaultsState(buildProps());
      return null;
    };

    render(() => <Harness />);

    expect(captured).toBeDefined();
    expect(
      captured!.sanitizeSnapshotConfig({
        criticalDays: 4,
        criticalSizeGiB: 1.08,
        enabled: true,
        warningDays: 9,
        warningSizeGiB: 3.16,
      }),
    ).toEqual({
      criticalDays: 4,
      criticalSizeGiB: 1.1,
      enabled: true,
      warningDays: 4,
      warningSizeGiB: 1.1,
    });

    expect(
      captured!.sanitizeBackupConfig({
        alertOrphaned: false,
        criticalDays: 2,
        enabled: true,
        freshHours: 9,
        ignoreVMIDs: ['101', ' ', '101', '202'],
        staleHours: 4,
        warningDays: 7,
      }),
    ).toEqual({
      alertOrphaned: false,
      criticalDays: 2,
      enabled: true,
      freshHours: 9,
      ignoreVMIDs: ['101', '202'],
      staleHours: 9,
      warningDays: 2,
    });
  });

  it('tracks factory drift for backup and snapshot defaults', () => {
    let captured: ReturnType<typeof useThresholdsRecoveryDefaultsState> | undefined;

    const Harness = () => {
      captured = useThresholdsRecoveryDefaultsState(buildProps());
      return null;
    };

    render(() => <Harness />);

    expect(captured).toBeDefined();
    expect(captured!.snapshotDefaultsRecord()).toEqual({
      'critical days': 10,
      'critical size (gib)': 18,
      'warning days': 6,
      'warning size (gib)': 12,
    });
    expect(captured!.backupFactoryDefaultsRecord()).toEqual({
      'critical days': 7,
      'fresh hours': 12,
      'stale hours': 48,
      'warning days': 3,
    });
    expect(captured!.snapshotOverridesCount()).toBe(1);
    expect(captured!.backupOverridesCount()).toBe(1);
  });
});
