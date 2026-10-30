import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildBackupServerRows } from '@/features/proxmox/ProxmoxBackupServersTable';
import { parsePBSBackupInventory } from '@/features/proxmox/proxmoxBackupInventory';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) => read(`frontend-modern/public/docs/${name}.md`));
const fragment = '#what-the-backup-server-count-means';
const section = () => {
  const heading = guide('PBS').querySelector(fragment);
  expect(heading?.textContent).toBe('What the backup server count means');
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-3]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
};
const text = () => section().textContent!.replace(/\s+/g, ' ');
const server = (name: string, datastore = 'tank'): Resource => ({
  id: `service-${name}`,
  name,
  displayName: 'Shared display name',
  type: 'pbs',
  status: 'online',
  lastSeen: 1_793_300_000_000,
  platformId: 'proxmox-pbs',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  pbs: {
    instanceId: `pbs-${name}`,
    hostname: 'shared.example.test',
    datastores: [{ name: datastore, total: 1000, used: 400 }],
  },
});
const entry = (instance: string | undefined, datastore = 'tank', namespace = '') => ({
  id: JSON.stringify([instance, datastore, namespace]),
  instance,
  datastore,
  namespace,
  backupType: 'vm',
  vmid: '100',
  backupTime: '2026-10-30T01:00:00Z',
  files: ['index.json.blob'],
});
const rows = (servers: Resource[], entries: ReturnType<typeof entry>[]) =>
  buildBackupServerRows(servers, parsePBSBackupInventory({ data: { backups: entries } }).backups);

describe('PBS server count guidance', () => {
  it('ships a discoverable section, matching mirror and usable related links', () => {
    expect(read('frontend-modern/public/docs/PBS.md')).toBe(read('docs/PBS.md'));
    expect(guide('PBS').querySelector(`a[href="${fragment}"]`)?.textContent).toBe(
      'backup server counts',
    );
    for (const [name, hash] of [
      ['RECOVERY', '#why-proxmox-backup-counts-differ'],
      ['PBS', '#slow-backup-loading'],
      ['VM_DISK_MONITORING', '#backup-safety'],
    ]) {
      const href = name === 'PBS' ? hash : `/docs/${name}${hash}`;
      expect(section().querySelector(`a[href="${href}"]`)).not.toBeNull();
      expect(guide(name).querySelector(hash)).not.toBeNull();
    }
  });

  it('explains the real count across namespaces without filtering in-progress or unverified entries', () => {
    const entries = [entry('main'), entry('main', 'tank', 'child')];
    Object.assign(entries[1], { inProgress: true, verified: false });
    expect(rows([server('main')], entries)[0].backupCount).toBe(2);
    expect(text()).toContain('across its namespaces');
    expect(text()).toContain('An in-progress or unverified entry can count too');
    expect(text()).toContain('not a count of guests, completed or verified restore points');
    expect(text()).toContain('separate from the filtered By date list');
  });

  it('keeps a genuine matching zero separate from another reader, datastore or missing identity', () => {
    const result = rows(
      [server('main'), server('pbs-main')],
      [entry('pbs-main'), entry('main', 'other'), entry(undefined)],
    );
    expect(result.map((row) => [row.serverName, row.backupCount])).toEqual([
      ['main', 0],
      ['pbs-main', 1],
    ]);
    expect(text()).toContain('exact configured connection and datastore');
    expect(text()).toContain('0 does not prove that the datastore is empty');
    expect(text()).toContain('or that every expected backup was collected');
    expect(text()).toContain(
      'an entry without reader or datastore identity cannot fill a named row',
    );
    expect(text()).toContain('Shared hostnames, display labels or guest IDs');
  });

  it('distinguishes inventory failure, loading, absent datastore and a hidden column from zero', () => {
    const meanings = [...section().querySelectorAll('tbody tr')].map((row) =>
      row.textContent!.replace(/\s+/g, ' ').trim(),
    );
    expect(meanings).toHaveLength(4);
    expect(meanings[1]).toContain('Loading The PBS inventory has not loaded yet');
    expect(meanings[2]).toContain('Unavailable The PBS inventory read failed');
    expect(meanings[2]).toContain(
      'reachability and datastore capacity can still be shown independently',
    );
    expect(meanings[3]).toContain('— This server row has no reported datastore to count');
    expect(text()).toContain('A hidden column is not missing inventory');
  });

  it('keeps comparison passive and private, with a separate guest-safety exit', () => {
    for (const boundary of [
      'Compare existing records',
      'Do not share full inventories or credentials, rename connections, widen permissions',
      'clear History or run another backup',
      'not repeated retries',
      'Older versions may attribute counts differently',
      'A count is not protection, verification, restore readiness or guest thaw',
      'stop the display comparison',
    ]) {
      expect(text()).toContain(boundary);
    }
    expect(section().querySelector('pre')).toBeNull();
  });
});
