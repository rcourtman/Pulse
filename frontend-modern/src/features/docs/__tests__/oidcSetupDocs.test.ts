import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const read = (name: string) =>
  readFileSync(resolve(process.cwd(), '../docs', `${name}.md`), 'utf8');
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
    expect(text(list.nextElementSibling!)).toContain(
      'does not create or restore local credentials',
    );
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

  it('distinguishes hiding local login from enforcing SSO-only authentication', () => {
    const guide = article('OIDC');
    const warning = [...guide.querySelectorAll('p')].find((paragraph) =>
      text(paragraph).startsWith('Hiding the form is not SSO-only enforcement.'),
    )!;
    expect(warning).toBeDefined();
    expect(text(warning.querySelector('strong')!)).toBe(
      'Hiding the form is not SSO-only enforcement.',
    );
    expect(text(warning.previousElementSibling!)).toContain('PULSE_AUTH_HIDE_LOCAL_LOGIN=true');
    expect(text(warning.previousElementSibling!)).toContain('?show_local=true');
    expect(text(warning)).toContain('valid existing local credentials can still sign in');
    expect(text(warning)).toContain('does not reject those requests or revoke local sessions');
    expect(text(warning)).toContain('Keep the independent administrator recovery path');
    expect(text(warning)).toContain('do not remove credentials or disable authentication');
    expect(text(warning)).toContain('not to the separate local password login');
    expect(warning.querySelector('pre, form, input, button')).toBeNull();
  });

  it('keeps an independent admin login when explaining OIDC administrator access', () => {
    const guide = article('OIDC');
    const access = guide.querySelector('#access-control')!;
    const intro = access.nextElementSibling!;
    const lists = intro.nextElementSibling!;
    const note = lists.nextElementSibling!;
    expect(text(intro)).toContain('every non-empty list must match');
    expect(text(lists.firstElementChild!)).toContain(
      'An empty group list skips only that check, and any domain and email restrictions still apply',
    );
    expect(note.tagName).toBe('BLOCKQUOTE');
    expect(text(note)).toContain('Passing the sign-in rules does not grant instance administrator');
    expect(note.querySelector('code')).toHaveTextContent('admin');
    expect(text(note)).toContain('Group Role Mappings');
    expect(text(note)).toContain('keep your independent administrator login even after SSO works');
    expect(text(note)).not.toContain('removing the local administrator');
    const link = note.querySelector('a[href="#-quick-start"]')!;
    expect(link).toHaveTextContent('Check admin access in a separate browser session');
    expect(guide.querySelector('[id="-quick-start"]')).not.toBeNull();
  });

  it('routes a client rejection after saving to a readback without inviting another write', () => {
    const guide = article('OIDC');
    const heading = [...guide.querySelectorAll('h3')].find((item) =>
      text(item).startsWith('invalid_client after saving'),
    )!;
    const reason = heading.nextElementSibling!;
    const action = reason.nextElementSibling!;
    expect(reason.querySelector('code')).toHaveTextContent('invalid_client');
    expect(text(reason)).toContain("rejected Pulse's client authentication");
    expect(text(reason)).toContain('does not prove Pulse lost the secret');
    expect(text(reason)).toContain('Compare the saved Client ID');
    expect(text(action)).toContain('Keep your working administrator session open');
    expect(text(action)).toContain('Do not repeat the save, rotate the secret or sign out');
    const readback = action.querySelector('a[href="#a-provider-change-could-not-be-confirmed"]')!;
    expect(readback).toHaveTextContent('check the saved provider');
    const destination = guide.querySelector('#a-provider-change-could-not-be-confirmed')!;
    expect(text(destination.nextElementSibling!.nextElementSibling!)).toContain(
      'Check saved providers',
    );
    expect(destination.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it('distinguishes a returned login, the options check, and a role problem', () => {
    const guide = article('OIDC');
    const heading = guide.querySelector('#safe-diagnosis')!;
    const diagnosis = heading.nextElementSibling!;
    const privacy = diagnosis.nextElementSibling!;
    expect(text(diagnosis)).toContain(
      'Back at Pulse sign-in after SSO? Your session is unconfirmed.',
    );
    expect(text(diagnosis.querySelector('strong')!)).toBe('Retry sign-in check');
    expect(text(diagnosis)).toContain('if shown, only reads the sign-in options');
    expect(text(diagnosis)).toContain('Keep your working admin session open and do not resave');
    expect(text(diagnosis)).toContain(
      'If Pulse opens without the expected controls, check the role',
    );
    expect(text(diagnosis)).toContain('existing login error');
    expect(text(privacy)).toContain('claim names locally');
    expect(text(privacy)).toContain('Do not paste a full IdP response');
    expect(text(privacy)).toContain('full callback query strings');
    expect(text(privacy)).toContain('Never disable TLS verification or access restrictions');
    expect(privacy.nextElementSibling).toHaveAttribute(
      'id',
      'a-provider-change-could-not-be-confirmed',
    );
  });

  it('routes OIDC and proxy readers to the same callback check before changing settings', () => {
    const guide = article('OIDC');
    const fragment = 'check-the-callback-address';
    const heading = guide.querySelector(`h3[id="${fragment}"]`)!;
    const oidcEntry = guide.querySelector(`tr a[href="#${fragment}"]`)!;
    const proxyEntry = article('REVERSE_PROXY').querySelector(`a[href="/docs/OIDC#${fragment}"]`)!;
    expect(oidcEntry.closest('tr')).toHaveTextContent('Redirect Loop');
    expect(proxyEntry.closest('li')).toHaveTextContent('OIDC redirects fail');

    const compare = heading.nextElementSibling!;
    const correct = compare.nextElementSibling!;
    expect(text(compare)).toContain('Keep your working administrator session open');
    expect(text(compare)).toContain('correct it in the identity provider');
    expect(text(compare)).toContain('There is no need to save the Pulse provider again');
    expect(text(correct)).toContain('A saved override takes precedence over Public URL');
    expect(
      correct.querySelector('a[href="/docs/REVERSE_PROXY#before-configuring-the-proxy"]'),
    ).toHaveTextContent('trusted proxy setup');
    expect(correct.nextElementSibling!.querySelector('a[href="#safe-diagnosis"]')).not.toBeNull();
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
