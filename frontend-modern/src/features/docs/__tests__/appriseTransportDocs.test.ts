import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';
import {
  ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL,
  ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL,
} from '@/utils/alertDestinationsPresentation';

const read = (name: string) =>
  readFileSync(resolve(process.cwd(), '../docs', `${name}.md`), 'utf8');
const article = createGuideFixture(read);
const target = 'apprise-api-reports-a-tls-or-connection-error';
const text = (node: Element) => node.textContent!.replace(/\s+/g, ' ').trim();
const section = () => {
  const guide = article('TROUBLESHOOTING');
  const heading = guide.querySelector(`#${target}`)!;
  expect(heading?.tagName).toBe('H4');
  const content = document.createElement('section');
  for (
    let node = heading.nextElementSibling;
    node && node.tagName !== 'H4';
    node = node.nextElementSibling
  ) {
    content.append(node.cloneNode(true));
  }
  return { guide, content };
};

describe('Apprise help keeps the two transports and recipient receipt separate', () => {
  it('routes the symptom chooser and TLS failure row to the rendered help', () => {
    const { guide } = section();
    const links = [...guide.querySelectorAll<HTMLAnchorElement>(`a[href="#${target}"]`)];
    expect(links).toHaveLength(2);
    expect(text(links[0].closest('li')!)).toContain('Apprise reports a TLS or connection error');
    expect(text(links[1].closest('tr')!)).toContain('do not disable verification');
    expect(readFileSync(resolve(process.cwd(), 'public/docs/TROUBLESHOOTING.md'), 'utf8')).toBe(
      read('TROUBLESHOOTING'),
    );
  });

  it('renders HTTPS bypass and plaintext HTTP as different risks, not a connection result', () => {
    const { content } = section();
    const rows = [...content.querySelectorAll('tbody tr')];
    expect(rows).toHaveLength(4);
    expect(text(rows[0])).toContain('not just your browser');
    expect(text(rows[0])).toContain('not from the failing connection');
    expect(text(rows[1])).toContain('disables certificate and hostname checks');
    expect(text(rows[1])).toContain(
      'Older versions call the same bypass Allow self-signed certificates',
    );
    expect(text(rows[1])).toContain('Leave this option unchecked');
    expect(text(rows[2])).toContain('checkbox has no effect');
    expect(text(rows[2])).toContain('plaintext');
    expect(text(rows[3])).toContain('does not establish bad credentials or a certificate fault');
    expect(text(content)).toContain(ALERT_DESTINATIONS_APPRISE_MODE_CLI_LABEL);
    expect(text(content)).toContain(ALERT_DESTINATIONS_APPRISE_MODE_HTTP_LABEL);
    expect(text(content)).toContain('affects neither the local CLI nor the separate connections');
  });

  it('keeps natural delivery, retained settings and private evidence boundaries usable', () => {
    const { guide, content } = section();
    const help = text(content);
    for (const boundary of [
      'not that its provider delivered it or that the recipient received it',
      "skips Pulse's persistent queue",
      'editing the current address or key does not update them',
      'not a batch replay',
      'a current form warning is not a record of an earlier attempt',
      'Do not switch modes or replace a saved key',
      'Do not send another Test, induce an alert, weaken verification or enable Debug',
      'configuration key, API key and header values, target URLs',
      'credential-bearing command or URL',
    ])
      expect(help).toContain(boundary);
    expect(content.querySelector('pre, input, button')).toBeNull();
    for (const anchor of content.querySelectorAll<HTMLAnchorElement>('a[href]')) {
      expect(anchor.getAttribute('href')).toMatch(/^#/);
      expect(guide.querySelector(anchor.getAttribute('href')!)).not.toBeNull();
    }
  });
});
