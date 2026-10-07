import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { UPDATES_PANEL_COPY } from '@/utils/updatesPresentation';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (name: string) => readFileSync(path.join(root, name), 'utf8');
const source = readFileSync(
  process.env.PULSE_SOURCE_UPDATE_GUIDE_TEST_INPUT ??
    path.join(root, 'frontend-modern/public/docs/AUTO_UPDATE.md'),
  'utf8',
);
const legacy = readFileSync(
  process.env.PULSE_LEGACY_UPDATE_GUIDE_TEST_INPUT ??
    path.join(root, 'docs/operations/AUTO_UPDATE.md'),
  'utf8',
);

function render(markdown: string, name = 'AUTO_UPDATE'): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
}

function section(id: string): HTMLElement {
  const heading = render(source).querySelector(`#${id}`);
  expect(heading).not.toBeNull();
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-6]$/.test(next.tagName)) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
}

const prose = (element: HTMLElement) => (element.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('source-built server update help', () => {
  it('distinguishes checkout build output from the installed executable', () => {
    // Bind the explanation to the actual development Makefile, not a native update claim.
    const makefile = read('Makefile');
    expect(makefile).toContain('backend:\n\tgo build -o pulse ./cmd/pulse');
    expect(makefile).toContain('build: frontend backend');
    const element = section('source-build');
    const text = prose(element);
    expect(text).toContain('writes a pulse binary in the source checkout');
    expect(text).toContain(
      "does not install that binary into the running service's executable path",
    );
    expect(text).toContain('restart the old installed binary');
    expect(text).toContain('Do not build in the active installation directory');
    expect(element.querySelector('pre')).toBeNull();
    expect(source).not.toContain('git pull\nmake build\nsudo systemctl restart pulse');
  });

  it('requires deliberate source selection and safe admission before an outage', () => {
    const text = prose(section('source-build'));
    expect(text).toContain('source tag or commit explicitly in a separate build checkout');
    expect(text).toContain('preserving local changes');
    expect(text).toContain('main branch is development source, not the latest published stable');
    expect(text).toContain("checkout's declared toolchain and locked dependencies");
    expect(text).toContain("binary's version before stopping the running service");
    expect(text).toContain('do not copy onto a running executable');
    expect(text).toContain('recovery procedure is unknown, stop before changing the service');
  });

  it('preserves edition and persistent state and does not equate restart with recovery', () => {
    const text = prose(section('source-build'));
    expect(text).toContain('Keep private Pro installations on their private runtime');
    expect(text).toContain(
      'service identity, executable path, configuration, credentials and data',
    );
    expect(text).toContain('Restore only a service that was active before the update');
    expect(text).toContain('running version, ordinary collection and notification delivery');
    expect(text).toContain('previous binary and state backup until recovery is verified');
    expect(text).toContain('Do not pull, rebuild or restart just to reproduce');
  });

  it('routes preparation and recovery to real headings in the shipped docs', () => {
    const links = [...section('source-build').querySelectorAll('a[href]')];
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/docs/INSTALL#-updates',
      '/docs/DEPLOYMENT_MODELS#updates-by-model',
    ]);
    for (const link of links) {
      const [name, fragment] = link.getAttribute('href')!.slice('/docs/'.length).split('#');
      expect(
        render(read(`frontend-modern/public/docs/${name}.md`), name).querySelector(`#${fragment}`),
      ).not.toBeNull();
      expect(link.hasAttribute('data-doc-link')).toBe(true);
      expect(link.hasAttribute('target')).toBe(false);
    }
  });

  it('retires shared temporary configuration writes and unconditional rollback advice', () => {
    const text = prose(render(legacy));
    expect(text).toContain('Do not rewrite system.json through a shared temporary file');
    expect(text).toContain('expose configuration and replace its ownership or permissions');
    expect(text).toContain('does not prove that the old version is still running');
    expect(text).toContain('automatic rollback restored all state');
    expect(text).toContain('keep the original failure evidence and recovery snapshots');
    expect(text).toContain('Do not start an update service just to test a reported failure');
    expect(render(legacy).querySelector('pre')).toBeNull();
    expect(legacy).not.toContain('/tmp/system.json');
    expect(legacy).not.toContain('failures auto-restore');
  });

  it('uses the current UI label without treating preference storage as timer installation', () => {
    const text = prose(render(source));
    expect(text).toContain(UPDATES_PANEL_COPY.autoUpdateTitle);
    expect(text).toContain('Saving the UI preference does not provision or start a missing timer');
    expect(text).toContain('systemd timer');
    expect(text).toContain('unattended systemd auto-updates remain stable-only');
    expect(read('frontend-modern/public/docs/AUTO_UPDATE.md')).toBe(read('docs/AUTO_UPDATE.md'));
  });
});
