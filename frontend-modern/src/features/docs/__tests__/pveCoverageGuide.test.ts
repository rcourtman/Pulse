import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { getProxmoxBackupSourcePresentation } from '@/features/proxmox/proxmoxBackupSourcePresentation';
import { COVERAGE_COLUMNS } from '@/features/proxmox/proxmoxBackupsTablePresentation';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) => read(`frontend-modern/public/docs/${name}.md`));
const fragment = '#local-pve-backups-are-newer-than-coverage';
const section = () => {
  const heading = guide('RECOVERY').querySelector(fragment);
  expect(heading?.textContent).toBe('Local PVE backups are newer than Coverage');
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-3]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
};
const prose = () => section().textContent!.replace(/\s+/g, ' ');

describe('local PVE Coverage comparison guidance', () => {
  it('links both existing help entries to the shipped comparison and keeps mirrors equal', () => {
    for (const name of ['PBS', 'TROUBLESHOOTING']) {
      const link = guide(name).querySelector(`a[href="/docs/RECOVERY${fragment}"]`);
      expect(link?.textContent).toBe('PVE Coverage comparison');
    }
    for (const name of ['PBS', 'TROUBLESHOOTING', 'RECOVERY']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
    expect(section().querySelectorAll('ol > li')).toHaveLength(4);
  });

  it('uses current source labels and keeps filtered evidence separate from canonical posture', () => {
    const text = prose();
    const source = getProxmoxBackupSourcePresentation('archive');
    for (const label of [source.filterLabel, source.compactFilterLabel]) {
      expect(text).toContain(label);
    }
    const latest = COVERAGE_COLUMNS.find((column) => column.id === 'latest')!;
    expect(text).toContain(latest.label);
    for (const distinction of [
      'not the protection policy',
      "does not refresh collection or change a backup's age",
      "does not recalculate the workload's posture",
      'the inventory reader, not necessarily local storage',
      'creation time',
      'By date to locate an omitted archive',
      'Unknown is not None',
      'native node, guest type/ID, storage, volume identifier and archive time',
    ]) {
      expect(text).toContain(distinction);
    }
  });

  it('keeps comparison passive, original identifiers private and guest recovery independent', () => {
    const text = prose();
    expect(section().querySelector('pre')).toBeNull();
    for (const boundary of [
      'retain the exact originals privately',
      'credentials and full inventories out of public reports',
      'Do not run another backup, verification or restore',
      'restart or re-enrol a node',
      'clear History, change retention or widen permissions',
      'stop this display comparison',
      'does not establish guest thaw',
      'every covered filesystem or workload liveness',
    ]) {
      expect(text).toContain(boundary);
    }
    const safety = '#backup-safety';
    expect(section().querySelector(`a[href="/docs/VM_DISK_MONITORING${safety}"]`)).not.toBeNull();
    expect(guide('VM_DISK_MONITORING').querySelector(safety)).not.toBeNull();
  });
});
