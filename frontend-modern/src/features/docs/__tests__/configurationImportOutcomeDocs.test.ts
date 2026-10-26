import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8');
const articleFor = createGuideFixture(readDoc);
const textOf = (element: Element) => element.textContent!.replace(/\s+/g, ' ');

function failureSection() {
  const heading = articleFor('MIGRATION').querySelector('#when-an-import-fails-or-times-out');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && !/^H[23]$/.test(node.tagName);
    node = node.nextElementSibling
  ) {
    section.append(node.cloneNode(true));
  }
  return section;
}

describe('Configuration import outcome guidance', () => {
  it('routes an unsuccessful transfer to outcome checks before cutover', () => {
    const article = articleFor('MIGRATION');
    const resultStep = article.querySelector(
      '#transfer-the-included-configuration + ol > li:nth-child(4)',
    );
    expect(resultStep?.querySelector('a')?.getAttribute('href')).toBe(
      '#when-an-import-fails-or-times-out',
    );
    const headings = [...article.querySelectorAll('h2, h3')].map((heading) => heading.id);
    expect(headings.indexOf('when-an-import-fails-or-times-out')).toBeLessThan(
      headings.indexOf('cut-over-and-verify'),
    );
    expect(textOf(failureSection())).toContain('A failed response is not a rollback');
  });

  it('matches the producer errors without mistaking a saved archive for a rejection', () => {
    const source = readFileSync(
      path.join(repoRoot, 'internal/api/configapi/config_export_import_handlers.go'),
      'utf8',
    );
    const section = failureSection();
    const code = [...section.querySelectorAll('code')].map((node) => node.textContent!);
    for (const message of [
      'Configuration saved, but live reload failed:',
      'Configuration imported but failed to reload',
      'Configuration imported but failed to apply changes',
    ]) {
      expect(source).toContain(message);
      expect(code).toContain(message);
    }
    const componentNames = [
      ...source.matchAll(/reloadFailures = append\(reloadFailures, "([^"]+)"\)/g),
    ].map((match) => match[1]);
    expect(new Set(componentNames).size).toBe(4);
    for (const component of componentNames) expect(textOf(section)).toContain(component);
    const text = textOf(section);
    expect(text).toContain('imported settings are on disk');
    expect(text).toContain('other components may already have applied');
    expect(text).toContain('Do not repeat the import');
    expect(text).toContain('overwrite legitimate changes made since the first attempt');
  });

  it('treats lost responses and duplicate guards as uncertainty, not a retry timer', () => {
    const text = textOf(failureSection());
    expect(text).toContain('No final response, or HTTP 409');
    expect(text).toContain('does not establish whether the original import saved or applied');
    expect(text).toContain('Do not resubmit it to discover the outcome');
    expect(text).toContain('temporary and process-local');
    expect(text).toContain('not a durable rollback or permission to retry');
    expect(text).toContain('waiting or restarting');
  });

  it('preserves isolated, same-organisation recovery without induced side effects', () => {
    const section = failureSection();
    const text = textOf(section);
    expect(text).toContain("destination's pre-import backup");
    expect(text).toContain('isolated from providers, agents, notifications and remote actions');
    expect(text).toContain('same destination and organisation');
    expect(text).toContain('destination-local administrator access');
    expect(text).toContain('does not prove notification delivery');
    expect(text).toContain('Do not restart Pulse, send a test notification or repeat a backup');
    expect(text).toContain('do not enable a second poller');
    expect(section.querySelector('pre, form, input, button')).toBeNull();
    const handover = section.querySelector('a[href="#hand-over-monitoring-once"]');
    expect(handover).not.toBeNull();
    expect(articleFor('MIGRATION').querySelector('#hand-over-monitoring-once')).not.toBeNull();
  });

  it('documents non-idempotent API outcomes with a working reader-facing recovery link', () => {
    const article = articleFor('API');
    const heading = article.querySelector('#import-configuration');
    expect(heading).not.toBeNull();
    const section = document.createElement('section');
    for (
      let node = heading!.nextElementSibling;
      node && !/^H[23]$/.test(node.tagName);
      node = node.nextElementSibling
    ) {
      section.append(node.cloneNode(true));
    }
    const text = textOf(section);
    expect(text).toContain('not an idempotent merge');
    expect(text).toContain('HTTP 500 can follow a saved archive');
    expect(text).toContain('Do not automatically retry this POST');
    expect(text).toContain('HTTP 409');
    expect(text).toContain('not the archive, passphrase or raw configuration');
    expect(
      section.querySelector('a[href="/docs/MIGRATION#when-an-import-fails-or-times-out"]'),
    ).not.toBeNull();
  });

  it('ships the exact migration and API guidance used by the renderer tests', () => {
    for (const name of ['MIGRATION', 'API']) {
      expect(readFileSync(path.join(frontendRoot, 'public/docs', `${name}.md`), 'utf8')).toBe(
        readDoc(name),
      );
    }
  });
});
