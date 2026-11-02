import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const read = (name: string) =>
  readFileSync(
    resolve(process.cwd(), '..', name === 'SECURITY' ? 'SECURITY.md' : `docs/${name}.md`),
    'utf8',
  );
const article = createGuideFixture(read);
const text = (element: Element) => element.textContent!.replace(/\s+/g, ' ').trim();
const registration = () => {
  const guide = article('SECURITY');
  const heading = guide.querySelector('#auto-registration-security')!;
  const section = document.createElement('section');
  for (
    let next = heading.nextElementSibling;
    next && !['H2', 'H3'].includes(next.tagName);
    next = next.nextElementSibling
  ) {
    section.append(next.cloneNode(true));
  }
  return section;
};

describe('registration security guidance', () => {
  it('explains always-authenticated registration instead of invented security modes', () => {
    const section = registration();
    expect(text(section)).toContain('Registration always authenticates its request');
    expect(text(section)).toContain('even when no long-lived API token is configured');
    expect(text(section)).toContain('does not mean that an unauthenticated caller can register');
    expect(section.querySelector('h4')).toBeNull();
    expect(text(section)).not.toContain('legacy env seeding');
  });

  it('distinguishes one-time setup, reporting and limited Proxmox bootstrap credentials', () => {
    const items = [...registration().querySelectorAll('li')];
    expect(items).toHaveLength(2);
    expect(text(items[0])).toContain('short-lived, node-type-specific setup token');
    expect(text(items[0])).toContain('Registration consumes it');
    expect(text(items[1])).toContain('agent:report');
    expect(text(items[1])).toContain('does not authorise arbitrary new connections');
    expect(text(items[1])).toContain('separate, limited bootstrap grant');
  });

  it('routes readers away from ignored environment variables without weakening access', () => {
    const section = registration();
    expect(text(section)).toContain('Settings → API Access');
    expect(text(section)).toContain('API_TOKEN and API_TOKENS environment variables are ignored');
    expect(text(section)).toContain('does not create a token, secure an installation');
    expect(text(section)).toContain('instead of widening scopes, disabling authentication');
    expect(text(section)).toContain(
      'Keep HTTPS verification and existing saved connections intact',
    );
    for (const [name, fragment] of [
      ['CONFIGURATION', '-api-tokens'],
      ['UNIFIED_AGENT', 'quick-start'],
    ]) {
      expect(section.querySelector(`a[href="/docs/${name}#${fragment}"]`)).not.toBeNull();
      expect(article(name).querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
    expect(section.querySelector('pre, form, input, button')).toBeNull();
    expect(readFileSync(resolve(process.cwd(), 'public/docs/SECURITY.md'), 'utf8')).toBe(
      read('SECURITY'),
    );
  });
});
