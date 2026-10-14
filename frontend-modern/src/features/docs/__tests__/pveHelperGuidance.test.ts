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
