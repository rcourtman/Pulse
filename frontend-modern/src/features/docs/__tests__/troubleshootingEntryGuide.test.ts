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

// Group shortcuts are navigation, not symptom rows. Keep the explicit full
// workflow matrix below so omitting a symptom's label or destination still fails.
const workflowRows = (section: HTMLElement): HTMLElement[] =>
  [...section.querySelectorAll<HTMLElement>('li')].filter(
    (row) => row.firstElementChild?.tagName === 'STRONG',
  );

// Name each shipped workflow and its destinations rather than accepting an
// aggregate row/link count, which cannot detect a missing or misrouted entry.
const workflowDestinations = [
  [
    'Private information was posted:',
    ['/docs/TROUBLESHOOTING#if-you-already-posted-private-information'],
  ],
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
    'SSO stopped working after editing a provider:',
    ['/docs/OIDC#sign-in-fails-after-editing-a-provider'],
  ],
  [
    'Already paid but Pro features are unavailable:',
    ['/docs/PULSE_PRO#already-paid-but-pro-features-are-unavailable'],
  ],
  [
    'Update failed or its result is uncertain:',
    ['/docs/AUTO_UPDATE#update-failed', '/docs/DOCKER#check-a-failed-or-pending-workload-update'],
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
    'Agent host is missing or reporting is rejected:',
    [
      '/docs/UNIFIED_AGENT#agent-lookup-returns-http-401-or-403',
      '/docs/UNIFIED_AGENT#agent-is-healthy-locally-but-host-reports-are-stale',
    ],
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
    'PBS or Mail Gateway State disagrees with a failed read:',
    ['/docs/TROUBLESHOOTING#pbs-or-mail-gateway-state-disagrees-with-a-failed-read'],
  ],
  [
    'PBS is Active but missing from monitoring:',
    [
      '/docs/PBS#pbs-is-active-but-missing-from-monitoring',
      '/docs/PBS#one-pbs-namespace-is-missing-or-stops-changing',
    ],
  ],
  [
    'Mail Gateway Test succeeds but collection fails:',
    [
      '/docs/MAIL_GATEWAY#troubleshooting-without-changing-the-gateway',
      '/docs/MAIL_GATEWAY#cluster-response-or-backup-lookup-errors',
    ],
  ],
  [
    'PBS datastore Usage is Unknown, Unavailable or differs from an alert:',
    ['/docs/PBS#datastore-capacity-is-unknown-or-disagrees-with-an-alert'],
  ],
  [
    'A PBS job says Healthy despite an error, warning or running status:',
    ['/docs/PBS#a-pbs-job-looks-healthy-but-its-status-disagrees'],
  ],
  [
    'Backup activity has no volume or the chart and row counts differ:',
    ['/docs/PBS#backup-activity-has-no-volume-or-fewer-rows-than-expected'],
  ],
  ['Backup rows look duplicated or copies are missing:', ['/docs/PBS#duplicate-backups']],
  [
    "Coverage's job identity, timing or latest result is unclear:",
    ['/docs/PBS#read-backup-job-identity-and-timing', '/docs/PBS#latest-backup-job-is-unknown'],
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
    'Apprise reports a TLS or connection error:',
    ['/docs/TROUBLESHOOTING#apprise-api-reports-a-tls-or-connection-error'],
  ],
  [
    'A usage alert stays open after usage drops:',
    ['/docs/TROUBLESHOOTING#a-usage-alert-stays-open-after-usage-drops'],
  ],
  ['The UI stalled after enabling Debug:', ['/docs/CONFIGURATION#if-debug-makes-the-ui-unusable']],
] as const;

describe('troubleshooting workflow entry', () => {
  it('puts guest incident safety before the symptom links and any collection recipe', () => {
    const section = entry();
    const link = section.querySelector('a');
    expect(link?.getAttribute('href')).toBe('/docs/VM_DISK_MONITORING#backup-safety');
    expect(workflowRows(section).map((row) => prose(row.querySelector('strong')!))).toEqual(
      workflowDestinations.map(([label]) => label),
    );
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
      const rows = workflowRows(section).filter((element) =>
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
      const rows = workflowRows(section).filter(
        (row) => prose(row.querySelector('strong')!) === label,
      );
      expect(rows, label).toHaveLength(1);
      expect(
        [...rows[0].querySelectorAll('a[href]')].map((link) => link.getAttribute('href')),
        label,
      ).toEqual(destinations);
    }
    const shortcuts = [...section.querySelectorAll('a[href^="#"]')];
    expect(shortcuts.map((link) => link.getAttribute('href'))).toEqual([
      '#sign-in-access-and-updates',
      '#missing-or-incorrect-readings',
      '#backups-and-storage',
      '#notifications-and-performance',
    ]);
    for (const shortcut of shortcuts) {
      const target = renderGuide('TROUBLESHOOTING').querySelector(shortcut.getAttribute('href')!);
      expect(target?.tagName).toBe('H3');
    }
    const links = [...section.querySelectorAll('a[href]')].filter(
      (link) => !link.getAttribute('href')!.startsWith('#'),
    );
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

  it('keeps service State separate from dataset and workload recovery', () => {
    const guide = renderGuide('TROUBLESHOOTING');
    const heading = guide.querySelector('#pbs-or-mail-gateway-state-disagrees-with-a-failed-read');
    expect(heading?.tagName).toBe('H4');
    const section = document.createElement('section');
    for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
      if (/^H[1-4]$/.test(next.tagName)) break;
      section.appendChild(next.cloneNode(true));
    }
    expect([...section.querySelectorAll('tbody tr')].map((row) => prose(row.firstElementChild as HTMLElement))).toEqual([
      'Healthy',
      'Degraded',
      'Unhealthy or Offline',
      'Unknown, a blank State or an unfamiliar label',
    ]);
    const text = prose(section);
    for (const phrase of [
      'A retained green State does not overrule a failed read',
      'do not restart Pulse or repeat Test',
      'a fresh sibling dataset does not clear the failed read',
      'same failed dataset resumes ordinary collection',
      'do not rotate secrets, re-add the connection or install an agent',
      'a healthy connection does not establish thaw or filesystem liveness',
    ]) expect(text).toContain(phrase);
    expect(section.querySelector('pre')).toBeNull();
    expect([...section.querySelectorAll('a')].map((link) => link.getAttribute('href'))).toEqual([
      '/docs/PBS#connection-failed-error',
      '/docs/MAIL_GATEWAY#troubleshooting-without-changing-the-gateway',
      '/docs/VM_DISK_MONITORING#backup-safety',
    ]);
    for (const name of ['PBS', 'MAIL_GATEWAY']) {
      expect(renderGuide(name).querySelector(
        'a[href="/docs/TROUBLESHOOTING#pbs-or-mail-gateway-state-disagrees-with-a-failed-read"]',
      )?.hasAttribute('data-doc-link'), name).toBe(true);
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
    expect(prose(detail)).toContain("Use PMG's own web interface");
    expect(prose(detail)).toContain('no new Test, backup, mail traffic or diagnostic run');
    expect(detail.querySelector('pre')).toBeNull();
  });

  it('routes a misleading PBS Healthy badge to same-task evidence without running new work', () => {
    const row = [...entry().querySelectorAll('li')].find((element) =>
      prose(element).startsWith(
        'A PBS job says Healthy despite an error, warning or running status:',
      ),
    );
    expect(row).toBeDefined();
    expect(prose(row!)).toContain("the same task's status evidence");
    expect(prose(row!)).toContain(
      'An OK or success word inside a longer message is not completed success',
    );
    expect(prose(row!)).toContain(
      'Use existing PBS records; do not run another job or verification to test the badge',
    );
    const heading = renderGuide('PBS').querySelector(
      '#a-pbs-job-looks-healthy-but-its-status-disagrees',
    );
    expect(heading).not.toBeNull();
    const detail = document.createElement('section');
    for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
      if (/^H[1-3]$/.test(next.tagName)) break;
      detail.appendChild(next.cloneNode(true));
    }
    expect(prose(detail)).toContain('a successful sibling job or earlier run cannot supply');
    expect(prose(detail)).toContain('A corrected badge alone is not monitoring or guest recovery');
    expect(detail.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]')).not.toBeNull();
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
