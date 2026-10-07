import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string) => readFileSync(path.join(root, name), 'utf8');
const guide = read('docs/CONFIGURATION.md');
const fragment = 'backup-polling-and-guest-safety';

function render(markdown: string, slug: string): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, slug);
  return article;
}

function help() {
  const section = guide.split('### Backup polling and guest safety')[1]?.split('\n### ')[0];
  expect(section).toBeTruthy();
  const article = render(section!, 'CONFIGURATION');
  return { article, text: article.textContent?.replace(/\s+/g, ' ') ?? '' };
}

describe('backup polling safety help', () => {
  it('ships one safety explanation in the public configuration guide', () => {
    expect(read('frontend-modern/public/docs/CONFIGURATION.md')).toBe(guide);
    const article = render(guide, 'CONFIGURATION');
    expect(article.querySelector(`#${fragment}`)?.textContent).toBe(
      'Backup polling and guest safety',
    );
    expect(article.querySelectorAll(`[id="${fragment}"]`)).toHaveLength(1);
  });

  it('makes the distinction reachable from each stored and environment setting', () => {
    const article = render(guide, 'CONFIGURATION');
    for (const key of [
      'backupPollingEnabled',
      'backupPollingInterval',
      'ENABLE_BACKUP_POLLING',
      'BACKUP_POLLING_INTERVAL',
    ]) {
      const cell = [...article.querySelectorAll('tbody tr td:first-child')].find(
        (element) => element.textContent === key,
      );
      expect(cell, key).toBeDefined();
      const link = cell!.parentElement?.querySelector(`a[href="#${fragment}"]`);
      expect(link?.textContent, key).toBe('Backup polling and guest safety');
      // Same-document fragments use the viewer's native fragment handler,
      // not its cross-document route marker.
      expect(link?.hasAttribute('data-doc-link'), key).toBe(false);
    }
    expect(article.querySelector(`#${fragment}`)).not.toBeNull();
  });

  it('distinguishes record collection from guest-agent reads and provider backup execution', () => {
    const { text } = help();
    expect(text).toContain('Enable backup polling');
    expect(text).toContain(
      'controls whether Pulse schedules collection of Proxmox/PBS backup records',
    );
    expect(text).toContain(
      'Turning it off does not pause ordinary PVE monitoring or its QEMU Guest Agent disk, memory and metadata reads',
    );
    expect(text).toContain('It does not stop Proxmox or PBS from running backup jobs');
    expect(text).toContain('does not cancel work already in flight');
    expect(text).toContain('Do not use this switch as a freeze-enabled backup safety precaution');
  });

  it('keeps automatic cadence and override precedence distinct from disabling collection', () => {
    const { text } = help();
    expect(text).toContain('backupPollingInterval and BACKUP_POLLING_INTERVAL');
    expect(text).toContain('0 means automatic cadence, not disabled');
    expect(text).toContain('Environment overrides take precedence over system.json');
    expect(text).toContain('lock the corresponding UI controls');
    expect(text).toContain('Schedule configuration changes outside backup or freeze/thaw windows');
    const article = render(guide, 'CONFIGURATION');
    for (const key of ['backupPollingInterval', 'BACKUP_POLLING_INTERVAL']) {
      const row = [...article.querySelectorAll('tbody tr')].find(
        (element) => element.querySelector('td')?.textContent === key,
      );
      expect(row?.textContent, key).toContain('0 = auto, not disabled');
    }
  });

  it('does not turn retained backup evidence into current protection or a disruptive diagnostic', () => {
    const { article, text } = help();
    expect(text).toContain('Longer intervals delay backup evidence refresh');
    expect(text).toContain('disabled polling leaves it unrefreshed');
    expect(text).toContain(
      'A retained backup row or posture is not proof of a current observation, a successful restore or guest thaw',
    );
    expect(text).toContain('matching workload, datastore, namespace and artifact time');
    expect(text).toContain("provider's own tools");
    expect(text).toContain('do not clear history or run another backup');
    expect(article.querySelector('pre')).toBeNull();
  });

  it('keeps the complete independent safety boundary rather than accepting an OK task or responsiveness', () => {
    const { text } = help();
    expect(text).toContain('pause the actual Pulse server');
    expect(text).toContain('prevent its updater or deployment controller from restarting it');
    expect(text).toContain('This is not incident recovery');
    expect(text).toContain('stopping Pulse does not cancel a guest-agent request already issued');
    expect(text).toContain('until the backup has ended and independent post-backup checks confirm');
    expect(text).toContain(
      'thaw, fresh successful workload writes to every filesystem covered by the backup, and workload liveness',
    );
    expect(text).toContain('independent of Pulse and the QEMU Guest Agent');
    expect(text).toContain('not new probes or forced writes');
    expect(text).toContain('If any check fails or is unavailable, leave Pulse and its updater paused');
    expect(text).toContain('only services and timers that were active before the pause');
    expect(text).toContain('do not guess unknown pre-pause states');
    expect(text).toContain('Pulse monitoring and alerts are unavailable while stopped');
    expect(text).toContain('arrange independent outage coverage');
    expect(text).toContain('does not establish a fixed monitoring defect');
  });

  it('links to the existing deployment-specific precaution without adding a probe command', () => {
    const { article } = help();
    expect(article.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]')?.textContent).toBe(
      'backup safety precaution',
    );
    const safety = read('docs/VM_DISK_MONITORING.md');
    expect(read('frontend-modern/public/docs/VM_DISK_MONITORING.md')).toBe(safety);
    const target = render(safety, 'VM_DISK_MONITORING');
    for (const id of [
      'backup-safety',
      'pause-pulse-for-a-planned-freeze-enabled-backup',
      'pause-a-docker-or-compose-server-for-a-planned-backup',
    ]) {
      expect(target.querySelector(`#${id}`), id).not.toBeNull();
    }
    expect(article.querySelector('pre')).toBeNull();
    expect(article.textContent).not.toMatch(
      /qm agent|pvesh|guest-fsfreeze|force-reset|disable backup freezing/,
    );
  });
});
