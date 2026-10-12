import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const read = (name: string) => readFileSync(resolve(process.cwd(), '../docs', `${name}.md`), 'utf8');
const article = createGuideFixture(read);
const text = (node: Element) => node.textContent!.replace(/\s+/g, ' ').trim();

describe('OIDC setup without losing access', () => {
  it('renders the real setup sequence before optional local-login hiding', () => {
    const guide = article('OIDC');
    const setup = guide.querySelector('[id="-quick-start"]')!;
    expect(text(setup.nextElementSibling!)).toContain('independent administrator login');
    const list = setup.nextElementSibling!.nextElementSibling!;
    expect(list.tagName).toBe('OL');
    expect([...list.children].map((step) => text(step.querySelector('strong')!))).toEqual([
      'Prepare the IdP application.',
      'Configure access before saving.',
      'Save, then register the actual callback.',
      'Verify without replacing the working login.',
    ]);
    expect(text(list.children[1])).toContain('Show access restrictions & role mapping');
    expect(text(list.children[1])).toContain('New UI providers are enabled on save');
    expect(text(list.children[2])).toContain('literal <provider-id> placeholder');
    expect(text(list.children[3])).toContain('not the callback, user claims');
    expect(text(list.nextElementSibling!)).toContain('does not create or restore local credentials');
  });

  it('keeps the common claim advice consistent with the Entra recipe', () => {
    const guide = article('OIDC');
    expect(text(guide)).not.toContain('Requires the groups scope/claim');
    expect(text(guide)).not.toContain('Ensure your IdP includes the groups scope');
    expect(text(guide)).toContain('a groups scope is not universally supported');
    expect(text(guide)).toContain('Pulse reads the configured groups claim, not a scope name');
    const entra = guide.querySelector('#microsoft-entra-id-formerly-azure-ad')!;
    expect(text(entra.nextElementSibling!)).toContain('client credentials first');
    expect(text(guide)).toContain('Complete the redirect registration before signing in');
    expect(text(guide)).toContain('Do not add groups');
    expect(readFileSync(resolve(process.cwd(), 'public/docs/OIDC.md'), 'utf8')).toBe(read('OIDC'));
  });

  it('links all setup decisions to real focused documentation headings', () => {
    const guide = article('OIDC');
    for (const [name, fragment] of [
      ['OIDC', '-provider-examples'],
      ['OIDC', 'access-control'],
      ['OIDC', 'group-to-role-mapping'],
      ['OIDC', 'safe-diagnosis'],
      ['OIDC', '-quick-start'],
      ['REVERSE_PROXY', 'before-configuring-the-proxy'],
    ]) {
      const href = name === 'OIDC' ? `#${fragment}` : `/docs/${name}#${fragment}`;
      expect(guide.querySelector(`a[href="${href}"]`)).not.toBeNull();
      expect(article(name).querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
  });
});
