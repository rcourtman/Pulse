import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const read = (file: string) => readFileSync(path.resolve(process.cwd(), '..', file), 'utf8');
const article = createGuideFixture((name) => read(`frontend-modern/public/docs/${name}.md`));
const text = (element: Element) => element.textContent!.replace(/\s+/g, ' ');

describe('AI provider privacy guidance', () => {
  it('corrects the local-processing promise before the Patrol walkthrough', () => {
    const rendered = article('AI');
    expect(text(rendered)).not.toContain('BYOK for complete privacy');
    expect(text(rendered)).not.toContain('only sends the minimal context');
    const notice = [...rendered.querySelectorAll('p')].find((node) =>
      text(node).includes('BYOK does not make model processing local'),
    );
    expect(notice).toBeDefined();
    expect(text(notice!)).toContain('a hosted model receives provider-bound context');
    expect(notice!.querySelector('a')?.getAttribute('href')).toBe('#privacy');
    const patrol = rendered.querySelector('#pulse-patrol');
    expect(patrol).not.toBeNull();
    expect(notice!.compareDocumentPosition(patrol!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('keeps model traffic, policy handling and usage telemetry distinct in both guides', () => {
    for (const [name, id] of [
      ['AI', 'privacy'],
      ['PRIVACY', 'ai-provider-data'],
    ]) {
      const rendered = article(name);
      const heading = rendered.querySelector(`#${id}`);
      expect(heading, name).not.toBeNull();
      const section = document.createElement('section');
      for (
        let node = heading!.nextElementSibling;
        node && !/^H[12]$/.test(node.tagName);
        node = node.nextElementSibling
      ) {
        section.appendChild(node.cloneNode(true));
      }
      const copy = text(section);
      for (const distinction of [
        'resource names and addresses',
        'tool results',
        'retention and training policies',
        'Internal and Sensitive resources can still be shared with a hosted model',
        'do not make every',
        'does not forward requests elsewhere',
        'does not stop AI provider requests or delete locally retained AI history',
      ]) {
        expect(copy, name).toContain(distinction);
      }
      expect(section.querySelector('pre'), name).toBeNull();
    }
  });

  it('links the real provider and local-history boundaries without weakening restricted access', () => {
    const rendered = article('AI');
    const copy = text(rendered);
    expect(copy).toContain('known restricted identifiers are redacted');
    expect(copy).toContain('secret filtering');
    expect(copy).toContain('Keep credentials out of notes and prompts');
    expect(copy).toContain('External MCP clients have their own data flow');
    expect(copy).toContain('tool results it is authorised to read');
    expect(rendered.querySelector('a[href="/docs/PRIVACY#ai-provider-data"]')).not.toBeNull();
    expect(article('PRIVACY').querySelector('#ai-provider-data')).not.toBeNull();
    expect(
      rendered.querySelector('a[href="/docs/CONFIGURATION#handle-configuration-and-history-privately"]'),
    ).not.toBeNull();
    expect(article('CONFIGURATION').querySelector('#handle-configuration-and-history-privately')).not.toBeNull();
    expect(copy).toContain('locally reviewed and redacted excerpt');
  });

  it('ships the same two guides through Docs and the repository', () => {
    for (const name of ['AI', 'PRIVACY']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });
});
