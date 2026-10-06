import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (relative: string): string => readFileSync(path.join(repoRoot, relative), 'utf8');
const guide = read('docs/PBS.md');
const heading = 'Backup health disagrees with visible PBS backups';
const fragment = 'backup-health-disagrees-with-visible-pbs-backups';
const section = guide.split(`### ${heading}\n`)[1]?.split('\n### ')[0] ?? '';
const render = (markdown: string, slug: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, slug);
  return article;
};
const text = () => render(section, 'PBS').textContent?.replace(/\s+/g, ' ');

// These checks exercise the shipped renderer and navigation, not a new
// classification oracle. The owning posture tests establish actual states.
describe('backup health help', () => {
  it('ships the same guide and makes the specific check reachable from both help entry points', () => {
    const target = render(guide, 'PBS');
    expect(target.querySelector(`#${fragment}`)?.textContent).toBe(heading);
    for (const slug of ['PBS', 'RECOVERY', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${slug}.md`)).toBe(read(`docs/${slug}.md`));
    }
    for (const slug of ['RECOVERY', 'TROUBLESHOOTING']) {
      const link = render(read(`docs/${slug}.md`), slug).querySelector(
        `a[href="/docs/PBS#${fragment}"]`,
      );
      expect(link?.textContent).toBe('backup health checks');
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
    }
  });

  it('separates listed artifacts, individual verification and workload-level protection', () => {
    const rows = [...render(section, 'PBS').querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(3);
    expect(text()).toContain('rates monitored workloads, not individual artifacts');
    expect(text()).toContain('protection calculation has the same linked evidence');
    expect(text()).toContain(
      'Every workload is protected, guest thaw succeeded or an application restore works',
    );
    expect(text()).toContain('it is not the label for an old successful backup alone');
    expect(text()).toContain('a PBS backup snapshot is different');
    expect(text()).not.toMatch(/(?:7|seven)[ -]day/);
    expect(render(section, 'PBS').querySelector('pre')).toBeNull();
  });

  it('names the existing expansion and provider labels without inventing a diagnostic action', () => {
    const ui = read('frontend-modern/src/features/proxmox/ProxmoxCoverageTable.tsx');
    expect(ui).toContain('{postureExplanation(row)}');
    for (const [label, field] of [
      ['Job', 'jobState'],
      ['History', 'historyCompleteness'],
      ['Access', 'permissions'],
    ]) {
      expect(ui).toContain(`${label} {evidenceQualityLabel(provider.${field})}`);
      expect(text()).toContain(label);
    }
    expect(text()).toContain('Access describes permissions');
    expect(text()).toContain('If the provider evidence is absent, record that');
    expect(text()).toContain(
      'Use the existing page; do not run Run Diagnostics or a guest-agent probe',
    );
  });

  it('retains native provenance and missing-value distinctions before requesting evidence', () => {
    expect(text()).toContain("PBS's own backup and verification records");
    expect(text()).toContain('existing authorised session');
    expect(text()).toContain(
      'datastore, namespace, guest type/ID, backup time and verification result',
    );
    expect(text()).toContain('independent PVE installations can reuse a VMID');
    expect(text()).toContain('not proof of readable backup history');
    expect(text()).toContain('report just that row');
    expect(text()).toContain('retain full inventories and credentials locally');
    expect(text()).toContain('An absent or unavailable value is not zero');
  });

  it('keeps mutation and freeze/thaw out of a display check and resolves both safety links', () => {
    expect(text()).toContain(
      'Do not restart, downgrade, recreate connections, re-enrol agents, delete history',
    );
    expect(text()).toContain('change retention or run another backup');
    expect(text()).toContain('Do not relax TLS or grant write/admin permissions');
    expect(text()).toContain('stop this display check');
    expect(text()).toContain('do not reproduce freeze/thaw');
    for (const [slug, anchor] of [
      ['RECOVERY', 'protection-posture'],
      ['VM_DISK_MONITORING', 'backup-safety'],
    ]) {
      expect(
        render(section, 'PBS').querySelector(`a[href="/docs/${slug}#${anchor}"]`),
      ).not.toBeNull();
      expect(render(read(`docs/${slug}.md`), slug).querySelector(`#${anchor}`)).not.toBeNull();
    }
  });
});
