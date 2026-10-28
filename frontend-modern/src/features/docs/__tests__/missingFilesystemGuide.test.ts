import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = createGuideFixture((name) => {
  const directory =
    process.env.PULSE_MISSING_FILESYSTEM_GUIDE_INPUT_DIRECTORY ??
    path.join(root, 'frontend-modern/public/docs');
  return readFileSync(path.join(directory, `${name}.md`), 'utf8');
});
const fragment = '#one-filesystem-is-missing-but-other-disks-have-readings';

function section() {
  const heading = guide('VM_DISK_MONITORING').querySelector(fragment);
  expect(heading?.textContent).toBe('One filesystem is missing but other disks have readings');
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-3]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
}

const prose = (element: Element) => element.textContent!.replace(/\s+/g, ' ').trim();

describe('missing local filesystem guidance', () => {
  it('reaches the shipped explanation from troubleshooting and the general disk context', () => {
    const link = guide('TROUBLESHOOTING').querySelector(
      `a[href="/docs/VM_DISK_MONITORING${fragment}"]`,
    );
    expect(link?.textContent).toBe('missing-filesystem checks');
    const disk = guide('VM_DISK_MONITORING');
    expect(disk.querySelector(`a[href="${fragment}"]`)?.textContent).toBe(
      'missing-filesystem checks',
    );
    expect(section()).toBeTruthy();
    for (const name of ['VM_DISK_MONITORING', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });

  it('separates allocated capacity, real local filesystems and intentionally excluded layers', () => {
    const content = section();
    const observations = [...content.querySelectorAll('ul > li')];
    expect(observations).toHaveLength(3);
    expect(observations.map((item) => item.querySelector('strong')?.textContent)).toEqual([
      'Attached virtual disk:',
      'Separate local filesystem:',
      'Excluded layer or remote mount:',
    ]);
    expect(prose(observations[0])).toContain('does not establish a mounted filesystem');
    expect(prose(observations[1])).toContain('not a container layer just because');
    expect(prose(observations[1])).toContain('unknown in Pulse, not zero');
    expect(prose(observations[2])).toContain('overlay, tmpfs, NFS and SMB');
    // Avoid another horizontally scrolling table in the current Docs reader.
    expect(content.querySelector('table')).toBeNull();
    const text = prose(content);
    expect(text).toContain('Pulse v6.6.0 filters /var/lib/containers and /var/lib/docker by path');
    expect(text).toContain('even when the QEMU Guest Agent returns it');
    expect(text).toContain('A missing filesystem at another path can have a different cause');
    expect(text).toContain('Backup inclusion and capacity monitoring are separate');
    expect(text).toContain('Leave an intentionally excluded disk excluded');
    expect(text).toContain('enabling backups is not a capacity-monitoring repair');
  });

  it('offers an existing capacity view without remounting, agent installation or backup changes', () => {
    const content = section();
    const text = prose(content);
    for (const distinction of [
      'same guest and collection source',
      "use the guest's established filesystem view",
      'does not repair the missing API reading or establish complete monitoring',
      'Do not remount or move data, change backup selection, install another agent',
      'restart services or run guest-agent probes',
      'Backup safety takes precedence',
    ]) {
      expect(text).toContain(distinction);
    }
    for (const target of ['use-existing-machines-readings-without-changing-the-guest', 'backup-safety']) {
      expect(content.querySelector(`a[href="#${target}"]`)).not.toBeNull();
      expect(guide('VM_DISK_MONITORING').querySelector(`#${target}`)).not.toBeNull();
    }
    expect(content.querySelector('pre')).toBeNull();
  });

  it('uses supplied source-specific facts and preserves private mount and configuration evidence', () => {
    const text = prose(section());
    for (const boundary of [
      'use facts already available',
      'whether this is the Proxmox API or a Pulse Agent view',
      'which filesystem is missing and which still have readings',
      'if already known; unknown is valid',
      'consistent private aliases',
      'without private suffixes',
      'Do not post full mount tables, fstab, VM configuration, raw guest-agent responses or Diagnostics exports',
      'no need to collect these facts again if they are in the existing report',
    ]) {
      expect(text).toContain(boundary);
    }
  });
});
