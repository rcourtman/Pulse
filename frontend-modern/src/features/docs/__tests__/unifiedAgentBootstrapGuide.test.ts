import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const source = readFileSync(path.join(frontendRoot, '../docs/UNIFIED_AGENT.md'), 'utf8');

function setupSection() {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source, 'UNIFIED_AGENT');
  const heading = article.querySelector('#private-file-installation-linux-macos-and-nas')!;
  const section = document.createElement('section');
  for (
    let node = heading.nextElementSibling;
    node && !/^H[23]$/.test(node.tagName);
    node = node.nextElementSibling
  ) {
    section.append(node.cloneNode(true));
  }
  return section;
}

describe('Unified Agent private-file setup help', () => {
  it('renders preparation and download as self-contained fences before installation', () => {
    const section = setupSection();
    const steps = [...section.querySelectorAll(':scope > ol > li')];
    expect(steps).toHaveLength(3);
    const commands = steps.map((step) => step.querySelector('pre code')?.textContent ?? '');
    for (const command of commands.slice(0, 2)) {
      expect(command.trim()).toMatch(/^\(\s+set -eu\s+umask 077/);
      expect(command.trim()).toMatch(/\)$/);
    }
    expect(commands[0]).toContain('vi "$credential_file"');
    expect(commands[1]).toContain('curl --disable --fail');
    expect(commands[1]).toContain('[ "$status" = 200 ]');
    expect(commands[2]).toContain('--token-file "$HOME/.config/pulse/agent-token"');
    expect(commands.join(' ')).not.toMatch(/--token\s|curl[^|]+\|\s*bash/);
  });

  it('keeps stopping, existing-file and retarget trust boundaries beside the commands', () => {
    const steps = [...setupSection().querySelectorAll(':scope > ol > li')];
    const preparation = steps[0].textContent!.replace(/\s+/g, ' ');
    const download = steps[1].textContent!.replace(/\s+/g, ' ');
    expect(preparation).toContain('Stop if preparation fails');
    expect(preparation).toContain('preserves an existing regular token file');
    expect(preparation).toContain('refuses symlinked or non-regular credential paths');
    expect(download).toContain('Stop if the download fails');
    expect(download).toContain('do not execute its temporary file');
    expect(download).toContain('does not follow redirects or overwrite an existing installer');
    expect(download).toContain('retargeting needs the installer from the new server');
    expect(download).toContain("add curl's --cacert after --disable");
    expect(download).toContain('A successful download is not signature verification');
  });

  it('keeps the existing profile link and ships an identical public help asset', () => {
    expect(setupSection().querySelector('a[href="#installation-options"]')).not.toBeNull();
    expect(readFileSync(path.join(frontendRoot, 'public/docs/UNIFIED_AGENT.md'), 'utf8')).toBe(
      source,
    );
  });
});
