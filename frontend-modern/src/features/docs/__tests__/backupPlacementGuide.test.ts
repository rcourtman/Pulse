import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  getBackupWorkloadNodeLabel,
  getBackupWorkloadPVEConnection,
} from '@/features/proxmox/proxmoxBackupsTableShared';
import { type WorkloadReference } from '@/features/proxmox/proxmoxBackupRecoveryModel';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) =>
  readFileSync(
    path.join(
      process.env.PULSE_BACKUP_PLACEMENT_GUIDE_INPUT_DIRECTORY ??
        path.join(root, 'frontend-modern/public/docs'),
      `${name}.md`,
    ),
    'utf8',
  ),
);
const fragment = '#backup-location-and-guest-placement';
const section = () => {
  const heading = guide('RECOVERY').querySelector(fragment);
  expect(heading?.textContent).toBe('Backup location and guest placement');
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-3]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
};
const prose = () => section().textContent!.replace(/\s+/g, ' ');

describe('backup placement guidance', () => {
  it('keeps discovery links and shipped copies aligned', () => {
    for (const name of ['PBS', 'TROUBLESHOOTING']) {
      expect(guide(name).querySelector(`a[href="/docs/RECOVERY${fragment}"]`)).not.toBeNull();
    }
    for (const name of ['RECOVERY', 'PBS', 'TROUBLESHOOTING']) {
      expect(read(`docs/${name}.md`)).toBe(read(`frontend-modern/public/docs/${name}.md`));
    }
  });

  it('distinguishes reported guest placement from repository and reader scope', () => {
    const rows = [...section().querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(3);
    expect(rows[0].textContent).toContain('Target ID');
    expect(rows[0].textContent).toContain('PVE connection');
    expect(rows[1].textContent).toContain('A repository name is not a guest node');
    expect(rows[2].textContent).toContain('not a substitute for missing workload ownership');
    const unmatched: WorkloadReference = {
      key: 'pbs:private-example:vm/100',
      type: 'vm',
      typeLabel: 'VM',
      vmid: '100',
      label: 'VM 100',
    };
    expect(rows[0].textContent).toContain(getBackupWorkloadNodeLabel(unmatched));
    expect(rows[0].textContent).toContain(getBackupWorkloadPVEConnection(unmatched));
    for (const distinction of [
      'does not mean that a backup failed, a guest was deleted or its node is offline',
      'without a known PVE owner',
      'Do not infer that owner or node from the datastore, namespace, connection label or a repeated VMID',
      'that label alone is not native placement evidence',
    ]) {
      expect(prose()).toContain(distinction);
    }
  });

  it('finds unmatched evidence without treating a display filter as repair', () => {
    for (const distinction of [
      'Node filter offers reported placement, not every backup location',
      'choose All nodes',
      'Backup location or search for its repository',
      'not evidence that its backups are missing',
      'neither refreshes collection nor repairs identity or protection posture',
      'On a phone, expand the row',
    ]) {
      expect(prose()).toContain(distinction);
    }
  });

  it('keeps comparison private and partial recovery distinct from original symptoms', () => {
    for (const boundary of [
      'Compare existing provider records',
      'Keep unknown ownership unknown',
      'do not rename nodes, move backups, widen permissions or recreate a connection',
      'Keep full identifiers and private paths out of public reports',
      'one corrected backup label does not establish recovery in those other views',
      'An unresponsive guest or uncertain thaw needs the separate backup safety procedure',
    ]) {
      expect(prose()).toContain(boundary);
    }
    expect(section().querySelector('pre')).toBeNull();
    for (const [name, target] of [
      ['TROUBLESHOOTING', '#monitoring-is-mixed-between-proxmox-installations'],
      ['VM_DISK_MONITORING', '#backup-safety'],
    ]) {
      expect(section().querySelector(`a[href="/docs/${name}${target}"]`)).not.toBeNull();
      expect(guide(name).querySelector(target)).not.toBeNull();
    }
  });
});
