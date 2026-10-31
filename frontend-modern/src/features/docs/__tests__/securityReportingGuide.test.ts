import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const read = (name: string) => readFileSync(path.resolve(process.cwd(), '..', name), 'utf8');
const source = read('SECURITY.md');
const article = () => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(stripLeadingTitle(source), 'SECURITY');
  return element;
};

describe('private vulnerability reporting guidance', () => {
  it('puts the existing private route before unrelated deployment guidance', () => {
    expect(read('frontend-modern/public/docs/SECURITY.md')).toBe(source);
    const rendered = article();
    const headings = Array.from(rendered.querySelectorAll('h2'), (heading) => heading.id);
    expect(headings[0]).toBe('reporting-a-vulnerability');
    expect(headings[1]).toBe('critical-security-notice-for-container-deployments');
    const contact = rendered.querySelector('a[href="mailto:security@pulserelay.pro"]');
    expect(contact?.textContent).toBe('security@pulserelay.pro');
    // Opening the contact must not prefill credentials or diagnostic material.
    expect(contact?.getAttribute('href')).not.toContain('?');
    expect(rendered.querySelectorAll('a[href^="mailto:"]')).toHaveLength(1);
    expect(rendered.querySelector('a[href="#reporting-a-vulnerability"]')).not.toBeNull();
  });

  it('separates sensitive reports from public support without requesting unsafe evidence', () => {
    const rendered = article();
    const heading = rendered.querySelector('#reporting-a-vulnerability')!;
    const paragraphs: string[] = [];
    for (let next = heading.nextElementSibling; next && next.tagName !== 'HR'; next = next.nextElementSibling) {
      paragraphs.push(next.textContent ?? '');
    }
    const text = paragraphs.join(' ').replace(/\s+/g, ' ');
    expect(text).toContain('Do not open a public issue or discussion for these reports.');
    expect(text).toMatch(/affected Pulse and agent versions, if known/);
    expect(text).toMatch(/do not send live credentials, private keys, full diagnostics or configuration exports/);
    expect(text).toContain('Do not repeat a risky action just to make a report.');
    expect(text).toMatch(/Removing a public copy is not credential revocation/);
    expect(text).toContain('The security address is not a general support route.');
    expect(
      rendered.querySelector('a[href="/docs/TROUBLESHOOTING#if-you-already-posted-private-information"]'),
    ).not.toBeNull();
    expect(rendered.querySelector('a[href="https://github.com/rcourtman/Pulse/issues"]')).not.toBeNull();
    expect(rendered.querySelector('a[href="https://github.com/rcourtman/Pulse/discussions"]')).not.toBeNull();
  });
});
