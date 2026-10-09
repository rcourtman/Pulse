import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const identityFragment = '#monitoring-is-mixed-between-proxmox-installations';

function renderGuide(name: string): HTMLElement {
  // Optional exact-base inputs exercise the former shipped advice without
  // changing the committed candidate being checked in the proof VM.
  const directory =
    process.env.PULSE_PROXMOX_IDENTITY_GUIDE_INPUT_DIRECTORY ??
    path.join(repoRoot, 'frontend-modern/public/docs');
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(
    readFileSync(path.join(directory, `${name}.md`), 'utf8'),
    name,
  );
  return article;
}

function sectionAt(article: HTMLElement, fragment: string): HTMLElement {
  const heading = article.querySelector(fragment);
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-4]$/.test(next.tagName)) break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const identitySection = (): HTMLElement =>
  sectionAt(renderGuide('TROUBLESHOOTING'), identityFragment);
const prose = (section: Element): string => (section.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('shipped cross-installation Proxmox identity guidance', () => {
  it('separates multi-installation support, cosmetic names and repair', () => {
    const article = renderGuide('CONFIGURATION');
    const section = sectionAt(article, '#multiple-proxmox-installations');
    const text = prose(section);
    expect(text).toContain('intended to monitor multiple Proxmox clusters and standalone nodes');
    expect(text).toContain('separate saved Proxmox connections');
    expect(text).toContain('matching name or VMID alone does not identify the same resource');
    expect(text).toContain('Neither that label nor a cluster member display name');
    expect(text).toContain('repairs incorrect attribution');
    expect(section.querySelector('a[href="#proxmox-cluster-node-display-names"]')).not.toBeNull();
    expect(article.querySelector('#proxmox-cluster-node-display-names')).not.toBeNull();
  });

  it('keeps all three symptoms separate with a safe comparison and a limitation for each', () => {
    const section = identitySection();
    const rows = [...section.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(3);
    expect(rows.map((row) => row.querySelector('td')?.textContent)).toEqual([
      'Node errors',
      'Guest backup status',
      'Agent-backed Docker monitoring',
    ]);
    for (const row of rows) expect(row.querySelectorAll('td')).toHaveLength(3);
    expect(prose(section)).toContain(
      'One restored view does not establish that the others recovered',
    );
    expect(prose(rows[0])).toContain("that installation's own Proxmox view");
    expect(prose(rows[1])).toContain(
      'owning installation, guest type/VMID, datastore, namespace and backup time',
    );
    expect(prose(rows[2])).toContain('Last seen and Identity evidence');
    expect(prose(rows[2])).toContain('Recent agent contact or a Healthy badge does not prove');
  });

  it('takes configuration, backup and agent help to the same real heading', () => {
    for (const name of ['CONFIGURATION', 'PBS']) {
      const link = renderGuide(name).querySelector<HTMLAnchorElement>(
        `a[href="/docs/TROUBLESHOOTING${identityFragment}"]`,
      );
      expect(link?.textContent).toBe('cross-installation identity checks');
      expect(
        renderGuide('TROUBLESHOOTING').querySelector(new URL(link!.href).hash)?.textContent,
      ).toBe('Monitoring is mixed between Proxmox installations');
    }
    const agent = sectionAt(
      renderGuide('TROUBLESHOOTING'),
      '#agent-fleet-update-or-identity-issue',
    );
    expect(agent.querySelector(`a[href="${identityFragment}"]`)).not.toBeNull();
    expect(prose(agent)).toContain('not an agent update or re-enrolment command');
  });

  it('reaches existing configuration, PBS and guest safety headings', () => {
    const section = identitySection();
    for (const [name, fragment] of [
      ['CONFIGURATION', '#multiple-proxmox-installations'],
      ['PBS', '#backups-are-visible-but-coverage-says-unprotected'],
      ['VM_DISK_MONITORING', '#backup-safety'],
    ]) {
      expect(section.querySelector(`a[href="/docs/${name}${fragment}"]`)).not.toBeNull();
      expect(renderGuide(name).querySelector(fragment)).not.toBeNull();
    }
    expect(prose(section)).toContain('An OK backup does not prove thaw');
    expect(prose(section)).toContain(
      'a frozen or unresponsive guest needs the separate backup safety procedure',
    );
  });

  it('offers independent native readings before asking for a minimal report', () => {
    const section = identitySection();
    const paragraphs = [...section.querySelectorAll('p')];
    const workaround = paragraphs.findIndex((p) =>
      prose(p).startsWith('While attribution is uncertain'),
    );
    const report = paragraphs.findIndex((p) => prose(p).startsWith('For a report'));
    expect(workaround).toBeGreaterThanOrEqual(0);
    expect(report).toBeGreaterThan(workaround);
    expect(prose(paragraphs[workaround])).toContain("Docker host's existing runtime view");
    expect(prose(paragraphs[workaround])).toContain(
      'Do not make backup, restore or workload changes',
    );
    expect(prose(paragraphs[report])).toContain('which of the three symptoms remain');
    expect(prose(paragraphs[report])).toContain('expected versus displayed origin');
  });

  it('preserves privacy while keeping equal native identities comparable across views', () => {
    const text = prose(identitySection());
    expect(text).toContain('consistent placeholders such as site-A, site-B, node-X and guest-100');
    expect(text).toContain('say explicitly when the native names or VMIDs are equal');
    expect(text).toContain(
      'Keep actual addresses, hostnames, connection IDs and machine identities private',
    );
    expect(text).toContain('Do not share a full Agent Doctor report');
    for (const withheld of ['API response', 'HAR export', 'token', 'unredacted screenshot']) {
      expect(text).toContain(withheld);
    }
  });

  it('avoids destructive recovery, live probes and incident recreation', () => {
    const section = identitySection();
    expect(section.querySelector('pre')).toBeNull();
    const text = prose(section);
    for (const boundary of [
      'existing observations only',
      'keep that working setup',
      'rather than undoing the repair or repeating the addition',
      'Missing evidence is unknown',
      'Unavailable original evidence can be reported as unavailable',
      'do not recreate the incident',
      'Do not rename production nodes, change VMIDs or machine IDs',
      'delete/re-add connections, re-enrol agents, rotate tokens, restart services or clear History',
      'Do not run live diagnostics, guest-agent probes, a new backup or a restore as an identity test',
    ]) {
      expect(text).toContain(boundary);
    }
  });
});
