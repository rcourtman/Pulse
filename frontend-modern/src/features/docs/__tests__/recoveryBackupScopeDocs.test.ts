import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', name), 'utf8');

function recoveryArticle() {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readDoc('RECOVERY.md'), 'RECOVERY');
  return article;
}

function scopeText() {
  const article = recoveryArticle();
  const heading = article.querySelector('#choose-the-right-backup');
  if (!heading) return '';
  const nodes: Element[] = [];
  for (
    let node = heading.nextElementSibling;
    node && node.tagName !== 'H2';
    node = node.nextElementSibling
  ) {
    nodes.push(node);
  }
  return nodes
    .map((node) => node.textContent)
    .join(' ')
    .replace(/\s+/g, ' ');
}

describe('Recovery backup scope guidance', () => {
  it('distinguishes four backup jobs before offering provider recovery views', () => {
    const article = recoveryArticle();
    const headings = [...article.querySelectorAll('h2')].map((heading) => heading.id);
    expect(headings[0]).toBe('choose-the-right-backup');
    expect(headings[1]).toBe('where-to-look');
    const rows = [
      ...article.querySelectorAll('table')[0].querySelectorAll<HTMLTableRowElement>('tbody tr'),
    ];
    expect(rows).toHaveLength(4);
    expect(rows.map((row) => row.cells[0].textContent)).toEqual([
      'Provider backup, snapshot or replication artifact',
      'Create Backup in Settings → System → Recovery',
      'Consistent filesystem/volume backup of Pulse',
      'Updater installation snapshot',
    ]);
    const text = scopeText();
    expect(text).toContain('history and agent enrolment state are excluded');
    expect(text).toContain('Not a backup of the workloads Pulse monitors');
    expect(text).toContain('does not prove a complete or consistent data backup');
  });

  it('makes replacement and passphrase precautions explicit, not a merge promise', () => {
    const text = scopeText();
    expect(text).toContain('not a VM, container, dataset or PVC');
    expect(text).toContain('Import replaces the included settings and API-token records');
    expect(text).toContain('it does not merge them');
    expect(text).toContain("Export the destination's existing configuration before importing");
    expect(text).toContain('retain the original passphrase privately');
  });

  it('does not turn configuration recovery into a guest-safety or backup-badge remedy', () => {
    const text = scopeText();
    expect(text).toContain('A successful task does not prove guest thaw or application recovery');
    expect(text).toContain('Do not import configuration, restore Pulse data or restart monitoring');
    expect(text).toContain('to clear a guest freeze or an incorrect backup badge');
    const whole = recoveryArticle().textContent!.replace(/\s+/g, ' ');
    expect(whole).toContain(
      'Fresh successful workload writes to every filesystem covered by the backup',
    );
    expect(whole).toContain('restore only services and timers that were active before the pause');
  });

  it('renders navigable links to the existing exact-scope and recovery procedures', () => {
    const article = recoveryArticle();
    for (const [name, anchor] of [
      ['MIGRATION', 'configuration-transfer'],
      ['MIGRATION', 'full-state-recovery'],
      ['MIGRATION', 'transfer-the-included-configuration'],
      ['AUTO_UPDATE', 'what-an-update-snapshot-contains'],
    ]) {
      expect(article.querySelector(`a[href="/docs/${name}#${anchor}"]`)).not.toBeNull();
      const target = document.createElement('article');
      target.innerHTML = renderDocMarkdown(readDoc(`${name}.md`), name);
      expect(target.querySelector(`#${anchor}`)).not.toBeNull();
    }
    expect(article.querySelector('a[href="#where-to-look"]')).not.toBeNull();
    expect(readFileSync(path.join(frontendRoot, 'public/docs/RECOVERY.md'), 'utf8')).toBe(
      readDoc('RECOVERY.md'),
    );
  });
});
