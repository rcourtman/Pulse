import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shippedDocs = path.join(repoRoot, 'frontend-modern/public/docs');
const source = readFileSync(path.join(shippedDocs, 'TROUBLESHOOTING.md'), 'utf8');
const renderedGuides = new Map<string, HTMLElement>();

function renderGuide(name: string): HTMLElement {
  // Several destinations share a long guide. Render each immutable document
  // once, rather than making navigation checks parse it repeatedly.
  // entry() clones its section, so the cached article is never modified.
  const cached = renderedGuides.get(name);
  if (cached) return cached;
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(
    stripLeadingTitle(readFileSync(path.join(shippedDocs, `${name}.md`), 'utf8')),
    name,
  );
  renderedGuides.set(name, article);
  return article;
}

function entry(): HTMLElement {
  const article = renderGuide('TROUBLESHOOTING');
  const heading = article.querySelector('h2');
  expect(heading?.id).toBe('choose-the-affected-workflow');
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (next.tagName === 'H2') break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const prose = (element: HTMLElement): string =>
  (element.textContent ?? '').replace(/\s+/g, ' ').trim();

// Name each shipped workflow and its destinations rather than accepting an
// aggregate row/link count, which cannot detect a missing or misrouted entry.
const workflowDestinations = [
  [
    'Not sure which version is affected:',
    ['/docs/TROUBLESHOOTING#find-the-running-server-and-agent-versions'],
  ],
  [
    'Cannot sign in:',
    [
      '/docs/TROUBLESHOOTING#i-forgot-my-password',
      '/docs/TROUBLESHOOTING#cannot-login--401-unauthorized',
    ],
  ],
  [
    'Update failed or its result is uncertain:',
    ['/docs/AUTO_UPDATE#update-failed', '/docs/DOCKER#check-a-failed-or-pending-workload-update'],
  ],
  [
    'Agent host is missing or reporting is rejected:',
    ['/docs/UNIFIED_AGENT#agent-lookup-returns-http-401-or-403'],
  ],
  [
    'Proxmox or PBS reports a certificate error:',
    ['/docs/TROUBLESHOOTING#proxmox-or-pbs-certificate-validation-failed'],
  ],
  [
    'Copied agent install or removal command failed:',
    ['/docs/UNIFIED_AGENT#copied-agent-command-fails-before-installation'],
  ],
  [
    'Guest readings are missing or disagree:',
    [
      '/docs/VM_DISK_MONITORING#current-retained-and-unavailable-disk-readings',
      '/docs/TROUBLESHOOTING#guest-memory-is-high-but-available-memory-is-plentiful',
    ],
  ],
  [
    'Proxmox data belongs to the wrong host, or healthy jobs look Pending:',
    [
      '/docs/TROUBLESHOOTING#monitoring-is-mixed-between-proxmox-installations',
      '/docs/TROUBLESHOOTING#replication-jobs-are-pending-stale-or-missing',
    ],
  ],
  ['TrueNAS readings are stale or absent:', ['/docs/TRUENAS#stale-truenas-data']],
  [
    'PBS is Active but missing from monitoring:',
    ['/docs/PBS#pbs-is-active-but-missing-from-monitoring'],
  ],
  [
    'Mail Gateway Test succeeds but collection fails:',
    [
      '/docs/MAIL_GATEWAY#troubleshooting-without-changing-the-gateway',
      '/docs/MAIL_GATEWAY#cluster-response-or-backup-lookup-errors',
    ],
  ],
  [
    'RAID is reported degraded despite a spare:',
    ['/docs/TROUBLESHOOTING#raid-array-is-degraded-with-a-spare'],
  ],
  [
    'Unraid reports missing disks in empty slots:',
    ['/docs/TROUBLESHOOTING#unraid-reports-missing-disks-in-empty-slots'],
  ],
  [
    'Pulse itself uses more memory, CPU or disk writes:',
    [
      '/docs/TROUBLESHOOTING#memory-use-keeps-growing',
      '/docs/TROUBLESHOOTING#excessive-cpu-writes-or-database-growth',
    ],
  ],
  [
    'Notification Test works but alerts do not arrive:',
    ['/docs/TROUBLESHOOTING#test-succeeds-but-real-alerts-are-missing'],
  ],
  [
    'Private information was posted:',
    ['/docs/TROUBLESHOOTING#if-you-already-posted-private-information'],
  ],
] as const;

describe('troubleshooting workflow entry', () => {
  it('puts guest incident safety before the symptom links and any collection recipe', () => {
    const section = entry();
    const link = section.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/docs/VM_DISK_MONITORING#backup-safety');
    expect(
      [...section.querySelectorAll('li')].map((row) => prose(row.querySelector('strong')!)),
    ).toEqual(workflowDestinations.map(([label]) => label));
    expect(section.querySelector('pre')).toBeNull();
    const beforeList = [...section.children]
      .slice(
        0,
        [...section.children].findIndex((element) => element.tagName === 'UL'),
      )
      .map((element) => prose(element as HTMLElement))
      .join(' ');
    expect(beforeList).toContain("an earlier guest request's state is uncertain");
    expect(beforeList).toContain(
      'Do not run Diagnostics, force guest-agent probes or repeat a backup',
    );
    expect(beforeList).toContain('responsive console alone does not establish safe recovery');
  });

  it('keeps server updates, workload updates and reporting acceptance distinct', () => {
    const section = entry();
    const row = (label: string) => {
      const rows = [...section.querySelectorAll('li')].filter((element) =>
        prose(element.querySelector('strong') as HTMLElement).startsWith(label),
      );
      expect(rows, label).toHaveLength(1);
      return rows[0] as HTMLElement;
    };
    const update = row('Update failed or its result is uncertain');
    const install = row('Agent host is missing or reporting is rejected');
    const command = row('Copied agent install or removal command failed');
    const reporting = row('Guest readings are missing or disagree');
    const memory = row('Pulse itself uses more memory, CPU or disk writes');
    const raid = row('RAID is reported degraded despite a spare');
    expect(raid.querySelector('a')?.getAttribute('href')).toBe(
      '/docs/TROUBLESHOOTING#raid-array-is-degraded-with-a-spare',
    );
    expect(prose(raid)).toContain('A spare-inclusive total is not the required member count');
    expect(prose(raid)).toContain('do not change the array just to clear a Pulse alert');
    expect(prose(update)).toContain('An update of a monitored container is a different job');
    const updateLinks = [...update.querySelectorAll('a')].map((link) => link.getAttribute('href'));
    expect(updateLinks).toEqual([
      '/docs/AUTO_UPDATE#update-failed',
      '/docs/DOCKER#check-a-failed-or-pending-workload-update',
    ]);
    expect(prose(update)).toContain('Preserve the current state and rollback evidence');
    expect(prose(install)).toContain('Installer success is not an accepted report');
    expect(prose(command)).toContain('A stopped new command does not diagnose the existing agent');
    expect(prose(command)).toContain('do not repeat removal or installation');
    expect(command.querySelector('a')?.getAttribute('href')).toBe(
      '/docs/UNIFIED_AGENT#copied-agent-command-fails-before-installation',
    );
    expect(prose(reporting)).toContain('a working Pulse Agent view does not prove API recovery');
    expect(prose(memory)).toContain('Guest usage and database size are different measurements');
  });

  it('routes every entry to an existing shipped heading in the production viewer', () => {
    const section = entry();
    for (const [label, destinations] of workflowDestinations) {
      const rows = [...section.querySelectorAll('li')].filter(
        (row) => prose(row.querySelector('strong')!) === label,
      );
      expect(rows, label).toHaveLength(1);
      expect(
        [...rows[0].querySelectorAll('a[href]')].map((link) => link.getAttribute('href')),
        label,
      ).toEqual(destinations);
    }
    const links = [...section.querySelectorAll('a[href]')];
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/docs/VM_DISK_MONITORING#backup-safety',
      ...workflowDestinations.flatMap(([, destinations]) => [...destinations]),
      '/docs/TROUBLESHOOTING#-getting-help',
    ]);
    for (const link of links) {
      const href = link.getAttribute('href')!;
      const match = /^\/docs\/([^#]+)#(.+)$/.exec(href);
      expect(match, href).not.toBeNull();
      const target = renderGuide(match![1]).querySelector(`[id="${match![2]}"]`);
      expect(target, href).not.toBeNull();
      expect(target?.tagName, href).toMatch(/^H[1-6]$/);
      expect(link.hasAttribute('data-doc-link'), href).toBe(true);
      expect(link.hasAttribute('target'), href).toBe(false);
    }
  });

  it('routes PMG collection failures to safe existing guidance, not agent or SMTP setup', () => {
    const row = [...entry().querySelectorAll('li')].find((element) =>
      prose(element).startsWith('Mail Gateway Test succeeds but collection fails:'),
    );
    expect(row).toBeDefined();
    expect(prose(row!)).toContain('before changing credentials, DNS or TLS');
    expect(prose(row!)).toContain('Missing queue data is not an empty queue');
    expect(row!.querySelectorAll('a')).toHaveLength(2);
    const guide = renderGuide('MAIL_GATEWAY');
    const errorHeading = guide.querySelector('#cluster-response-or-backup-lookup-errors');
    expect(errorHeading).not.toBeNull();
    const detail = document.createElement('section');
    for (let next = errorHeading!.nextElementSibling; next; next = next.nextElementSibling) {
      if (/^H[1-3]$/.test(next.tagName)) break;
      detail.appendChild(next.cloneNode(true));
    }
    expect(prose(detail)).toContain('Use PMG\'s own web interface');
    expect(prose(detail)).toContain('no new Test, backup, mail traffic or diagnostic run');
    expect(detail.querySelector('pre')).toBeNull();
  });

  it('keeps the source and shipped entry identical and preserves existing anchors', () => {
    const canonical = readFileSync(path.join(repoRoot, 'docs/TROUBLESHOOTING.md'), 'utf8');
    expect(source).toBe(canonical);
    const article = renderGuide('TROUBLESHOOTING');
    for (const id of [
      'i-forgot-my-password',
      'memory-use-keeps-growing',
      'collect-diagnostics-safely',
      'recovery-mode',
      '-getting-help',
    ]) {
      expect(article.querySelector(`[id="${id}"]`), id).not.toBeNull();
    }
    expect(prose(entry())).toContain('unknown or unavailable details are valid');
    expect(prose(entry())).toContain('Deleting the public copy does not revoke a credential');
    expect(prose(entry())).toContain('Do not trigger an alert or replay the queue');
  });
});
