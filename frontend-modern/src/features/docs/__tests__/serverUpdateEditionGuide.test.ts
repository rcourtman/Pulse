import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const read = (file: string) => readFileSync(path.resolve(process.cwd(), '..', file), 'utf8');
const published = read('frontend-modern/public/docs/INSTALL.md');
const render = createGuideFixture(() => published);
const text = (element: Element) => element.textContent!.replace(/\s+/g, ' ').trim();

function manualUpdate() {
  const article = render('INSTALL');
  const heading = article.querySelector('#manual-update');
  expect(heading?.tagName).toBe('H4');
  const section = document.createElement('section');
  for (let node = heading!.nextElementSibling; node && !/^H[1-4]$/.test(node.tagName);) {
    section.appendChild(node.cloneNode(true));
    node = node.nextElementSibling;
  }
  return section;
}

describe('manual Pulse server update guidance', () => {
  it('puts the Community command and the signed Pro route in the same platform choice', () => {
    expect(published).toBe(read('docs/INSTALL.md'));
    const guide = manualUpdate();
    const rows = Array.from(guide.querySelectorAll<HTMLTableRowElement>('tbody tr'));
    expect(rows.map((row) => text(row.cells[0]))).toEqual([
      'Docker / Compose',
      'Kubernetes',
      'Systemd / Proxmox LXC',
    ]);
    const cell = rows[2].cells[1];
    const command = cell.querySelector('code')!;
    const download = cell.querySelector('a')!;
    expect(Array.from(cell.querySelectorAll('strong'), text)).toEqual(['Community:', 'Pro:']);
    expect(command.textContent).toBe('sudo /bin/update');
    expect(command.previousElementSibling?.textContent).toBe('Community:');
    expect(download.previousElementSibling?.textContent).toBe('Pro:');
    expect(text(cell)).toMatch(/Pulse-owned helper only.*Pro:.*matching signed Linux archive/);
    expect(download.getAttribute('href')).toBe('https://pulserelay.pro/download.html');
    expect(download.hasAttribute('data-doc-link')).toBe(false);
  });

  it('keeps a backup before the platform choices and the exact target helper Community-only', () => {
    const guide = manualUpdate();
    const table = guide.querySelector('table')!;
    const backup = guide.querySelector('a[href="/docs/MIGRATION#full-state-recovery"]')!;
    expect(backup.compareDocumentPosition(table)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    const exactTarget = Array.from(guide.querySelectorAll('p')).find((p) =>
      text(p).includes('/bin/update --version vX.Y.Z'),
    )!;
    expect(text(exactTarget)).toMatch(
      /^For an exact Community target.*only when the Pulse server installer installed that helper/,
    );
    expect(text(exactTarget)).toMatch(/owner is unknown, do not run it/);
  });

  it('keeps the private image route available in the shared guidance below the Linux choice', () => {
    const guide = manualUpdate();
    const table = guide.querySelector('table')!;
    const download = Array.from(
      guide.querySelectorAll('a[href="https://pulserelay.pro/download.html"]'),
    ).find((link) => table.compareDocumentPosition(link) === Node.DOCUMENT_POSITION_FOLLOWING)!;
    const boundary = text(download.closest('p')!);
    expect(boundary).toMatch(/Community builds, even with a matching version and valid signature/);
    expect(boundary).toMatch(
      /Keep your current Pro runtime until the matching private image or signed Linux archive is available/,
    );
    expect(boundary).toMatch(/A public build removes the private runtime hooks/);
  });
});
