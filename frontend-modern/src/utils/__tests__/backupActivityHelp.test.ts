import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';
import { buildBackupActivityTimeline } from '@/features/proxmox/proxmoxBackupActivityPresentation';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const fragment = 'backup-activity-has-no-volume-or-fewer-rows-than-expected';
const render = (slug: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(read(`docs/${slug}.md`), slug);
  return article;
};
const section = () => {
  const heading = render('PBS').querySelector(`#${fragment}`)!;
  const content = document.createElement('section');
  for (
    let next = heading.nextElementSibling;
    next && !/^H[1-3]$/.test(next.tagName);
    next = next.nextElementSibling
  ) {
    content.appendChild(next.cloneNode(true));
  }
  return content;
};

describe('backup activity evidence help', () => {
  it('ships the same guide and routes the troubleshooting entry to its unique rendered heading', () => {
    for (const slug of ['PBS', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${slug}.md`)).toBe(read(`docs/${slug}.md`));
    }
    expect(render('PBS').querySelectorAll(`#${fragment}`)).toHaveLength(1);
    const entry = render('TROUBLESHOOTING').querySelector(`a[href="/docs/PBS#${fragment}"]`);
    expect(entry?.textContent).toBe('Count, range and size-evidence checks');
    expect(entry?.hasAttribute('data-doc-link')).toBe(true);
    expect(section().querySelectorAll('tbody tr')).toHaveLength(3);
  });

  it('describes the actual Count/Volume difference without converting missing sizes to measured zero', () => {
    const now = new Date(2026, 9, 19, 12);
    const items = [
      { time: new Date(2026, 9, 19, 10).getTime(), size: 1024 },
      { time: new Date(2026, 9, 19, 11).getTime(), size: 0 },
      { time: new Date(2026, 9, 20, 10).getTime(), size: 4096 },
      { time: new Date(2026, 7, 1, 10).getTime(), size: 2048 },
    ];
    const count = buildBackupActivityTimeline(
      7,
      items,
      (item) => item.time,
      () => 'pbs',
      { now },
    );
    const volume = buildBackupActivityTimeline(
      7,
      items,
      (item) => item.time,
      () => 'pbs',
      {
        now,
        getValue: (item) => item.size,
      },
    );
    expect(count.points.at(-1)?.total).toBe(2);
    expect(volume.points.at(-1)?.total).toBe(1024);
    expect(volume.points.at(-1)?.unmeasuredCount).toBe(1);
    expect(volume.points.reduce((sum, point) => sum + point.total, 0)).toBe(1024);
    const words = section().textContent?.replace(/\s+/g, ' ');
    expect(words).toContain(
      'A point without a usable size still counts; it is not a zero-byte backup.',
    );
    expect(words).toContain('Treat the total as partial');
    expect(words).toContain('not a count of successful backup jobs or distinct protected guests');
  });

  it('retains scope, capacity and safety limits with working links instead of new diagnostic commands', () => {
    const content = section();
    const words = content.textContent?.replace(/\s+/g, ' ');
    for (const phrase of [
      'The activity chart and table use the same search, node, type, source',
      'Clear date filter',
      "browser's local timezone",
      'not datastore space used, free space or bytes transferred',
      'not proof of protection, verification, restore readiness or guest thaw',
      'Do not run a backup, verification, restore, live diagnostics or guest-agent probe',
      'change retention, delete records or restart anything',
      'Keep private identifiers and paths out of reports',
    ])
      expect(words).toContain(phrase);
    for (const [href, slug, id] of [
      [
        '#backups-are-visible-but-coverage-says-unprotected',
        'PBS',
        'backups-are-visible-but-coverage-says-unprotected',
      ],
      ['/docs/VM_DISK_MONITORING#backup-safety', 'VM_DISK_MONITORING', 'backup-safety'],
    ]) {
      expect(content.querySelector(`a[href="${href}"]`)).not.toBeNull();
      expect(render(slug).querySelector(`#${id}`)).not.toBeNull();
    }
    expect(content.querySelector('pre')).toBeNull();
  });
});
