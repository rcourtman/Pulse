import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPhysicalDiskPresentationDataMap,
  buildPhysicalDiskGroupFilterOptions,
  buildPhysicalDiskRoleFilterOptions,
  comparePhysicalDiskPresentation,
  extractPhysicalDiskPresentationData,
  filterAndSortPhysicalDisks,
  PHYSICAL_DISK_EMPTY_CARD_CLASS,
  PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL,
  PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL,
  PHYSICAL_DISK_HEADER_DEVICE_CLASS,
  PHYSICAL_DISK_HEADER_DISK_CLASS,
  PHYSICAL_DISK_HEADER_LIFE_CLASS,
  PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
  PHYSICAL_DISK_NAME_TEXT_CLASS,
  PHYSICAL_DISK_SOURCE_BADGE_CLASS,
  PHYSICAL_DISK_TABLE_CLASS,
  PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS,
  getPhysicalDiskEmptyStatePresentation,
  getPhysicalDiskCellPaddingClass,
  getPhysicalDiskColumnWidthStyle,
  getPhysicalDiskCollectionMessages,
  getPhysicalDiskFieldStatusMessage,
  getPhysicalDiskHealthCompactLabel,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskHealthSummary,
  getPhysicalDiskHostLabel,
  getPhysicalDiskLifeLabel,
  getPhysicalDiskLifeTextClass,
  getPhysicalDiskNormalizedHealth,
  getPhysicalDiskParentLabel,
  getPhysicalDiskPlatformLabel,
  getPhysicalDiskRoleLabel,
  getPhysicalDiskRoleFilterValue,
  getPhysicalDiskSourceKey,
  getPhysicalDiskSourceBadgePresentation,
  getPhysicalDiskTableLayoutModeForContainer,
  getPhysicalDiskTemperaturePresentation,
  hasUnraidPhysicalDiskFaultSignal,
  hasPhysicalDiskSmartWarning,
  isPhysicalDiskWearoutReported,
  isPhysicalDiskColumnVisible,
  isPhysicalDiskTemperatureCurrent,
  isUnraidPhysicalDisk,
  matchesPhysicalDiskSearch,
  normalizePhysicalDiskFacetFilter,
  type PhysicalDiskPresentationData,
} from '@/features/storageBackups/diskPresentation';

function makeDiskData(
  overrides: Partial<PhysicalDiskPresentationData> = {},
): PhysicalDiskPresentationData {
  return {
    node: '',
    instance: '',
    devPath: '',
    model: '',
    serial: '',
    wwn: '',
    size: 0,
    health: 'UNKNOWN',
    riskReasons: [],
    wearout: -1,
    type: '',
    temperature: 0,
    rpm: 0,
    used: '',
    ...overrides,
  };
}

describe('diskPresentation', () => {
  it('reads a disk temperature as current only when its collection state says so', () => {
    // Sources that predate collection state, and empty states, stay current.
    expect(isPhysicalDiskTemperatureCurrent(undefined)).toBe(true);
    expect(isPhysicalDiskTemperatureCurrent({})).toBe(true);
    expect(isPhysicalDiskTemperatureCurrent({ temperature: { state: '' as never } })).toBe(true);
    expect(
      isPhysicalDiskTemperatureCurrent({ temperature: { state: 'available', source: 'smartctl' } }),
    ).toBe(true);
    for (const state of ['unavailable', 'unsupported', 'missing'] as const) {
      expect(isPhysicalDiskTemperatureCurrent({ temperature: { state } })).toBe(false);
    }

    expect(
      getPhysicalDiskTemperaturePresentation(
        makeDiskData({
          temperature: 72,
          collection: { temperature: { state: 'available', source: 'host_agent' } },
        }),
      ),
    ).toEqual({ label: '72°C', current: true, title: undefined });
    expect(
      getPhysicalDiskTemperaturePresentation(
        makeDiskData({
          temperature: 72,
          collection: {
            temperature: {
              state: 'unavailable',
              source: 'host_agent',
              reason: 'host agent stopped reporting',
            },
          },
        }),
      ),
    ).toEqual({
      label: '72°C',
      current: false,
      title: 'Last known reading, not current: host agent stopped reporting',
    });
    expect(
      getPhysicalDiskTemperaturePresentation(
        makeDiskData({
          temperature: 41,
          collection: { temperature: { state: 'missing', source: 'smartctl', reason: '  ' } },
        }),
      ),
    ).toEqual({ label: '41°C', current: false, title: 'Last known reading, not current' });
    // No reading at all is not a last-known reading either.
    for (const temperature of [0, -1, NaN]) {
      expect(
        getPhysicalDiskTemperaturePresentation(
          makeDiskData({
            temperature,
            collection: { temperature: { state: 'unavailable', reason: 'disk is in standby' } },
          }),
        ),
      ).toBeNull();
    }
  });

  it('distinguishes unsupported, unavailable, and unexpectedly missing disk evidence', () => {
    expect(
      getPhysicalDiskFieldStatusMessage('Disk I/O', {
        state: 'unsupported',
        source: 'controller',
        reason: 'per-member counters unavailable',
      }),
    ).toBe('Disk I/O is unsupported: per-member counters unavailable');
    expect(
      getPhysicalDiskFieldStatusMessage('Temperature', {
        state: 'unavailable',
        source: 'smartctl',
        reason: 'collection deadline exceeded',
      }),
    ).toBe('Temperature is temporarily unavailable: collection deadline exceeded');
    expect(
      getPhysicalDiskFieldStatusMessage('Serial number', {
        state: 'missing',
        source: 'smartctl',
        reason: 'serial absent from successful response',
      }),
    ).toBe('Serial number is unexpectedly missing: serial absent from successful response');
    expect(
      getPhysicalDiskCollectionMessages(
        makeDiskData({
          collection: {
            serial: { state: 'available', source: 'smartctl' },
            temperature: { state: 'unavailable', source: 'smartctl' },
            io: { state: 'unsupported', source: 'controller' },
            controller: { state: 'missing', source: 'linux-sysfs' },
            pool: { state: 'available', source: 'zpool-status' },
          },
        }),
      ),
    ).toEqual([
      'Temperature is temporarily unavailable.',
      'Disk I/O is unsupported.',
      'Controller association is unexpectedly missing.',
    ]);
  });

  it('returns critical presentation for failed disks', () => {
    expect(PHYSICAL_DISK_EMPTY_CARD_CLASS).toBe('text-center');
    expect(PHYSICAL_DISK_TABLE_CLASS).toBe('platform-table w-full table-fixed text-xs');
    expect(PHYSICAL_DISK_TABLE_ROW_HOVER_CLASS).toContain('hover:bg-surface-hover');
    expect(PHYSICAL_DISK_HEADER_DISK_CLASS).toContain('uppercase');
    expect(PHYSICAL_DISK_HEADER_DEVICE_CLASS).toContain('uppercase');
    expect(PHYSICAL_DISK_HEADER_LIFE_CLASS).toContain('uppercase');
    expect(PHYSICAL_DISK_NAME_TEXT_CLASS).toContain('font-semibold');
    expect(PHYSICAL_DISK_SOURCE_BADGE_CLASS).toContain('justify-center');

    expect(
      getPhysicalDiskHealthStatus({
        ...makeDiskData({
          health: 'FAILED',
          wearout: 20,
          type: 'ssd',
        }),
      }),
    ).toEqual({
      label: 'Replace Now',
      summary: 'Disk health has degraded to a critical state.',
      tone: 'text-red-700 dark:text-red-300',
    });
  });

  it('selects operator-priority disk layouts from the rendered table width', () => {
    expect(getPhysicalDiskTableLayoutModeForContainer(0)).toBe('compact');
    expect(getPhysicalDiskTableLayoutModeForContainer(359)).toBe('narrow');
    expect(getPhysicalDiskTableLayoutModeForContainer(360)).toBe('compact');
    expect(getPhysicalDiskTableLayoutModeForContainer(519)).toBe('compact');
    expect(getPhysicalDiskTableLayoutModeForContainer(520)).toBe('basic');
    expect(getPhysicalDiskTableLayoutModeForContainer(649)).toBe('basic');
    expect(getPhysicalDiskTableLayoutModeForContainer(650)).toBe('operational');
    expect(getPhysicalDiskTableLayoutModeForContainer(899)).toBe('operational');
    expect(getPhysicalDiskTableLayoutModeForContainer(900)).toBe('expanded');
    expect(getPhysicalDiskTableLayoutModeForContainer(1_119)).toBe('expanded');
    expect(getPhysicalDiskTableLayoutModeForContainer(1_120)).toBe('full');
    expect(isPhysicalDiskColumnVisible('operational', 'parent')).toBe(true);
    expect(isPhysicalDiskColumnVisible('operational', 'device')).toBe(false);
    expect(isPhysicalDiskColumnVisible('expanded', 'role')).toBe(true);
    expect(isPhysicalDiskColumnVisible('expanded', 'device')).toBe(false);
    expect(
      ['disk', 'host', 'health', 'life', 'temp', 'size'].every((column) =>
        isPhysicalDiskColumnVisible(
          'compact',
          column as 'disk' | 'host' | 'health' | 'life' | 'temp' | 'size',
        ),
      ),
    ).toBe(true);
    expect(getPhysicalDiskColumnWidthStyle('compact', 'disk')).toEqual({ width: '34%' });
    expect(getPhysicalDiskColumnWidthStyle('compact', 'temp')).toEqual({ width: '12%' });
    expect(getPhysicalDiskColumnWidthStyle('compact', 'size')).toEqual({ width: '15%' });
    expect(getPhysicalDiskColumnWidthStyle('operational', 'life')).toEqual({ width: '9%' });
    expect(getPhysicalDiskColumnWidthStyle('compact', 'device')).toEqual({ width: '0%' });
    expect(isPhysicalDiskColumnVisible('narrow', 'life')).toBe(false);
    expect(isPhysicalDiskColumnVisible('narrow', 'temp')).toBe(true);
    expect(getPhysicalDiskColumnWidthStyle('narrow', 'disk')).toEqual({ width: '41%' });
    expect(getPhysicalDiskColumnWidthStyle('narrow', 'temp')).toEqual({ width: '12%' });
    expect(getPhysicalDiskColumnWidthStyle('narrow', 'size')).toEqual({ width: '16%' });
  });

  it('keeps the desktop disk column widths on the canonical weighted helper', () => {
    const columns = [
      'disk',
      'device',
      'host',
      'role',
      'parent',
      'health',
      'life',
      'temp',
      'size',
    ] as const;
    expect(columns.map((column) => getPhysicalDiskColumnWidthStyle('full', column).width)).toEqual([
      '19%',
      '9%',
      '10%',
      '8%',
      '12%',
      '17%',
      '6%',
      '7%',
      '12%',
    ]);
  });

  it('sheds the shared cell padding only on the phone disk layouts', () => {
    expect(getPhysicalDiskCellPaddingClass('narrow')).toBe('px-1!');
    expect(getPhysicalDiskCellPaddingClass('compact')).toBe('px-1!');
    expect(getPhysicalDiskCellPaddingClass('basic')).toBe('');
    expect(getPhysicalDiskCellPaddingClass('full')).toBe('');
  });

  it('shortens only the health words that cannot fit a phone health column', () => {
    expect(getPhysicalDiskHealthCompactLabel('Needs Attention')).toBe('Attention');
    expect(getPhysicalDiskHealthCompactLabel('Replace Now')).toBe('Replace');
    expect(getPhysicalDiskHealthCompactLabel('Running Hot')).toBe('Hot');
    expect(getPhysicalDiskHealthCompactLabel('Healthy')).toBe('Healthy');
    expect(getPhysicalDiskHealthCompactLabel('Unknown')).toBe('Unknown');
  });

  describe('heat versus replacement evidence', () => {
    const buildRiskDisk = (
      id: string,
      risk: { level: string; reasons: { code: string; severity: string; summary: string }[] },
      overrides: Record<string, unknown> = {},
    ): Resource =>
      ({
        id,
        name: id,
        type: 'physical_disk',
        status: 'warning',
        physicalDisk: {
          devPath: `/dev/${id}`,
          model: 'Crucial MX500 2TB',
          diskType: 'sata',
          health: 'PASSED',
          wearout: 95,
          temperature: 72,
          risk,
          ...overrides,
        },
        identity: { hostname: 'pve3' },
        canonicalIdentity: { hostname: 'pve3' },
        platformType: 'proxmox-pve',
      }) as unknown as Resource;
    const hot = (severity: string, celsius = 72) => ({
      code: 'temperature_high',
      severity,
      summary: `Disk temperature is ${celsius}C`,
    });

    it('calls a disk that is only hot Running Hot at the tier of its reading', () => {
      const criticalDisk = buildRiskDisk('sda', { level: 'critical', reasons: [hot('critical')] });
      const criticalData = extractPhysicalDiskPresentationData(criticalDisk);
      expect(criticalData.riskReasonDetails).toEqual([hot('critical')]);
      expect(getPhysicalDiskHealthStatus(criticalData)).toEqual({
        label: 'Running Hot',
        summary: 'Disk temperature is 72C',
        tone: 'text-red-700 dark:text-red-300',
      });
      expect(getPhysicalDiskNormalizedHealth(criticalDisk, criticalData)).toBe('critical');

      const warmDisk = buildRiskDisk(
        'sdb',
        { level: 'warning', reasons: [hot('warning', 63)] },
        { temperature: 63 },
      );
      const warmData = extractPhysicalDiskPresentationData(warmDisk);
      expect(getPhysicalDiskHealthStatus(warmData)).toEqual({
        label: 'Running Hot',
        summary: 'Disk temperature is 63C',
        tone: 'text-amber-700 dark:text-amber-300',
      });
      expect(getPhysicalDiskNormalizedHealth(warmDisk, warmData)).toBe('warning');
    });

    it('keeps Replace Now for failure evidence even when heat is listed first', () => {
      // Merged risks keep insertion order, so heat can precede the reason
      // that actually calls for a new disk.
      const disk = buildRiskDisk('sda', {
        level: 'critical',
        reasons: [
          hot('critical'),
          {
            code: 'pending_sectors',
            severity: 'critical',
            summary: 'Pending sectors detected (2)',
          },
        ],
      });
      const data = extractPhysicalDiskPresentationData(disk);
      expect(getPhysicalDiskHealthStatus(data)).toEqual({
        label: 'Replace Now',
        summary: 'Pending sectors detected (2)',
        tone: 'text-red-700 dark:text-red-300',
      });

      const failed = extractPhysicalDiskPresentationData(
        buildRiskDisk(
          'sdb',
          {
            level: 'critical',
            reasons: [
              hot('critical'),
              {
                code: 'health_status',
                severity: 'critical',
                summary: 'Disk reports health status FAILED',
              },
            ],
          },
          { health: 'FAILED' },
        ),
      );
      expect(getPhysicalDiskHealthStatus(failed).label).toBe('Replace Now');
      expect(getPhysicalDiskHealthStatus(failed).summary).toBe('Disk reports health status FAILED');
    });

    it('lets the more severe of heat and wear evidence name the verdict', () => {
      const hotAndWorn = extractPhysicalDiskPresentationData(
        buildRiskDisk('sda', {
          level: 'critical',
          reasons: [
            hot('critical'),
            { code: 'wearout_low', severity: 'warning', summary: 'SSD life remaining is 8%' },
          ],
        }),
      );
      expect(getPhysicalDiskHealthStatus(hotAndWorn).label).toBe('Running Hot');
      expect(getPhysicalDiskHealthStatus(hotAndWorn).summary).toBe('Disk temperature is 72C');

      const warmAndWorn = extractPhysicalDiskPresentationData(
        buildRiskDisk(
          'sdb',
          {
            level: 'warning',
            reasons: [
              hot('warning', 63),
              { code: 'wearout_low', severity: 'warning', summary: 'SSD life remaining is 8%' },
            ],
          },
          { temperature: 63 },
        ),
      );
      expect(getPhysicalDiskHealthStatus(warmAndWorn)).toEqual({
        label: 'Needs Attention',
        summary: 'SSD life remaining is 8%',
        tone: 'text-amber-700 dark:text-amber-300',
      });
    });

    it('keeps a critical level that no listed reason explains as Replace Now', () => {
      const data = extractPhysicalDiskPresentationData(
        buildRiskDisk('sda', { level: 'critical', reasons: [hot('warning', 63)] }),
      );
      expect(getPhysicalDiskHealthStatus(data)).toEqual({
        label: 'Replace Now',
        summary: 'Disk health has degraded to a critical state.',
        tone: 'text-red-700 dark:text-red-300',
      });
    });

    it('decides the verdict from codes and severities even without display text', () => {
      const pendingWithoutText = extractPhysicalDiskPresentationData(
        buildRiskDisk('sda', {
          level: 'critical',
          reasons: [
            hot('critical'),
            { code: 'pending_sectors', severity: 'critical', summary: '' },
          ],
        }),
      );
      expect(pendingWithoutText.riskReasons).toEqual(['Disk temperature is 72C']);
      expect(getPhysicalDiskHealthStatus(pendingWithoutText)).toEqual({
        label: 'Replace Now',
        summary: 'Disk health has degraded to a critical state.',
        tone: 'text-red-700 dark:text-red-300',
      });

      const heatWithoutText = extractPhysicalDiskPresentationData(
        buildRiskDisk('sdb', {
          level: 'critical',
          reasons: [{ code: 'temperature_high', severity: 'critical', summary: '' }],
        }),
      );
      expect(getPhysicalDiskHealthStatus(heatWithoutText).label).toBe('Running Hot');
    });

    it('does not let a weaker reason stand in for an unexplained critical level', () => {
      const data = extractPhysicalDiskPresentationData(
        buildRiskDisk('sda', {
          level: 'critical',
          reasons: [
            { code: 'crc_errors', severity: 'monitor', summary: 'UDMA CRC errors detected (2)' },
            hot('warning', 63),
          ],
        }),
      );
      expect(getPhysicalDiskHealthStatus(data)).toEqual({
        label: 'Replace Now',
        summary: 'Disk health has degraded to a critical state.',
        tone: 'text-red-700 dark:text-red-300',
      });
    });

    it('reads severities case-insensitively and in any order within a class', () => {
      const shouting = extractPhysicalDiskPresentationData(
        buildRiskDisk('sda', {
          level: 'CRITICAL',
          reasons: [{ ...hot('critical'), severity: ' Critical ' }],
        }),
      );
      expect(getPhysicalDiskHealthStatus(shouting).label).toBe('Running Hot');
      expect(getPhysicalDiskHealthStatus(shouting).tone).toBe('text-red-700 dark:text-red-300');

      const unsorted = extractPhysicalDiskPresentationData(
        buildRiskDisk('sdb', {
          level: 'critical',
          reasons: [
            { code: 'wearout_low', severity: 'warning', summary: 'SSD life remaining is 8%' },
            {
              code: 'pending_sectors',
              severity: 'critical',
              summary: 'Pending sectors detected (2)',
            },
          ],
        }),
      );
      expect(getPhysicalDiskHealthStatus(unsorted).summary).toBe('Pending sectors detected (2)');
    });

    it('lets SMART counters and low life outrank warning heat', () => {
      const warmAndLow = extractPhysicalDiskPresentationData(
        buildRiskDisk(
          'sda',
          { level: 'warning', reasons: [hot('warning', 63)] },
          { diskType: 'ssd', wearout: 8, temperature: 63 },
        ),
      );
      expect(getPhysicalDiskHealthStatus(warmAndLow)).toEqual({
        label: 'Needs Attention',
        summary: 'SSD life is running low.',
        tone: 'text-amber-700 dark:text-amber-300',
      });

      const warmAndReallocating = extractPhysicalDiskPresentationData(
        buildRiskDisk(
          'sdb',
          { level: 'warning', reasons: [hot('warning', 63)] },
          { temperature: 63, smart: { reallocatedSectors: 3 } },
        ),
      );
      expect(getPhysicalDiskHealthStatus(warmAndReallocating).label).toBe('Needs Attention');
    });

    it('sorts a FAILED disk with no risk payload ahead of a critically hot one', () => {
      const failedDisk = buildRiskDisk(
        'sdb',
        { level: 'healthy', reasons: [] },
        { risk: undefined, health: 'FAILED', temperature: 40 },
      );
      const hotDisk = buildRiskDisk('sda', { level: 'critical', reasons: [hot('critical')] });
      const failedData = extractPhysicalDiskPresentationData(failedDisk);
      const hotData = extractPhysicalDiskPresentationData(hotDisk);
      expect(getPhysicalDiskHealthStatus(failedData).label).toBe('Replace Now');
      expect(
        comparePhysicalDiskPresentation(failedDisk, failedData, hotDisk, hotData),
      ).toBeLessThan(0);
    });

    it('sorts and filters hot disks by the action their verdict asks for', () => {
      const warm = buildRiskDisk('sdc', { level: 'warning', reasons: [hot('warning', 63)] });
      const worn = buildRiskDisk('sdd', {
        level: 'warning',
        reasons: [
          { code: 'wearout_low', severity: 'warning', summary: 'SSD life remaining is 8%' },
        ],
      });
      const hotDisk = buildRiskDisk('sda', { level: 'critical', reasons: [hot('critical')] });
      const failing = buildRiskDisk('sdb', {
        level: 'critical',
        reasons: [
          { code: 'media_errors', severity: 'critical', summary: 'Media errors detected (4)' },
        ],
      });
      const healthy = buildRiskDisk(
        'sde',
        { level: 'healthy', reasons: [] },
        { risk: undefined, temperature: 38 },
      );
      const disks = [healthy, warm, worn, hotDisk, failing];
      const dataMap = buildPhysicalDiskPresentationDataMap(disks);
      const sorted = (healthFilter: 'all' | 'attention' | 'critical' | 'warning') =>
        filterAndSortPhysicalDisks(disks, {
          selectedNode: null,
          healthFilter,
          searchTerm: '',
          getDiskData: (disk) => dataMap.get(disk.id)!,
          matchesNode: () => true,
        }).map((disk) => disk.id);

      // Device order alone would put each hot disk first.
      expect(sorted('all')).toEqual(['sdb', 'sda', 'sdd', 'sdc', 'sde']);
      expect(sorted('attention')).toEqual(['sdb', 'sda', 'sdd', 'sdc']);
      expect(sorted('critical')).toEqual(['sdb', 'sda']);
      expect(sorted('warning')).toEqual(['sdd', 'sdc']);
    });
  });

  it('detects SMART warnings from counters', () => {
    expect(
      hasPhysicalDiskSmartWarning({
        ...makeDiskData({
          health: 'PASSED',
          wearout: 50,
          type: 'hdd',
        }),
        smartAttributes: { pendingSectors: 2 },
      }),
    ).toBe(true);
  });

  it('returns role, parent, and platform labels canonically', () => {
    expect(
      getPhysicalDiskRoleLabel({
        ...makeDiskData({
          health: 'PASSED',
          wearout: 50,
          type: 'ssd',
        }),
        storageRole: 'cache_pool',
      }),
    ).toBe('Cache Pool');
    expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'nvme' }))).toBe('NVMe disk');
    expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'sata' }))).toBe('SATA disk');
    expect(getPhysicalDiskRoleLabel(makeDiskData({ type: 'sas' }))).toBe('SAS disk');
    expect(
      getPhysicalDiskParentLabel({
        ...makeDiskData({
          health: 'PASSED',
          wearout: 50,
          type: 'ssd',
        }),
        storageGroup: 'tank',
      }),
    ).toBe('tank');
    expect(getPhysicalDiskParentLabel(makeDiskData({ storageGroup: '', used: 'ZFS' }))).toBe('ZFS');
    expect(getPhysicalDiskParentLabel(makeDiskData({ storageGroup: 'tank', used: 'ZFS' }))).toBe(
      'tank',
    );
    expect(getPhysicalDiskParentLabel(makeDiskData({ storageGroup: '', used: 'unknown' }))).toBe(
      '',
    );
    expect(getPhysicalDiskLifeLabel(makeDiskData({ wearout: 96 }))).toBe('96%');
    expect(getPhysicalDiskLifeLabel(makeDiskData({ wearout: 0 }))).toBe('0%');
    expect(getPhysicalDiskLifeLabel(makeDiskData({ wearout: -1 }))).toBe('');
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 96 }))).toContain('text-green');
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 35 }))).toContain('text-amber');
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 9 }))).toContain('text-red');
    expect(getPhysicalDiskPlatformLabel({} as Resource, 'PVE')).toBe('PVE');
    expect(
      getPhysicalDiskSourceBadgePresentation({
        platformType: 'proxmox-pve',
      } as Resource),
    ).toEqual({
      label: 'PVE',
      className:
        'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-400 inline-flex max-w-full min-w-0 justify-center overflow-hidden text-ellipsis px-1 sm:px-1.5 py-px text-[9px] font-medium',
    });
    expect(
      getPhysicalDiskSourceKey({
        platformType: 'agent',
        platformData: { sources: ['agent', 'proxmox'] },
      } as unknown as Resource),
    ).toBe('proxmox-pve');
    expect(
      getPhysicalDiskSourceBadgePresentation({
        platformType: 'agent',
        platformData: { sources: ['agent', 'proxmox'] },
      } as unknown as Resource).label,
    ).toBe('PVE');
    expect(
      getPhysicalDiskSourceKey({
        platformType: 'agent',
        sources: ['agent'],
        platformScopes: ['agent', 'proxmox-pve'],
      } as unknown as Resource),
    ).toBe('proxmox-pve');
  });

  it('returns host, health summary, and empty-state presentation canonically', () => {
    expect(
      getPhysicalDiskHostLabel(
        {
          ...makeDiskData({
            health: 'PASSED',
            wearout: 50,
            type: 'ssd',
          }),
          node: 'tower',
        },
        { parentName: 'fallback-host' } as Resource,
      ),
    ).toBe('tower');

    expect(
      getPhysicalDiskHealthSummary({
        label: 'Healthy',
        summary: 'No active disk-health issues.',
        tone: 'text-base-content',
      }),
    ).toBe('');

    expect(
      getPhysicalDiskHealthSummary({
        label: 'Needs Attention',
        summary: 'Pending sectors detected.',
        tone: 'text-amber-700 dark:text-amber-300',
      }),
    ).toBe('Pending sectors detected.');
    expect(getPhysicalDiskHealthStatus(makeDiskData({ health: 'UNKNOWN' })).label).toBe('Unknown');
    // PVE reports SCSI/SAS drives as OK; older servers pass it through raw (#1595)
    expect(getPhysicalDiskHealthStatus(makeDiskData({ health: 'OK' })).label).toBe('Healthy');

    expect(
      getPhysicalDiskEmptyStatePresentation({
        selectedNodeName: 'tower',
        searchTerm: '',
        diskCount: 0,
        hasPVENodes: true,
      }),
    ).toMatchObject({
      title: 'No physical disks found',
      nodeMessage: 'for node tower',
      filterMessages: [],
      showRequirements: true,
      fallbackMessage:
        'No Proxmox nodes configured. Add Proxmox VE in Settings → Infrastructure to monitor physical disks.',
      requirementsTitle: 'Physical disk monitoring requirements:',
      requirementsItems: expect.arrayContaining([
        'Enable "Monitor physical disk health (SMART)" in Settings → Infrastructure for the Proxmox node',
      ]),
    });

    expect(
      getPhysicalDiskEmptyStatePresentation({
        selectedNodeName: null,
        searchTerm: '',
        diskCount: 4,
        hasPVENodes: true,
        healthFilter: 'attention',
        sourceFilterLabel: 'PVE',
        roleFilterLabel: 'Cache Pool',
        groupFilterLabel: 'tank',
      }),
    ).toMatchObject({
      title: 'No disks need attention',
      filterMessages: ['from PVE', 'with role Cache Pool', 'in tank'],
      showRequirements: false,
    });
  });

  it('keeps Unraid disks with missing SMART data out of attention unless Unraid reports a fault', () => {
    const spunDownUnraidDisk = makeDiskData({
      health: 'UNKNOWN',
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'online',
      spunDown: true,
      temperature: 0,
    });
    const spunDownResource = {
      status: 'offline',
      platformType: 'agent',
      physicalDisk: {
        health: 'UNKNOWN',
        storageRole: 'data',
        storageGroup: 'unraid-array',
        storageState: 'online',
        spunDown: true,
      },
    } as unknown as Resource;

    expect(isUnraidPhysicalDisk(spunDownUnraidDisk)).toBe(true);
    expect(hasUnraidPhysicalDiskFaultSignal(spunDownUnraidDisk)).toBe(false);
    expect(getPhysicalDiskHealthStatus(spunDownUnraidDisk)).toMatchObject({
      label: 'Online',
      summary: 'No active disk-health issues.',
    });
    expect(getPhysicalDiskNormalizedHealth(spunDownResource, spunDownUnraidDisk)).toBe('healthy');

    const diskWithUnraidErrors = makeDiskData({
      health: 'UNKNOWN',
      riskLevel: 'warning',
      riskReasons: ['Unraid disk disk2 reports 3 error(s)'],
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'online',
      errorCount: 3,
    });
    expect(hasUnraidPhysicalDiskFaultSignal(diskWithUnraidErrors)).toBe(true);
    expect(getPhysicalDiskHealthStatus(diskWithUnraidErrors)).toMatchObject({
      label: 'Needs Attention',
      summary: 'Unraid disk disk2 reports 3 error(s)',
    });

    const missingUnraidDisk = makeDiskData({
      health: 'FAILED',
      storageRole: 'data',
      storageGroup: 'unraid-array',
      storageState: 'missing',
      riskLevel: 'critical',
      riskReasons: ['Unraid disk disk3 is MISSING'],
    });
    expect(hasUnraidPhysicalDiskFaultSignal(missingUnraidDisk)).toBe(true);
    expect(getPhysicalDiskHealthStatus(missingUnraidDisk)).toMatchObject({
      label: 'Replace Now',
      summary: 'Unraid disk disk3 is MISSING',
    });
  });

  it('extracts search and sort presentation canonically from resources', () => {
    const warningDisk = {
      id: 'disk-warning',
      name: 'disk-warning',
      physicalDisk: {
        devPath: '/dev/sdb',
        model: 'Cache SSD',
        serial: 'SERIAL-2',
        diskType: 'ssd',
        health: 'PASSED',
        storageRole: 'cache_pool',
        storageGroup: 'tank',
        risk: {
          level: 'warning',
          reasons: [{ summary: 'Pending sectors detected.' }],
        },
        smart: { pendingSectors: 2 },
      },
      identity: { hostname: 'tower' },
      canonicalIdentity: { hostname: 'tower' },
      platformType: 'proxmox-pve',
    } as unknown as Resource;

    const healthyDisk = {
      id: 'disk-healthy',
      name: 'disk-healthy',
      physicalDisk: {
        devPath: '/dev/sda',
        model: 'Archive HDD',
        serial: 'SERIAL-1',
        diskType: 'hdd',
        health: 'PASSED',
      },
      identity: { hostname: 'tower' },
      canonicalIdentity: { hostname: 'tower' },
      platformType: 'proxmox-pve',
    } as unknown as Resource;

    const warningData = extractPhysicalDiskPresentationData(warningDisk);
    const healthyData = extractPhysicalDiskPresentationData(healthyDisk);

    expect(warningData.model).toBe('Cache SSD');
    expect(warningData.riskReasons).toEqual(['Pending sectors detected.']);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'cache')).toBe(true);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'tank')).toBe(true);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'node:tower')).toBe(true);
    expect(matchesPhysicalDiskSearch(warningDisk, warningData, 'node:pve1')).toBe(false);
    expect(
      matchesPhysicalDiskSearch(
        warningDisk,
        {
          ...warningData,
          wwn: '5-c500-da60ca43',
          controller: '/dev/sg2',
          target: 'sssraid,0,1',
          type: 'sas',
          instance: 'cluster-a',
        },
        '5-c500 sssraid cluster-a sas',
      ),
    ).toBe(true);
    expect(getPhysicalDiskSourceKey(warningDisk)).toBe('proxmox-pve');
    expect(getPhysicalDiskRoleFilterValue(warningData)).toBe('cache-pool');
    expect(normalizePhysicalDiskFacetFilter(' Cache Pool ')).toBe('cache-pool');
    expect(buildPhysicalDiskRoleFilterOptions([warningDisk, healthyDisk])).toContainEqual({
      value: 'cache-pool',
      label: 'Cache Pool',
    });
    expect(buildPhysicalDiskRoleFilterOptions([warningDisk, healthyDisk])[0]).toEqual({
      value: 'all',
      label: PHYSICAL_DISK_ALL_ROLES_FILTER_LABEL,
    });
    expect(buildPhysicalDiskGroupFilterOptions([warningDisk, healthyDisk])).toContainEqual({
      value: 'tank',
      label: 'tank',
    });
    expect(buildPhysicalDiskGroupFilterOptions([warningDisk, healthyDisk])[0]).toEqual({
      value: 'all',
      label: PHYSICAL_DISK_ALL_GROUPS_FILTER_LABEL,
    });
    expect(getPhysicalDiskNormalizedHealth(warningDisk, warningData)).toBe('warning');
    expect(
      comparePhysicalDiskPresentation(warningDisk, warningData, healthyDisk, healthyData),
    ).toBeLessThan(0);
    expect(buildPhysicalDiskPresentationDataMap([warningDisk, healthyDisk]).size).toBe(2);
    expect(
      filterAndSortPhysicalDisks([warningDisk, healthyDisk], {
        selectedNode: null,
        searchTerm: 'cache',
        sourceFilter: 'proxmox-pve',
        healthFilter: 'attention',
        roleFilter: 'cache-pool',
        groupFilter: 'tank',
        getDiskData: (disk) => (disk.id === warningDisk.id ? warningData : healthyData),
        matchesNode: () => true,
      }).map((disk) => disk.id),
    ).toEqual(['disk-warning']);
    expect(
      filterAndSortPhysicalDisks(
        [
          warningDisk,
          {
            ...healthyDisk,
            id: 'pbs-disk',
            platformType: 'proxmox-pbs',
            sources: ['agent', 'pbs'],
          },
        ],
        {
          selectedNode: null,
          searchTerm: '',
          sourceFilter: 'proxmox-all',
          healthFilter: 'all',
          getDiskData: (disk) => (disk.id === warningDisk.id ? warningData : healthyData),
          matchesNode: () => true,
        },
      ).map((disk) => disk.id),
    ).toEqual(['disk-warning', 'pbs-disk']);
    expect(
      filterAndSortPhysicalDisks(
        [
          {
            ...healthyDisk,
            id: 'megaraid-member',
            platformType: 'agent',
            sources: ['agent'],
            platformScopes: ['agent', 'proxmox-pve'],
          },
        ],
        {
          selectedNode: null,
          searchTerm: '',
          sourceFilter: 'proxmox-all',
          healthFilter: 'all',
          getDiskData: () => healthyData,
          matchesNode: () => true,
        },
      ).map((disk) => disk.id),
    ).toEqual(['megaraid-member']);
    expect(
      filterAndSortPhysicalDisks([warningDisk, healthyDisk], {
        selectedNode: null,
        searchTerm: '',
        sourceFilter: 'agent',
        healthFilter: 'all',
        getDiskData: (disk) => (disk.id === warningDisk.id ? warningData : healthyData),
        matchesNode: () => true,
      }).map((disk) => disk.id),
    ).toEqual([]);
  });
});

describe('wearout evidence boundary', () => {
  // Mirrors storagehealth.WearoutReported on the server. The Physical Disks
  // view and the alert path have to agree about the same disk, so both gate on
  // this predicate rather than each picking their own boundary.
  it('treats -1 as unreported and a real 0 as evidence only from endurance-reporting devices', () => {
    expect(isPhysicalDiskWearoutReported(makeDiskData({ wearout: -1, type: 'ssd' }))).toBe(false);
    expect(isPhysicalDiskWearoutReported(makeDiskData({ wearout: 0, type: 'ssd' }))).toBe(true);
    expect(isPhysicalDiskWearoutReported(makeDiskData({ wearout: 0, type: 'NVMe' }))).toBe(true);
    expect(isPhysicalDiskWearoutReported(makeDiskData({ wearout: 0, type: 'hdd' }))).toBe(false);
    expect(isPhysicalDiskWearoutReported(makeDiskData({ wearout: 0, type: '' }))).toBe(false);
    expect(isPhysicalDiskWearoutReported(makeDiskData({ wearout: 42, type: 'hdd' }))).toBe(true);
  });

  it('flags a spent SSD but leaves a rotational zero alone', () => {
    expect(
      getPhysicalDiskHealthStatus(makeDiskData({ health: 'PASSED', wearout: 0, type: 'ssd' }))
        .label,
    ).toBe('Needs Attention');
    expect(
      getPhysicalDiskHealthStatus(makeDiskData({ health: 'PASSED', wearout: 0, type: 'hdd' }))
        .label,
    ).toBe('Healthy');
  });

  it('mutes the life column when there is no endurance evidence', () => {
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: -1, type: 'ssd' }))).toBe(
      PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
    );
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 0, type: 'hdd' }))).toBe(
      PHYSICAL_DISK_MUTED_PLACEHOLDER_CLASS,
    );
    expect(getPhysicalDiskLifeTextClass(makeDiskData({ wearout: 0, type: 'ssd' }))).toBe(
      'text-red-600 dark:text-red-400',
    );
  });
});
