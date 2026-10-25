import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8');

function articleFor(name: string) {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readDoc(name), name);
  return article;
}

const textOf = (element: Element) => element.textContent!.replace(/\s+/g, ' ');

function handover() {
  const article = articleFor('MIGRATION');
  const heading = article.querySelector('#hand-over-monitoring-once');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && !/^H[23]$/.test(node.tagName);
    node = node.nextElementSibling
  ) {
    section.append(node.cloneNode(true));
  }
  return section;
}

describe('Migration monitoring handover safety', () => {
  it('isolates provider access before starting either kind of recovery trial', () => {
    const article = articleFor('MIGRATION');
    const step = article.querySelector(
      '#transfer-the-included-configuration + ol > li:nth-child(2)',
    );
    expect(step).not.toBeNull();
    const text = textOf(step!);
    expect(text).toContain('monitored provider APIs, agents, notification destinations');
    expect(text).toContain('remote-action targets before its first start or import');
    expect(text).toContain('QEMU Guest Agent requests through Proxmox');
    expect(text).toContain('API-only is not a passive trial');
    expect(text).toContain('full-state recovery trial too');
    expect(text).toContain("never the source's live data volume");
  });

  it('requires retained storage before a full-state backup stop', () => {
    const article = articleFor('MIGRATION');
    const text = textOf(article);
    expect(text).toContain('Before stopping Pulse, confirm that all state survives the stop');
    expect(text).toContain('emptyDir');
    expect(text).toContain('If persistence is unknown, preserve the state');
    expect(text).toContain('do not delete or scale down the server to find out');
    expect(text.indexOf('Before stopping Pulse')).toBeLessThan(
      text.indexOf('Stop Pulse before taking the consistent backup'),
    );
  });

  it('places the four-step handover before fresh destination checks', () => {
    const article = articleFor('MIGRATION');
    const headings = [...article.querySelectorAll('h3')].map((heading) => heading.id);
    expect(headings.indexOf('hand-over-monitoring-once')).toBeLessThan(
      headings.indexOf('check-the-destination'),
    );
    const section = handover();
    expect(textOf(section)).toContain('Fresh provider readings belong after the handover');
    expect(textOf(section)).toContain('turning off backup polling does not stop');
    const steps = [...section.querySelectorAll('ol > li')].map(textOf);
    expect(steps).toHaveLength(4);
    expect(steps[0]).toContain('Do not cut over during a backup, freeze/thaw');
    expect(steps[1]).toContain('every source Pulse server');
    expect(steps[1]).toContain('prevent an updater or controller from restarting it');
    expect(steps[1]).toContain('Require a confirmed normal shutdown');
    expect(steps[2]).toContain('does not cancel a guest-agent request already issued');
    expect(steps[2]).toContain('not new guest-agent probes or an arbitrary delay');
    expect(steps[2]).toContain('keep the destination isolated');
    expect(steps[3]).toContain('Only then allow the destination');
    expect(steps[3]).toContain('reconcile retained pending notifications and actions');
    expect(section.querySelector('pre')).toBeNull();
  });

  it('keeps rollback single-owner and preserves both installations', () => {
    const text = textOf(articleFor('MIGRATION'));
    expect(text).toContain(
      'Confirm the destination is stopped and cannot be automatically restarted',
    );
    expect(text).toContain("restoring the source's recorded prior-active controls");
    expect(text).toContain('do not enable a second poller');
    expect(text).toContain('do not copy its changed data over the source');
    expect(text).toContain('re-import merely to retry');
  });

  it('routes storage and incident safeguards to shipped, existing headings', () => {
    const article = articleFor('MIGRATION');
    for (const [name, fragment] of [
      ['KUBERNETES', 'proxmox-backup-safety'],
      ['VM_DISK_MONITORING', 'backup-safety'],
    ]) {
      const link = article.querySelector(`a[href="/docs/${name}#${fragment}"]`);
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(articleFor(name).querySelector(`#${fragment}`)).not.toBeNull();
    }
    expect(readFileSync(path.join(frontendRoot, 'public/docs/MIGRATION.md'), 'utf8')).toBe(
      readDoc('MIGRATION'),
    );
  });
});
