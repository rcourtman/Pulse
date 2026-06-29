import { describe, expect, it } from 'vitest';
import { diskResourceId, type PhysicalDisk } from '@/types/api';

function createDisk(overrides: Partial<PhysicalDisk> = {}): PhysicalDisk {
  return {
    id: 'inst-node1-dev-sda',
    node: 'node1',
    instance: 'inst',
    devPath: '/dev/sda',
    model: 'Test Disk',
    serial: '',
    wwn: '',
    type: 'sata',
    size: 0,
    health: 'PASSED',
    wearout: -1,
    temperature: 0,
    rpm: 0,
    used: '',
    lastChecked: new Date(0).toISOString(),
    ...overrides,
  };
}

describe('diskResourceId', () => {
  it('prefers canonical disk ID before serial or WWN', () => {
    expect(diskResourceId(createDisk({ serial: 'SER123', wwn: 'WWN123' }))).toBe(
      'inst-node1-dev-sda',
    );
  });

  it('falls back to serial then WWN when canonical disk ID is unavailable', () => {
    expect(diskResourceId(createDisk({ id: '', serial: 'SER123', wwn: 'WWN123' }))).toBe(
      'SER123',
    );
    expect(diskResourceId(createDisk({ id: '', wwn: 'WWN123' }))).toBe('WWN123');
  });

  it('skips placeholder identities and falls back to canonical disk ID', () => {
    expect(diskResourceId(createDisk({ serial: 'N/A', wwn: 'unknown' }))).toBe(
      'inst-node1-dev-sda',
    );
  });
});
