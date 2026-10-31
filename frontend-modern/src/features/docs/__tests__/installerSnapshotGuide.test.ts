import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const read = (file: string) => readFileSync(path.resolve(process.cwd(), '..', file), 'utf8');
const article = (name: string) => {
  const result = document.createElement('article');
  result.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return result;
};
const section = () => {
  const heading = article('AUTO_UPDATE').querySelector('#installer-configuration-snapshots');
  expect(heading).not.toBeNull();
  const result = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && node.tagName !== 'H3' && node.tagName !== 'H2';
    node = node.nextElementSibling
  ) {
    result.appendChild(node.cloneNode(true));
  }
  return result;
};

describe('installer configuration snapshot guidance', () => {
  it('keeps the two default locations literal and distinct from in-app snapshots', () => {
    const rendered = section();
    const copy = rendered.textContent!.replace(/\s+/g, ' ');
    const rows = [...rendered.querySelectorAll('tbody tr')];
    expect(rows.map((row) => row.querySelectorAll('td')[1].textContent)).toEqual([
      '/etc/pulse.backup.<timestamp>…/',
      '/opt/pulse/config-backups/pulse.backup.<timestamp>…/',
    ]);
    const installer = read('install.sh');
    expect(installer).toContain('backup_parent="$(dirname "$CONFIG_DIR")"');
    expect(installer).toContain('backup_parent="$INSTALL_DIR/config-backups"');
    expect(installer).toContain('local keep="${CONFIG_BACKUP_KEEP_COUNT:-5}"');
    expect(installer).toContain('cp -a -- "$CONFIG_DIR/."');
    expect(copy).toContain('normally retains five completed copies');
    expect(copy).toContain('Custom service names');
    expect(copy).toContain('Update History does not list or restore these installer copies');
    expect(copy).toContain('An in-app backup_path is not the location of an installer copy');
  });

  it('does not turn a rotating file copy into complete, consistent or automatic recovery', () => {
    const rendered = section();
    const copy = rendered.textContent!.replace(/\s+/g, ' ');
    for (const distinction of [
      'not a full-state backup',
      'not the contents of targets outside the copied tree',
      'not automatically included',
      'server executable, service units or update helpers stored elsewhere',
      'Pulse may still be writing',
      'not a durable archive',
      'does not prove that the update completed or that data was lost',
      'Do not rerun an update or reinstall',
      'Keep the copies private',
      'do not delete another instance',
      'reset keys to make recovery fit',
    ]) {
      expect(copy).toContain(distinction);
    }
    expect(rendered.querySelector('pre')).toBeNull();
  });

  it('reaches real shipped recovery sections from the existing installation guide', () => {
    expect(
      article('INSTALL').querySelector(
        'a[href="/docs/AUTO_UPDATE#installer-configuration-snapshots"]',
      ),
    ).not.toBeNull();
    for (const [name, fragment] of [
      ['AUTO_UPDATE', 'manual-rollback'],
      ['MIGRATION', 'full-state-recovery'],
    ]) {
      const href = name === 'AUTO_UPDATE' ? `#${fragment}` : `/docs/${name}#${fragment}`;
      expect(section().querySelector(`a[href="${href}"]`)).not.toBeNull();
      expect(article(name).querySelector(`#${fragment}`)).not.toBeNull();
    }
    for (const name of ['INSTALL', 'AUTO_UPDATE']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });
});
