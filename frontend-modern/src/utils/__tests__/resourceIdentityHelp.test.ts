import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const api = read('docs/API.md');
const section =
  api.split('#### Resource IDs and integration labels\n')[1]?.split('\n#### ')[0] ?? '';
const article = () => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(section, 'API');
  return element;
};
const text = () => article().textContent?.replace(/\s+/g, ' ');

describe('resource identity integration help', () => {
  it('preserves returned IDs rather than applying one guest separator to every resource', () => {
    expect(read('frontend-modern/public/docs/API.md')).toBe(api);
    expect(text()).toContain('Treat each returned id as an opaque value');
    expect(text()).toContain('do not split it on -, : or /, replace separators');
    expect(text()).toContain('resource collection and its returned id together as the key');
    expect(text()).toContain('not interchangeable with IDs returned by Unified Resources');
    expect(text()).toContain('encode that whole ID as one path segment');
    const link = article().querySelector('a[href="#unified-resources"]');
    expect(link?.textContent).toBe('Unified Resources');
    expect(link?.hasAttribute('data-doc-link')).toBe(true);
    const complete = document.createElement('article');
    complete.innerHTML = renderDocMarkdown(api, 'API');
    expect(complete.querySelector('#unified-resources')).not.toBeNull();
  });

  it('renders context fields actually supplied by the legacy models without promising a unique name', () => {
    const rows = [...article().querySelectorAll('tbody tr')].map((row) =>
      [...row.querySelectorAll('td')].map((cell) => cell.textContent?.trim()),
    );
    expect(rows.map((row) => row[0])).toEqual(['nodes', 'vms, containers', 'storage']);
    const models = read('internal/models/models.go');
    for (const [model, fields] of [
      ['Node', ['name', 'displayName', 'instance']],
      ['VM', ['name', 'instance', 'node', 'vmid']],
      ['Container', ['name', 'instance', 'node', 'vmid']],
      ['Storage', ['name', 'instance', 'node', 'nodes']],
    ] as const) {
      const body = models.split(`type ${model} struct {`)[1]?.split('\n}')[0] ?? '';
      for (const field of fields) expect(body).toMatch(new RegExp(`json:"${field}(?:,|")`));
    }
    expect(text()).toContain('not necessarily a single monitored host');
    expect(text()).toContain('A repeated name or VMID is not sufficient to link resources');
    expect(text()).toContain('not a substitute for an explicit relationship');
  });

  it('does not turn delimiter differences into destructive diagnostic instructions', () => {
    expect(text()).toContain(
      'Do not assume an ID survives a connection being removed and recreated',
    );
    expect(text()).toContain(
      'Do not restart Pulse, recreate connections or edit/remove guest_metadata.json',
    );
    expect(text()).toContain('without repeating the connection change');
    expect(text()).toContain('Keep state exports and private names out of public reports');
    expect(article().querySelector('pre')).toBeNull();
  });
});
