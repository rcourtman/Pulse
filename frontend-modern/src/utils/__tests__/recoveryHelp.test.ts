import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (relative: string): string => readFileSync(path.join(repoRoot, relative), 'utf8');
const guide = read('docs/RECOVERY.md');

describe('recovery help', () => {
  it('separates backup evidence from thaw and tested recovery in the shipped guide', () => {
    expect(read('frontend-modern/public/docs/RECOVERY.md')).toBe(guide);
    expect(guide).toContain('Pulse does not restore workloads from these views.');
    expect(guide).toContain('OK status is not confirmation that the guest has thawed.');
    expect(guide).toContain('only after independently confirming guest thaw');
    expect(guide).toContain('Pulse monitoring and alert\ndelivery are unavailable');
    expect(guide).toContain('restore to an isolated destination');
    expect(guide).toContain('Do not overwrite the live\nworkload');
    expect(guide).toContain('an omitted value supplies no verification result');
    expect(guide).toContain('An unknown outcome is not success');
    expect(guide).not.toContain('What can I actually recover?');
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
