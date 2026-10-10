import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = read('docs/PBS.md');
const section =
  guide.split('### Backups are visible but Coverage says Unprotected')[1]?.split('\n### ')[0] ?? '';
const render = (markdown: string, name: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
};
const article = () => render(section, 'PBS');
const text = () => article().textContent?.replace(/\s+/g, ' ');

describe('PBS protection disagreement help', () => {
  it('ships the section and distinguishes matching, verification and assessment from recovery proof', () => {
    expect(read('frontend-modern/public/docs/PBS.md')).toBe(guide);
    expect(
      render(guide, 'PBS').querySelector('#backups-are-visible-but-coverage-says-unprotected'),
    ).not.toBeNull();
    const rows = [...article().querySelectorAll('tbody tr')].map((row) => row.textContent);
    expect(rows).toHaveLength(3);
    expect(rows[0]).toContain('subject-linked backup evidence');
    expect(rows[0]).toContain('not identity proof');
    expect(rows[1]).toContain('not a successful restore');
    expect(rows[1]).toContain('every covered disk');
    expect(rows[2]).toContain('not instructions to change backups');
    expect(text()).toContain("PBS's own backup and verification records");
    expect(text()).toContain('do not assume either a failed backup or working protection');
  });

  it('uses existing Coverage evidence and source identity without an induced request', () => {
    const steps = [...article().querySelectorAll('ol > li')].map((step) =>
      step.textContent?.replace(/\s+/g, ' '),
    );
    expect(steps).toHaveLength(3);
    expect(steps[0]).toContain('Proxmox → Backups → Coverage');
    expect(steps[0]).toContain('Job, History and Access');
    expect(steps[0]).toContain('provider evidence is absent');
    expect(steps[0]).toContain('not the PBS host');
    expect(steps[1]).toContain(
      'server, datastore, namespace (including root), guest type/ID and backup time',
    );
    expect(steps[1]).toContain('confirm the owning installation');
    expect(steps[1]).toContain('guest snapshot is not a PBS backup');
    expect(steps[2]).toContain('protection explanation');
    expect(article().querySelector('pre')).toBeNull();
    const coverage = read('frontend-modern/src/features/proxmox/ProxmoxCoverageTable.tsx');
    expect(coverage).toContain('Job {evidenceQualityLabel(provider.jobState)}');
    expect(coverage).toContain('History {evidenceQualityLabel(provider.historyCompleteness)}');
    expect(coverage).toContain('Access {evidenceQualityLabel(provider.permissions)}');
  });

  it('prevents destructive diagnostics and an assumed freshness-policy cause', () => {
    expect(text()).toContain(
      'Do not delete backups, clear History, change retention or freshness settings',
    );
    expect(text()).toContain('recreate tokens/connections, restart or downgrade Pulse');
    expect(text()).toContain('run a new backup, verification or restore just to clear the strip');
    expect(text()).toContain('not a destructive diagnostic or an assumed freshness-policy cause');
    expect(text()).toContain('Use your established backup checks meanwhile');
  });

  it('links the frozen-guest boundary and preserves every restoration prerequisite', () => {
    const link = article().querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]');
    expect(link?.textContent).toBe('Backup safety');
    expect(
      render(read('docs/VM_DISK_MONITORING.md'), 'VM_DISK_MONITORING').querySelector(
        '#backup-safety',
      ),
    ).not.toBeNull();
    expect(text()).toContain(
      'An OK backup task or a Verified label does not prove the guest thawed',
    );
    expect(text()).toContain('not this display check');
    expect(text()).toContain('independent post-backup checks confirm thaw');
    expect(text()).toContain(
      'fresh successful writes to every filesystem covered by the backup and workload liveness',
    );
    expect(text()).toContain('restore only services and timers active before the pause');
    expect(text()).toContain('Monitoring and alerts are unavailable while Pulse is stopped');
  });

  it('limits public diagnostics to the selected evidence without credentials or broad exports', () => {
    expect(text()).toContain(
      'Use consistent placeholders for private server, datastore, namespace and guest identities',
    );
    expect(text()).toContain(
      'Do not share tokens, full API responses, HAR exports, or screenshots with secrets',
    );
    expect(text()).toContain("one guest's protection explanation, PBS Job/History/Access values");
  });
});
