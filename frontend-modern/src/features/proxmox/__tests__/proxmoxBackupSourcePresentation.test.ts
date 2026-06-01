import { describe, expect, it } from 'vitest';

import {
  PROXMOX_BACKUP_SOURCE_KINDS,
  getProxmoxArchiveSourceTitle,
  getProxmoxBackupSourcePresentation,
} from '../proxmoxBackupSourcePresentation';

describe('proxmoxBackupSourcePresentation', () => {
  it('keeps Proxmox backup source vocabulary canonical across filters and badges', () => {
    expect(PROXMOX_BACKUP_SOURCE_KINDS).toEqual(['pbs', 'archive', 'snapshot']);

    expect(getProxmoxBackupSourcePresentation('pbs')).toMatchObject({
      badgeLabel: 'PBS',
      coverageColumnLabel: 'Latest PBS',
      filterLabel: 'PBS snapshots',
      timelineLabel: 'PBS snapshots',
    });
    expect(getProxmoxBackupSourcePresentation('archive')).toMatchObject({
      badgeLabel: 'PVE file',
      coverageColumnLabel: 'Latest PVE file',
      filterLabel: 'PVE backup files',
      timelineLabel: 'PVE backup files',
    });
    expect(getProxmoxBackupSourcePresentation('snapshot')).toMatchObject({
      badgeLabel: 'Snapshot',
      coverageColumnLabel: 'Latest snapshot',
      filterLabel: 'Guest snapshots',
      timelineLabel: 'Guest snapshots',
    });
  });

  it('explains PVE file rows without making them look like direct PBS rows', () => {
    expect(getProxmoxArchiveSourceTitle(false)).toContain('Proxmox VE storage');
    expect(getProxmoxArchiveSourceTitle(true)).toBe(
      'Backup volume reported by Proxmox VE from a PBS-backed storage target',
    );
  });
});
