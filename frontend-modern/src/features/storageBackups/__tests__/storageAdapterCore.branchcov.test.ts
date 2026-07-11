import { describe, expect, it } from 'vitest';
import type { StorageRecord } from '@/features/storageBackups/models';
import {
  canonicalStorageIdentityKey,
  normalizeStorageResourceHealth,
  resolveStoragePlatformFamily,
} from '@/features/storageBackups/storageAdapterCore';

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'pve1', scope: 'node' },
  capacity: { totalBytes: 100, usedBytes: 40, freeBytes: 60, usagePercent: 40 },
  capabilities: ['capacity'],
  source: {
    platform: 'proxmox-pve',
    family: 'virtualization',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

// ---------------------------------------------------------------------------
// normalizeHealthValue  (module-private — exercised via normalizeStorageResourceHealth)
// ---------------------------------------------------------------------------

describe('normalizeHealthValue via normalizeStorageResourceHealth', () => {
  it('returns unknown when every signal is undefined or blank', () => {
    expect(normalizeStorageResourceHealth(undefined, undefined, undefined)).toBe('unknown');
    expect(normalizeStorageResourceHealth('', [], '')).toBe('unknown');
    expect(normalizeStorageResourceHealth('   ', undefined, '  ')).toBe('unknown');
  });

  it.each([
    'online',
    'running',
    'available',
    'healthy',
    'ok',
    'optimal',
  ])('maps exact healthy keyword %s to "healthy"', (status) => {
    expect(normalizeStorageResourceHealth(status, undefined)).toBe('healthy');
  });

  it.each([
    'warning',
    'warn',
    'degraded',
    'health_warn',
  ])('maps exact warning keyword %s to "warning"', (status) => {
    expect(normalizeStorageResourceHealth(status, undefined)).toBe('warning');
  });

  it.each([
    'critical',
    'faulted',
    'failed',
    'error',
    'unhealthy',
    'health_crit',
    'health_err',
  ])('maps exact critical keyword %s to "critical"', (status) => {
    expect(normalizeStorageResourceHealth(status, undefined)).toBe('critical');
  });

  it.each(['offline', 'stopped', 'down', 'unavailable'])(
    'maps exact offline keyword %s to "offline"',
    (status) => {
      expect(normalizeStorageResourceHealth(status, undefined)).toBe('offline');
    },
  );

  it('maps the exact literal "unknown" to "unknown"', () => {
    expect(normalizeStorageResourceHealth('unknown', undefined)).toBe('unknown');
  });

  it('falls through to unknown when the status matches no keyword or substring arm', () => {
    expect(normalizeStorageResourceHealth('quantum-state', undefined)).toBe('unknown');
  });

  it('maps substring (non-exact) critical signals to "critical"', () => {
    expect(normalizeStorageResourceHealth('system fault detected', undefined)).toBe('critical');
    expect(normalizeStorageResourceHealth('prefail warning', undefined)).toBe('critical');
  });

  it('maps substring (non-exact) warning signals to "warning"', () => {
    // 'prewarn-state' includes 'warn' but matches no exact keyword nor any critical substring
    expect(normalizeStorageResourceHealth('prewarn-state', undefined)).toBe('warning');
    expect(normalizeStorageResourceHealth('mildly degraded-ish', undefined)).toBe('warning');
  });

  it('maps substring (non-exact) offline signals to "offline"', () => {
    // 'system shutdown' includes 'down' (via shut**down**) but is not the exact keyword 'down'
    expect(normalizeStorageResourceHealth('system shutdown', undefined)).toBe('offline');
  });

  it('maps substring (non-exact) healthy signals to "healthy"', () => {
    // 'back online soon' includes 'online' but is not the exact keyword 'online'
    expect(normalizeStorageResourceHealth('back online soon', undefined)).toBe('healthy');
  });

  it('normalizes case and surrounding whitespace before classification', () => {
    expect(normalizeStorageResourceHealth('ONLINE', undefined)).toBe('healthy');
    expect(normalizeStorageResourceHealth('  CriTiCal  ', undefined)).toBe('critical');
    expect(normalizeStorageResourceHealth('\tWarn\n', undefined)).toBe('warning');
  });
});

// ---------------------------------------------------------------------------
// extractHealthTag  (module-private — exercised via normalizeStorageResourceHealth)
// ---------------------------------------------------------------------------

describe('extractHealthTag via normalizeStorageResourceHealth', () => {
  it('ignores a missing, empty, or non-health tag array and falls back to status', () => {
    expect(normalizeStorageResourceHealth('online', undefined)).toBe('healthy');
    expect(normalizeStorageResourceHealth('online', [])).toBe('healthy');
    expect(normalizeStorageResourceHealth('critical', ['region:eu', 'tier:gold'])).toBe('critical');
  });

  it('reads the trimmed value of a "health:" tag and overrides the status', () => {
    expect(normalizeStorageResourceHealth('online', ['health:warning'])).toBe('warning');
    expect(normalizeStorageResourceHealth('online', ['health:  Critical  '])).toBe('critical');
  });

  it('uses the LAST health: tag when several are present', () => {
    expect(
      normalizeStorageResourceHealth('online', ['health:ok', 'health:critical']),
    ).toBe('critical');
    expect(
      normalizeStorageResourceHealth('offline', ['health:critical', 'health:healthy']),
    ).toBe('healthy');
  });

  it('matches the health: prefix case-insensitively after trimming each tag', () => {
    expect(normalizeStorageResourceHealth('online', ['  HEALTH:down  '])).toBe('offline');
  });
});

describe('normalizeStorageResourceHealth priority chain', () => {
  it('prefers incidentSeverity over health tag over status', () => {
    // incidentSeverity beats a health tag and the status
    expect(
      normalizeStorageResourceHealth('online', ['health:warning'], 'critical'),
    ).toBe('critical');
    // health tag beats status
    expect(normalizeStorageResourceHealth('online', ['health:critical'])).toBe('critical');
  });
});

// ---------------------------------------------------------------------------
// resolveStoragePlatformFamily
// ---------------------------------------------------------------------------

describe('resolveStoragePlatformFamily', () => {
  it('defers to the platform support manifest for known platforms', () => {
    // These resolve to a non-null storage family in the manifest.
    expect(resolveStoragePlatformFamily('kubernetes')).toBe('container');
    expect(resolveStoragePlatformFamily('docker')).toBe('container');
    expect(resolveStoragePlatformFamily('proxmox-pve')).toBe('virtualization');
    expect(resolveStoragePlatformFamily('aws')).toBe('cloud');
    expect(resolveStoragePlatformFamily('truenas')).toBe('onprem');
  });

  it('lowercases the platform before resolving so casing does not matter', () => {
    expect(resolveStoragePlatformFamily('KUBERNETES')).toBe('container');
    expect(resolveStoragePlatformFamily('Proxmox-PVE')).toBe('virtualization');
  });

  it('falls back to "container" for unrecognized platforms containing kubernetes/docker', () => {
    expect(resolveStoragePlatformFamily('my-kubernetes-cluster')).toBe('container');
    expect(resolveStoragePlatformFamily('docker-swarm-x')).toBe('container');
  });

  it('falls back to "cloud" for unrecognized platforms containing "cloud"', () => {
    expect(resolveStoragePlatformFamily('private-cloud-net')).toBe('cloud');
  });

  it('falls back to "virtualization" for unrecognized platforms containing proxmox/vmware/hyperv', () => {
    expect(resolveStoragePlatformFamily('vmware-test-xyz')).toBe('virtualization');
    expect(resolveStoragePlatformFamily('custom-hyperv-node')).toBe('virtualization');
    expect(resolveStoragePlatformFamily('proxmox-unlisted-build')).toBe('virtualization');
  });

  it('falls back to "generic" when the value contains "generic" but is not a manifest id', () => {
    // 'generic' itself has a manifest entry whose storage family is null, so it also reaches
    // the substring arm; an unlisted variant exercises the same arm unambiguously.
    expect(resolveStoragePlatformFamily('generic')).toBe('generic');
    expect(resolveStoragePlatformFamily('generic-backup-target')).toBe('generic');
  });

  it('falls back to "onprem" for a value matching no manifest entry and no substring', () => {
    expect(resolveStoragePlatformFamily('local-zfs-node')).toBe('onprem');
    expect(resolveStoragePlatformFamily('')).toBe('onprem');
  });
});

// ---------------------------------------------------------------------------
// canonicalStorageIdentityKey  (also exercises module-private normalizeIdentityPart)
// ---------------------------------------------------------------------------

describe('canonicalStorageIdentityKey', () => {
  it('joins trimmed/lowercased platform, location, name, and category', () => {
    expect(canonicalStorageIdentityKey(makeRecord())).toBe('proxmox-pve|pve1|tank|pool');
  });

  it('trims and lowercases every identity part', () => {
    expect(
      canonicalStorageIdentityKey(
        makeRecord({
          source: { platform: '  TrueNAS ', family: 'onprem', origin: 'resource', adapterId: 'a' },
          location: { label: ' TOWER ', scope: 'host' },
          name: ' Tank ',
          category: ' Dataset ' as unknown as StorageRecord['category'],
        }),
      ),
    ).toBe('truenas|tower|tank|dataset');
  });

  it('defaults a missing platform to "generic"', () => {
    expect(
      canonicalStorageIdentityKey(
        makeRecord({
          source: { platform: '', family: 'onprem', origin: 'resource', adapterId: 'a' },
        }),
      ),
    ).toBe('generic|pve1|tank|pool');
  });

  it('falls back to refs.platformEntityId when location.label is blank', () => {
    expect(
      canonicalStorageIdentityKey(
        makeRecord({
          location: { label: '', scope: 'node' },
          refs: { platformEntityId: 'ent-9' },
        }),
      ),
    ).toBe('proxmox-pve|ent-9|tank|pool');
  });

  it('uses "unknown-location" when both location.label and refs.platformEntityId are blank', () => {
    expect(
      canonicalStorageIdentityKey(
        makeRecord({ location: { label: '', scope: 'node' }, refs: {} }),
      ),
    ).toBe('proxmox-pve|unknown-location|tank|pool');
    // refs absent entirely as well
    expect(
      canonicalStorageIdentityKey(
        makeRecord({ location: { label: '   ', scope: 'node' }, refs: undefined }),
      ),
    ).toBe('proxmox-pve|unknown-location|tank|pool');
  });

  it('falls back to the record id when name is blank', () => {
    expect(canonicalStorageIdentityKey(makeRecord({ name: '' }))).toBe(
      'proxmox-pve|pve1|storage-1|pool',
    );
  });

  it('uses "unknown-name" when both name and id are blank', () => {
    expect(
      canonicalStorageIdentityKey(makeRecord({ name: '', id: '' })),
    ).toBe('proxmox-pve|pve1|unknown-name|pool');
  });

  it('defaults a missing category to "other"', () => {
    expect(
      canonicalStorageIdentityKey(
        makeRecord({ category: '' as unknown as StorageRecord['category'] }),
      ),
    ).toBe('proxmox-pve|pve1|tank|other');
  });
});
