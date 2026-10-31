import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { buildProxmoxBackupRecoveryModel } from '@/features/proxmox/proxmoxBackupRecoveryModel';
import {
  BACKUP_IDENTITY_NOW,
  identityGuest,
  identityPBS,
} from '@/features/proxmox/__fixtures__/backupIdentity';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) => read(`frontend-modern/public/docs/${name}.md`));
const fragment = '#restore-point-sizes-and-unknown-values';
const section = () => {
  const heading = guide('RECOVERY').querySelector(fragment);
  expect(heading?.textContent).toBe('Restore-point sizes and unknown values');
  const result = document.createElement('section');
  for (
    let next = heading!.nextElementSibling;
    next && next.tagName !== 'H2';
    next = next.nextElementSibling
  ) {
    result.appendChild(next.cloneNode(true));
  }
  return result;
};
const text = () => section().textContent!.replace(/\s+/g, ' ');

describe('restore-point size guidance', () => {
  it('ships a reachable explanation with the same wide and narrow inspection labels', () => {
    expect(read('frontend-modern/public/docs/RECOVERY.md')).toBe(read('docs/RECOVERY.md'));
    expect(guide('RECOVERY').querySelector(`a[href="${fragment}"]`)?.textContent).toBe(
      'individual size checks',
    );
    const consumer = read('frontend-modern/src/features/proxmox/ProxmoxRecoverableTable.tsx');
    for (const label of ['Unknown size', 'Restore point', 'Size']) {
      expect(text()).toContain(label);
      expect(consumer).toContain(`'${label}'`);
    }
    expect(text()).toContain('On a phone, expand the restore-point row');
    expect(text()).toContain('A hidden column is not a missing reading');
    expect(text()).toContain(
      'Sorting it keeps measured sizes ahead of unknowns in both directions',
    );
    expect(text()).toContain("narrow layouts show it alongside the point's identity");
  });

  it.each([undefined, 0, 1])("explains the current model's size evidence for %s", (size) => {
    const model = buildProxmoxBackupRecoveryModel({
      workloads: [identityGuest('fixture', 'east', 'pve-1')],
      pbsBackups: [identityPBS({ size })],
      archives: [],
      snapshots: [],
      tasks: [],
      nowMs: BACKUP_IDENTITY_NOW,
    });
    expect(model.recoverableArtifacts[0].size).toBe(size ? size : undefined);
    expect(model.coverageSummary.recoverableArtifacts).toBe(1);
    expect(text()).toContain('Missing sizes and zero placeholders both remain unknown');
    expect(text()).toContain('not that the restore point is empty, corrupt or failed');
    expect(text()).toContain('its completion, verification and protection are separate evidence');
    expect(text()).toContain("not the guest's disk capacity");
  });

  it('uses existing records and keeps size inspection separate from guest recovery', () => {
    const copy = text();
    expect(copy).toContain('Compare the same workload, repository and time');
    expect(copy).toContain('Do not run another backup, verification or restore');
    expect(copy).toContain('delete a point or clear History just to obtain a size');
    expect(copy).toContain('Older versions may show a dash or zero instead');
    expect(copy).toContain('stop this display comparison');
    expect(section().querySelector('pre')).toBeNull();
    for (const [name, anchor] of [
      ['RECOVERY', 'why-proxmox-backup-counts-differ'],
      ['VM_DISK_MONITORING', 'backup-safety'],
    ]) {
      const href = name === 'RECOVERY' ? `#${anchor}` : `/docs/${name}#${anchor}`;
      expect(section().querySelector(`a[href="${href}"]`)).not.toBeNull();
      expect(guide(name).querySelector(`#${anchor}`)).not.toBeNull();
    }
  });
});
