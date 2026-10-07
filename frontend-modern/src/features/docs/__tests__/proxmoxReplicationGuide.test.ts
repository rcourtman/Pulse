import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const fragment = '#replication-jobs-are-pending-stale-or-missing';

function renderGuide(name: string): HTMLElement {
  // Literal assigned-base inputs exercise missing guidance without editing
  // the candidate or implying acceptance of its runtime replication path.
  const directory =
    process.env.PULSE_REPLICATION_GUIDE_INPUT_DIRECTORY ??
    path.join(repoRoot, 'frontend-modern/public/docs');
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(
    readFileSync(path.join(directory, `${name}.md`), 'utf8'),
    name,
  );
  return article;
}

function replicationSection(): HTMLElement {
  const heading = renderGuide('TROUBLESHOOTING').querySelector(fragment);
  expect(heading?.textContent).toBe('Replication jobs are Pending, stale or missing');
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-4]$/.test(next.tagName)) break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const prose = (section: Element): string =>
  (section.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('shipped Proxmox replication disagreement guidance', () => {
  it('takes existing missing-data help to the real replication heading', () => {
    const link = renderGuide('FAQ').querySelector<HTMLAnchorElement>(
      `a[href="/docs/TROUBLESHOOTING${fragment}"]`,
    );
    expect(link?.textContent).toBe('missing or stale PVE replication jobs');
    expect(renderGuide('TROUBLESHOOTING').querySelector(new URL(link!.href).hash)).not.toBeNull();
  });

  it('distinguishes filtering, outcome classification and collection failures', () => {
    const section = replicationSection();
    const text = prose(section);
    expect(text).toContain("Start with All and clear the table's search");
    expect(text).toContain('No replication jobs match current filters');
    expect(text).toContain('Could not load replication jobs');
    const rows = [...section.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(3);
    expect(rows.map((row) => row.querySelector('td')?.textContent)).toEqual([
      'Pending, with current sync times',
      'Old sync times or an overdue next sync',
      'Empty inventory, missing tab or load error',
    ]);
    for (const row of rows) expect(row.querySelectorAll('td')).toHaveLength(3);
    expect(prose(rows[0])).toContain('Pending alone does not prove a job is waiting or failed');
    expect(prose(rows[0])).toContain('alone does not prove success');
    expect(prose(rows[1])).toContain('browser clock advancing, not evidence of a new poll');
    expect(prose(rows[2])).toContain('unknown, not an empty healthy result');
    expect(prose(rows[2])).toContain('does not establish lifecycle or polling recovery');
  });

  it('makes the matching job and narrow-layout evidence reachable without new probes', () => {
    const text = prose(replicationSection());
    expect(text).toContain('expand its row using the control beside Guest');
    expect(text).toContain('Job, Route, Last sync, Next sync, Duration and Failures');
    expect(text).toContain('even when narrow layouts hide table columns');
    expect(text).toContain('owning installation, job/guest ID and source → target route');
    expect(text).toContain('Similar names or VMIDs in separate installations');
    expect(text).toContain('existing native Proxmox replication view');
    expect(text).toContain('do not force a sync to obtain one');
  });

  it('preserves separate symptoms, original recovery and a minimal private report', () => {
    const text = prose(replicationSection());
    expect(text).toContain('classification and stale or absent inventory after a lifecycle change');
    expect(text).toContain('keep that observation and the now-working setup');
    expect(text).toContain('One current row does not prove every job recovered');
    expect(text).toContain('which symptoms remain');
    expect(text).toContain('Pulse versus native status, sync times, failures and redacted error');
    expect(text).toContain('without recreating missing original evidence');
    expect(text).toContain('consistent placeholders');
    for (const withheld of ['tokens', 'full API responses', 'configuration', 'HAR exports']) {
      expect(text).toContain(withheld);
    }
  });

  it('keeps replication separate from backups, restore readiness and guest safety', () => {
    const section = replicationSection();
    const text = prose(section);
    expect(text).toContain('storage replication between PVE nodes, not PBS backups');
    expect(text).toContain('does not establish backup coverage, restore readiness or guest thaw');
    const link = section.querySelector<HTMLAnchorElement>(
      'a[href="/docs/VM_DISK_MONITORING#backup-safety"]',
    );
    expect(link?.textContent).toBe('backup safety procedure');
    expect(
      renderGuide('VM_DISK_MONITORING').querySelector(new URL(link!.href).hash),
    ).not.toBeNull();
  });

  it('rejects disruptive diagnostics, broad permissions and live reenactment', () => {
    const section = replicationSection();
    expect(section.querySelector('pre')).toBeNull();
    const text = prose(section);
    for (const boundary of [
      'do not repeat a restart, reboot or update to reproduce it',
      'Do not restart or upgrade Pulse, reboot nodes, change schedules, disable/re-enable jobs',
      'recreate connections, widen token permissions or disable privilege separation',
      'Do not run a new sync, backup, restore, live diagnostics or guest-agent probe as a test',
    ]) {
      expect(text).toContain(boundary);
    }
  });
});
