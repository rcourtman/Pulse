import { describe, expect, it } from 'vitest';
import {
  ALERT_THRESHOLDS_SECTION_DISABLED_LABEL,
  ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE,
  getAlertThresholdsDefaultsSummary,
  getAlertThresholdsSectionDisabledLabel,
  getDockerContainerRuleSummaryItems,
  getAlertThresholdsSectionUnsavedChangesTitle,
} from '@/utils/alertThresholdsSectionPresentation';

describe('alertThresholdsSectionPresentation', () => {
  it('returns canonical thresholds section status vocabulary', () => {
    expect(ALERT_THRESHOLDS_SECTION_DISABLED_LABEL).toBe('Disabled');
    expect(ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE).toBe('Unsaved changes');
    expect(getAlertThresholdsSectionDisabledLabel()).toBe('Disabled');
    expect(getAlertThresholdsSectionUnsavedChangesTitle()).toBe('Unsaved changes');
  });

  it('names the enabled default limits a threshold group alerts on', () => {
    expect(
      getAlertThresholdsDefaultsSummary(
        ['CPU %', 'Memory %', 'Disk %', 'Backup', 'Snapshot', 'Disk R MB/s', 'Net In MB/s'],
        { cpu: 80, memory: 85, disk: 90, diskRead: 0, networkIn: -1 },
      ),
    ).toBe('Defaults: CPU 80% · Memory 85% · Disk 90%');
    expect(
      getAlertThresholdsDefaultsSummary(
        ['CPU %', 'Memory %', 'Disk %', 'Disk R MB/s', 'Net In MB/s'],
        {
          cpu: 80,
          memory: 85,
          disk: 90,
          diskRead: 50,
          networkIn: 100,
        },
      ),
    ).toBe('Defaults: CPU 80% · Memory 85% · Disk 90% · Disk read 50 MB/s · +1 more');
  });

  it('reads day, hour, size and percent units from the column label', () => {
    expect(
      getAlertThresholdsDefaultsSummary(
        ['Warning Days', 'Critical Days', 'Warning Size (GiB)', 'Critical Size (GiB)'],
        { 'warning days': 30, 'critical days': 1, warningSizeGiB: 0, criticalSizeGiB: 0 },
      ),
    ).toBe('Defaults: Warning 30 days · Critical 1 day');
    expect(
      getAlertThresholdsDefaultsSummary(['Fresh Hours', 'Stale Hours'], {
        'fresh hours': 24,
        'stale hours': 72,
      }),
    ).toBe('Defaults: Fresh 24 hours · Stale 72 hours');
    expect(
      getAlertThresholdsDefaultsSummary(['Restart Count', 'Restart Window (s)', 'Memory Warn %'], {
        restartCount: 3,
        restartWindow: 300,
        memoryWarnPct: 90,
      }),
    ).toBe('Defaults: Restart Count 3 · Restart Window 300s · Memory Warn 90%');
  });

  it('says when every metric alert is off and stays silent without numeric defaults', () => {
    expect(getAlertThresholdsDefaultsSummary(['CPU %', 'Memory %'], { cpu: 0, memory: -1 })).toBe(
      'Defaults: all metric alerts off',
    );
    expect(getAlertThresholdsDefaultsSummary([], { cpu: 80 })).toBeUndefined();
    expect(getAlertThresholdsDefaultsSummary(['Backup'], { cpu: 80 })).toBeUndefined();
    expect(getAlertThresholdsDefaultsSummary(['CPU %'], undefined)).toBeUndefined();
  });

  it('states Docker restart-loop and memory-limit rules with their effective fallbacks', () => {
    expect(
      getDockerContainerRuleSummaryItems({
        restartCount: 3,
        restartWindow: 300,
        memoryWarnPct: 90,
        memoryCriticalPct: 95,
      }),
    ).toEqual(['Restart loop 3 in 5 min', 'Memory limit 90% warn, 95% critical']);
    // Config normalization replaces <= 0 with the fallbacks, so these rules
    // never read as Off.
    expect(
      getDockerContainerRuleSummaryItems({
        restartCount: -1,
        restartWindow: 0,
        memoryWarnPct: 0,
        memoryCriticalPct: -1,
      }),
    ).toEqual(['Restart loop 3 in 5 min', 'Memory limit 90% warn, 95% critical']);
    expect(getDockerContainerRuleSummaryItems({ restartCount: 5, restartWindow: 90 })[0]).toBe(
      'Restart loop 5 in 90s',
    );
    expect(
      getAlertThresholdsDefaultsSummary(
        ['CPU %', 'Memory %', 'Disk %'],
        { cpu: 0, memory: 0, disk: 0 },
        { ruleItems: getDockerContainerRuleSummaryItems({}) },
      ),
    ).toBe('Defaults: Restart loop 3 in 5 min · Memory limit 90% warn, 95% critical');
  });

  it('summarizes backup alerts by their day thresholds only', () => {
    expect(
      getAlertThresholdsDefaultsSummary(['Warning Days', 'Critical Days'], {
        'fresh hours': 24,
        'stale hours': 72,
        'warning days': 0,
        'critical days': 0,
      }),
    ).toBe('Defaults: all metric alerts off');
  });
});
