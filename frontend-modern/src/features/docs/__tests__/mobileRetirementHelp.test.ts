import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

// These are checks of the shipped server documentation, not app or Relay
// qualification. No pairing, notification, subscription or provider is used.
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const shipped = (name: string) =>
  readFileSync(path.join(root, 'frontend-modern/public/docs', `${name}.md`), 'utf8');
const render = createGuideFixture(shipped);
const prose = (element: Element) => (element.textContent ?? '').replace(/\s+/g, ' ').trim();

function section(name: string, fragment: string, level: number): HTMLElement {
  const heading = render(name).querySelector(`[id="${fragment}"]`);
  expect(heading, `${name}#${fragment}`).not.toBeNull();
  const result = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-6]$/.test(next.tagName) && Number(next.tagName.slice(1)) <= level) break;
    result.appendChild(next.cloneNode(true));
  }
  return result;
}

describe('existing phone recovery and retirement help', () => {
  it('routes Cloud phone setup to existing web access, not a new Relay pairing', () => {
    const guide = render('CLOUD');
    const text = prose(guide);
    const setup = prose(section('CLOUD', '4-set-up-mobile-access', 3));
    expect(setup).toContain("usual HTTPS address in your phone's browser");
    expect(setup).toContain('Relay is not needed and does not provide web access');
    expect(setup).toContain('Alerts → Notifications');
    expect(setup).toContain('ntfy, Gotify or Pushover');
    expect(setup).toContain('A successful Test is not proof of ordinary alert delivery');
    expect(text).not.toMatch(/Relay is (?:enabled by default|pre-configured)/i);
    expect(text).not.toContain('to pair a phone');
    expect(text).not.toContain('supported Pulse Mobile client');
    expect(text).not.toContain('pre-configured on Cloud');
  });

  it('preserves existing Cloud phone connections and continuing subscriber terms', () => {
    const text = prose(section('CLOUD', '4-set-up-mobile-access', 3));
    expect(text).toContain('31 March 2027');
    expect(text).toContain('Existing paired phones keep working until then');
    expect(text).toContain('do not remove or re-pair them just to follow this guide');
    expect(text).toContain('Relay is no longer sold');
    expect(text).toContain('Existing Relay subscribers receive Pro features');
    expect(text).toContain('current price for as long as their subscription continues');
    expect(text).toContain('including after the app retires');
    expect(text).toContain('do not change your subscription or require a new purchase');
  });

  it('keeps retirement, continuing paid terms and app/web access distinct', () => {
    const text = prose(section('TROUBLESHOOTING', 'relay-showing-disconnected', 4));
    expect(text).toContain('31 March 2027');
    expect(text).toContain('existing paired phones keep working until then');
    expect(text).toContain('Relay is no longer sold');
    expect(text).toContain('current price for as long as their subscription continues');
    expect(text).toContain('including after the app retires');
    expect(text).toContain('Do not buy or switch plans');
    expect(text).toContain('not the web UI');
    expect(text).toContain('ntfy, Gotify or Pushover');
    expect(text).toContain('own VPN or tunnel');
    expect(prose(section('RELAY', 'requirements', 2))).toContain(
      'An existing licence with the relay entitlement',
    );
    expect(prose(section('RELAY', 'requirements', 2))).not.toContain('Relay-or-higher');
  });

  it('replaces unbounded reads and speculative recovery with preserved evidence', () => {
    const element = section('RELAY', 'troubleshooting', 2);
    const text = prose(element);
    expect(element.querySelector('pre')).toBeNull();
    expect(text).toContain('An active server connection does not prove phone delivery');
    expect(text).toContain('Unknown details are valid');
    expect(text).toContain('bounded Pulse log readers');
    expect(text).toContain('Do not post full logs, QR codes, pairing deep links, cookies, tokens');
    expect(text).toContain('Do not disable/re-enable relay, delete a pairing, rescan a QR code');
    expect(text).toContain('restart services or change firewall rules merely to collect evidence');
    expect(text).toContain('do not pair another device as a test');
    expect(text).toContain('A successful Test is not proof of ordinary alert delivery');
    expect(text).not.toContain('Confirm your mobile account has beta access');
    expect(text).not.toContain('Restart Pulse Mobile');
  });

  it('keeps existing bookmarks and every recovery link usable in the actual renderer', () => {
    const sections = [
      section('TROUBLESHOOTING', 'relay-showing-disconnected', 4),
      section('RELAY', 'troubleshooting', 2),
      section('CLOUD', '4-set-up-mobile-access', 3),
    ];
    for (const element of sections) {
      for (const link of element.querySelectorAll('a[href]')) {
        const href = link.getAttribute('href')!;
        const same = /^#(.+)$/.exec(href);
        const cross = /^\/docs\/([^#]+)(?:#(.+))?$/.exec(href);
        expect(same || cross, href).not.toBeNull();
        const destination = render(cross ? cross[1] : 'RELAY');
        const fragment = cross ? cross[2] : same![1];
        if (fragment) expect(destination.querySelector(`[id="${fragment}"]`), href).not.toBeNull();
        else expect(prose(destination), href).not.toBe('');
        // Both link forms use shared explicit activation. Bare fragments keep
        // their native href so modified/new-tab clicks still work.
        expect(link.hasAttribute('data-doc-link'), href).toBe(true);
      }
    }
    for (const fragment of [
      'relay-showing-disconnected',
      'pulse-mobile-cant-connect',
      'open-pulse-handoff-not-loading',
    ]) {
      expect(render('RELAY').querySelector(`[id="${fragment}"]`), fragment).not.toBeNull();
    }
  });

  it('ships the same guidance as the canonical repository pages', () => {
    for (const name of ['TROUBLESHOOTING', 'RELAY', 'CLOUD']) {
      expect(shipped(name)).toBe(readFileSync(path.join(root, 'docs', `${name}.md`), 'utf8'));
    }
  });
});
