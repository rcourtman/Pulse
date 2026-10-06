import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (relative: string): string => readFileSync(path.join(repoRoot, relative), 'utf8');
const guide = read('docs/RECOVERY.md');
const safetyText = () => {
  const article = document.createElement('article');
  const section = guide
    .split("**A backup task's OK status")[1]
    .split('Before relying on an artifact')[0];
  article.innerHTML = renderDocMarkdown(`**A backup task's OK status${section}`, 'RECOVERY');
  return article.textContent?.replace(/\s+/g, ' ');
};

describe('recovery help', () => {
  it('separates backup evidence from thaw and tested recovery in the shipped guide', () => {
    expect(read('frontend-modern/public/docs/RECOVERY.md')).toBe(guide);
    expect(guide).toContain('Pulse does not restore workloads from these views.');
    expect(guide).toContain('OK status is not confirmation that the guest has thawed.');
    expect(safetyText()).toContain('Pulse monitoring and alert delivery are unavailable');
    expect(guide).toContain('restore to an isolated destination');
    expect(guide).toContain('Do not overwrite the live\nworkload');
    expect(guide).toContain('an omitted value supplies no verification result');
    expect(guide).toContain('An unknown outcome is not success');
    expect(guide).not.toContain('What can I actually recover?');
  });

  it('keeps the planned pause distinct from incident recovery and in-flight completion', () => {
    expect(safetyText()).toContain('For an affected installation');
    expect(safetyText()).toContain('before a planned freeze-enabled Proxmox backup');
    expect(safetyText()).toContain(
      'actual server deployment and automatic updaters, not just a guest agent',
    );
    expect(safetyText()).toContain('A planned pause is not an incident recovery procedure');
    expect(safetyText()).toContain(
      'stopping Pulse does not cancel a guest-agent request already issued',
    );
    expect(safetyText()).toContain(
      'do not start a backup on the strength of a stopped service or an elapsed waiting period',
    );
  });

  it('requires all three independent post-backup checks, including writes to every covered filesystem', () => {
    const article = document.createElement('article');
    article.innerHTML = renderDocMarkdown(guide, 'RECOVERY');
    const checks = [...article.querySelector('ol')!.querySelectorAll('li')].map(
      (item) => item.textContent,
    );
    expect(checks).toEqual([
      'Guest thaw.',
      'Fresh successful workload writes to every filesystem covered by the backup.',
      'Workload liveness.',
    ]);
    expect(safetyText()).toContain(
      'until the backup has ended and independent post-backup checks confirm all three',
    );
    expect(safetyText()).toContain(
      'established safe checks, independent of Pulse and the QEMU Guest Agent',
    );
    expect(safetyText()).toContain(
      'a console connection, a successful read or a write to only the OS disk is not enough',
    );
  });

  it('leaves failed or unavailable recovery checks paused without inducing unsafe proof', () => {
    expect(safetyText()).toContain(
      'If any check fails or is unavailable, leave Pulse and its automatic updater paused',
    );
    expect(safetyText()).toContain("use the guest/platform's recovery procedure");
    expect(safetyText()).toContain(
      'Do not force writes, repeat a backup or send guest-agent probes to fill the gap',
    );
    expect(safetyText()).toContain('Do not disable filesystem freezing');
    const article = document.createElement('article');
    article.innerHTML = renderDocMarkdown(
      guide
        .split('If the guest stopped responding during backup')[1]
        .split('When reporting missing records')[0],
      'RECOVERY',
    );
    expect(article.textContent?.replace(/\s+/g, ' ')).toContain(
      'a console connection alone does not clear the precaution',
    );
  });

  it('restores only prior-active services and timers and preserves outage coverage', () => {
    expect(safetyText()).toContain(
      'After all checks pass, restore only services and timers that were active before the pause',
    );
    expect(safetyText()).toContain('Unknown pre-pause states are not permission to start them');
    expect(safetyText()).toContain('An updater must not restart a previously inactive server');
    expect(safetyText()).toContain('deployment-specific restoration steps');
    expect(safetyText()).toContain('arrange independent outage coverage');
    expect(safetyText()).toContain('not proof of a repaired or reproduced native thaw failure');
  });

  it('points to existing platform views and a usable backup-safety heading', () => {
    expect(guide).toContain('There is no top-level Recovery page.');
    expect(guide).toContain('**Proxmox → Backups** (`/proxmox/backups`)');
    expect(guide).toContain('**TrueNAS → Protection** (`/truenas/protection`)');
    const proxmox = read('frontend-modern/src/features/proxmox/ProxmoxBackupsTable.tsx');
    expect(proxmox).toContain("label: 'By date'");
    expect(proxmox).toContain("label: 'Coverage'");
    expect(read('frontend-modern/src/features/truenas/truenasPageModel.ts')).toContain(
      "path: '/truenas/protection'",
    );
    const rendered = document.createElement('article');
    rendered.innerHTML = renderDocMarkdown(guide, 'RECOVERY');
    const safety = rendered.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]');
    expect(safety?.textContent).toBe('backup safety precaution');
    const target = document.createElement('article');
    target.innerHTML = renderDocMarkdown(read('docs/VM_DISK_MONITORING.md'), 'VM_DISK_MONITORING');
    expect(target.querySelector('#backup-safety')?.textContent).toBe('Backup safety');
  });

  it('resolves safe issue reporting to the actual troubleshooting heading', () => {
    const rendered = document.createElement('article');
    rendered.innerHTML = renderDocMarkdown(guide, 'RECOVERY');
    const link = rendered.querySelector<HTMLAnchorElement>('a[href^="/docs/TROUBLESHOOTING#"]');
    expect(link?.textContent).toBe('safe issue reporting');
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    const destination = new URL(link!.getAttribute('href')!, 'https://pulse.example.invalid');
    expect(destination.pathname).toBe('/docs/TROUBLESHOOTING');
    expect(destination.hash).not.toBe('');

    const target = document.createElement('article');
    target.innerHTML = renderDocMarkdown(read('docs/TROUBLESHOOTING.md'), 'TROUBLESHOOTING');
    const heading = [...target.querySelectorAll('h2')].find(
      (element) => element.id === decodeURIComponent(destination.hash.slice(1)),
    );
    expect(heading?.textContent).toBe('🆘 Getting Help');
  });

  it('documents actual point filters and an example the handlers consume', () => {
    const handlers = read('internal/api/recovery_handlers.go');
    const model = read('internal/recovery/model/types.go');
    const section = guide.split('### Point and rollup filters')[1].split('### Posture lookup')[0];
    const table = section.split('For example,')[0];
    const parameters = [...table.matchAll(/^\| `([^`]+)` \|/gm)].map((match) => match[1]);
    expect(parameters).toEqual([
      'platform',
      'kind',
      'mode',
      'outcome',
      'from',
      'to',
      'subjectResourceId',
      'rollupId',
      'q',
    ]);
    for (const parameter of parameters) {
      expect(handlers).toContain(`qs.Get("${parameter}")`);
    }
    const example = section.match(/```text\n([^\n]+)\n```/)?.[1];
    expect(example).toBeDefined();
    const request = new URL(example!, 'https://pulse.example.invalid');
    expect(request.pathname).toBe('/api/recovery/points');
    expect([...request.searchParams.keys()]).toEqual([
      'platform',
      'kind',
      'from',
      'to',
      'page',
      'limit',
    ]);
    expect(request.searchParams.get('platform')).toBe('proxmox-pbs');
    expect(model).toContain('ProviderProxmoxPBS Provider = "proxmox-pbs"');
    expect(model).toContain('DefaultListPageLimit = 100');
    expect(model).toContain('MaxListPageLimit     = 500');
    expect(section).toContain('default **100**');
    expect(section).toContain('maximum **500**');
    expect(section).toContain('Unrecognised query names do not apply those filters');
    expect(request.searchParams.has('token')).toBe(false);
  });

  it('keeps read-only endpoints and posture batch lookup distinct from point filters', () => {
    const routes = read('internal/api/router_routes_monitoring.go');
    const handlers = read('internal/api/recovery_handlers.go');
    const endpoints = [...guide.matchAll(/^\| `GET` \| `([^`]+)` \|/gm)].map((match) => match[1]);
    expect(endpoints).toEqual([
      '/api/recovery/points',
      '/api/recovery/rollups',
      '/api/recovery/series',
      '/api/recovery/facets',
      '/api/recovery/postures',
    ]);
    for (const endpoint of endpoints) {
      expect(routes).toContain(`HandleFunc("${endpoint}", RequireAuth`);
    }
    expect(guide).toContain('`monitoring:read` token supplied through a private header file');
    const posture = guide.split('### Posture lookup')[1].split('## See also')[0];
    expect(posture).toContain('Repeat `resourceId`');
    expect(posture).toContain('at most **200**');
    expect(handlers).toContain('const maxProtectionPostureResourceIDs = 200');
    expect(posture).toContain('Do not\napply point filters to this endpoint');
  });
});
