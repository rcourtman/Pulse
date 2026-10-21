import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';
import type { RecoverableArtifact } from '@/features/proxmox/proxmoxBackupRecoveryModel';
import { artifactStateLabel } from '@/features/proxmox/proxmoxBackupsTableShared';

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
    // The section also contains job-identity/timing tables. Assert the safety
    // comparison itself, not every descendant row added by those subsections.
    const comparisons = [...article().querySelectorAll('table')].filter(
      (table) => table.querySelector('thead th')?.textContent === 'Observation in Pulse',
    );
    expect(comparisons).toHaveLength(1);
    const rows = [...comparisons[0].querySelectorAll('tbody tr')].map((row) => row.textContent);
    expect(rows).toHaveLength(4);
    expect(rows[0]).toContain('subject-linked backup evidence');
    expect(rows[0]).toContain('not identity proof');
    expect(rows[1]).toContain('not a successful restore');
    expect(rows[1]).toContain('every covered disk');
    expect(rows[2]).toContain("restore point's State");
    expect(rows[2]).toContain('not the workload');
    expect(rows[3]).toContain('not instructions to change backups');
    expect(text()).toContain("PBS's own backup and verification records");
    expect(text()).toContain('do not assume either a failed backup or working protection');
  });

  it('uses existing Coverage evidence and source identity without an induced request', () => {
    const steps = [...article().querySelectorAll('ol > li')].map((step) =>
      step.textContent?.replace(/\s+/g, ' '),
    );
    expect(steps).toHaveLength(3);
    expect(steps[0]).toContain('Proxmox → Backups → Coverage');
    expect(steps[0]).toContain('Scope and Source');
    expect(steps[0]).toContain('Result, History and Access');
    expect(steps[0]).toContain('A successful inventory read is not a successful backup job');
    expect(steps[0]).toContain('Older versions call Result Job');
    expect(steps[0]).toContain('keep absent identity unknown');
    expect(steps[0]).toContain('provider evidence is absent');
    expect(steps[0]).toContain('not the PBS host');
    expect(steps[1]).toContain(
      'server, datastore, namespace (including root), guest type/ID and backup time',
    );
    expect(steps[1]).toContain('confirm the owning installation');
    expect(steps[1]).toContain('choose the newest point whose Source is PBS');
    expect(steps[1]).toContain('Created time and State');
    expect(steps[1]).toContain('Hover the Created age for its absolute timestamp');
    expect(steps[1]).toContain('if unavailable, keep the time unknown');
    expect(steps[1]).toContain('refresh time is not a matching backup timestamp');
    expect(steps[1]).toContain('guest snapshot is not a PBS backup');
    expect(steps[2]).toContain('protection explanation');
    expect(article().querySelector('pre')).toBeNull();
    const coverage = read('frontend-modern/src/features/proxmox/ProxmoxCoverageTable.tsx')
      .replace(/\{' '\}/g, ' ')
      .replace(/\s+/g, ' ');
    expect(coverage).toContain('Result {evidenceQualityLabel(provider.jobState)}');
    expect(coverage).toContain('data-backup-provider-scope');
    expect(coverage).toContain('data-backup-provider-source');
    expect(coverage).toContain('History {evidenceQualityLabel(provider.historyCompleteness)}');
    expect(coverage).toContain('Access {evidenceQualityLabel(provider.permissions)}');
  });

  it('names the current coverage strip without losing the older-version signpost', () => {
    expect(text()).toContain('Backup coverage strip (called Backup health in older versions)');
    const source = read('frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx');
    expect(source).toContain('title="Backup coverage"');
  });

  it('distinguishes the available observations without claiming a PBS cause or verification failure', () => {
    expect(text()).toContain('an older or absent point in Pulse is an inventory discrepancy');
    expect(text()).toContain(
      'Unverified on the matching point means Pulse has no successful verification evidence',
    );
    expect(text()).toContain("does not prove that PBS's verification job failed");
    expect(text()).toContain(
      'Verified with a workload still needing attention is a separate assessment disagreement',
    );
    expect(text()).toContain('retain its explanation and Provider evidence');
    expect(text()).toContain("Job OK is not an individual point's verification result");
    expect(text()).toContain('without running Diagnostics or another verification');
  });

  it('keeps pruning separate from every reported or absent verification value', () => {
    const point: RecoverableArtifact = {
      id: 'fixture-point',
      nativeId: 'fixture-point',
      sourceKind: 'pbs',
      sourceLabel: 'PBS',
      workload: {
        key: 'fixture-guest',
        type: 'vm',
        typeLabel: 'VM',
        vmid: '100',
        label: 'Fixture VM',
      },
      createdAt: '2026-10-12T02:00:00Z',
      location: 'fixture / (root)',
      detail: '',
      protected: true,
    };
    for (const [verified, label] of [
      [true, 'Verified'],
      [false, 'Unverified'],
      [undefined, 'Unknown'],
    ] as const) {
      expect(artifactStateLabel({ ...point, verified })).toBe(label);
      expect(artifactStateLabel({ ...point, protected: false, verified })).toBe(label);
    }
    expect(artifactStateLabel({ ...point, protected: false, verified: true })).toBe('Verified');
    expect(artifactStateLabel({ ...point, protected: false, verified: false })).toBe('Unverified');
    expect(text()).toContain(
      'Prune protected does not change the verification result alongside it',
    );
    expect(text()).toContain(
      "provider's protection-from-pruning flag, shown separately from verification",
    );
    expect(text()).toContain('Unknown on a PBS point means no verification result was reported');
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
    expect(text()).toContain(
      "one guest's protection explanation, the PBS scope/source and Result/History/Access values",
    );
  });
});
