import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const anchor = 'proxmox-lxc-filesystem-readings-with-the-typed-helper';
const article = (name: string): HTMLElement => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(
    readFileSync(path.join(root, 'frontend-modern/public/docs', `${name}.md`), 'utf8'),
    name,
  );
  return element;
};

describe('confined PVE helper reading guidance', () => {
  it('links the deployment choice and support matrix to the same shipped section', () => {
    const guide = article('AGENT_SECURITY');
    expect(guide.querySelector(`#${anchor}`)?.textContent).toBe(
      'Proxmox LXC filesystem readings with the typed helper',
    );
    expect(guide.querySelectorAll(`a[href="#${anchor}"]`).length).toBeGreaterThanOrEqual(3);
    const row = [...guide.querySelectorAll('tr')].find((element) =>
      element.textContent?.includes('Proxmox node-local LXC filesystem telemetry'),
    )!;
    expect(row.textContent).toContain('command fallback works only outside the helper');
    expect(row.querySelector(`a[href="#${anchor}"]`)).not.toBeNull();
  });

  it('keeps the confined-helper distinction reachable from Unified Agent', () => {
    const guide = article('UNIFIED_AGENT');
    const link = guide.querySelector(`a[href="/docs/AGENT_SECURITY#${anchor}"]`);
    expect(link?.textContent).toBe('helper-backed LXC readings');
    expect(link?.closest('p')?.textContent?.replace(/\s+/g, ' ')).toContain(
      'those commands cannot recover readings inside the isolated typed helper',
    );
  });
});

describe('least-privilege PVE guest reading choices', () => {
  const choice = (label: string): HTMLTableRowElement => {
    const row = [...article('AGENT_SECURITY').querySelectorAll('tbody tr')].find(
      (element) => element.querySelector('td')?.textContent === label,
    );
    expect(row, `separate deployment choice for ${label}`).toBeDefined();
    return row as HTMLTableRowElement;
  };

  it('does not prescribe optional Linux file access for filesystem auditing', () => {
    const row = choice('VM filesystem usage and guest information through QEMU Guest Agent');
    const permissions = row.querySelectorAll('td')[1];
    expect([...permissions.querySelectorAll('code')].map((element) => element.textContent)).toEqual([
      'VM.GuestAgent.Audit',
      'VM.Monitor',
    ]);
    expect(permissions.textContent).toContain('PVE 9+');
    expect(permissions.textContent).toContain('PVE 8 uses the legacy');
    expect(permissions.textContent).toContain('intended guests');
    expect(row.querySelectorAll('td')[2].textContent).toBe('No host agent for the Proxmox node');
  });

  it('keeps Linux memory-file access a separate, optional choice', () => {
    const row = choice(
      'Linux guest-memory detail by reading /proc/meminfo through QEMU Guest Agent',
    );
    const permissions = row.querySelectorAll('td')[1];
    expect([...permissions.querySelectorAll('code')].map((element) => element.textContent)).toEqual([
      'VM.GuestAgent.FileRead',
      'VM.Monitor',
    ]);
    expect(permissions.textContent).toContain('On PVE 9+, this separate read also needs');
    expect(permissions.textContent).toContain('PVE 8 uses');
    expect(permissions.textContent).toContain('Leave file reads denied if they are not intended');
  });

  it('routes denials to existing scoped guidance, not another setup or probe', () => {
    const guide = article('AGENT_SECURITY');
    const section = guide.textContent?.split('Filesystem auditing does not require')[1]?.split(
      'Inside-guest runtime visibility',
    )[0]?.replace(/\s+/g, ' ');
    expect(section).toContain('Windows has no /proc/meminfo');
    expect(section).toContain('not evidence that Windows disk collection needs VM.GuestAgent.FileRead');
    expect(section).toContain('does not by itself prove fresh readings, workload health or backup safety');
    expect(section).toContain('keeping privilege separation enabled');
    expect(section).toContain('not a reason to widen access, install another agent or rerun setup');
    expect(section).toContain('existing observations instead of manual guest-agent probes');
    const trouble = article('TROUBLESHOOTING');
    for (const fragment of [
      'check-permissions-proxmox',
      'optional-proxmox-checks-return-403-repeatedly',
    ]) {
      expect(guide.querySelector(`a[href="/docs/TROUBLESHOOTING#${fragment}"]`)).not.toBeNull();
      expect(trouble.querySelector(`#${fragment}`)).not.toBeNull();
    }
    expect(guide.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]')).not.toBeNull();
  });
});
