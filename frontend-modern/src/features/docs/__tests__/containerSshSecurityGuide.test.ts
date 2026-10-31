import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown, stripLeadingTitle } from '../docMarkdown';

const read = (name: string) => readFileSync(path.resolve(process.cwd(), '..', name), 'utf8');
const source = read('SECURITY.md');
const notice = source
  .split('## Critical Security Notice for Container Deployments\n')[1]
  .split('\n---\n')[0];
const article = () => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(stripLeadingTitle(notice), 'SECURITY');
  return element;
};
const text = () => article().textContent!.replace(/\s+/g, ' ').trim();

describe('container SSH security guidance', () => {
  it('ships the canonical notice and distinguishes setup restriction from legacy collection', () => {
    expect(read('frontend-modern/public/docs/SECURITY.md')).toBe(source);
    expect(text()).toMatch(/setup flow blocks automatic SSH-key generation in containers/);
    expect(text()).toMatch(/existing usable key can still be used for legacy SSH collection/);
    expect(text()).toMatch(
      /with a security warning.*does not revoke the key or prevent outbound SSH/,
    );
    expect(text()).not.toMatch(
      /SSH-based temperature monitoring is blocked|❌ Blocked|✅ Not affected/,
    );
  });

  it('never offers a development override, wider key access or missing warnings as protection', () => {
    expect(text()).toMatch(/Do not enable PULSE_DEV_ALLOW_CONTAINER_SSH or mount a host key/);
    expect(text()).toMatch(/not needed for the legacy collector to attempt SSH/);
    expect(text()).toMatch(/missing warning is not proof.*container has no key/);
    expect(text()).toMatch(/Homelab use does not remove the risk/);
    expect(text()).toMatch(/deployment choice, not a guarantee enforced by container detection/);
    expect(article().querySelector('#auditing-your-deployment')).not.toBeNull();
    expect(article().querySelector('a[href="#auditing-your-deployment"]')).not.toBeNull();
    expect(article().querySelector('pre code')?.textContent).not.toContain(
      'PULSE_DEV_ALLOW_CONTAINER_SSH',
    );
  });

  it('preserves readings and independent access before any planned key retirement', () => {
    expect(text()).toMatch(/API-only monitoring without temperature data is also a valid choice/);
    expect(text()).toMatch(/not as a prerequisite for diagnosing an outage/);
    expect(text()).toMatch(/First verify.*collection works.*independent administrative login/);
    expect(text()).toMatch(/remove only that key's entry/);
    expect(text()).toMatch(/Deleting a container file does not erase an old image layer or backup/);
    expect(text()).toMatch(/Revoking the public key on every target host is the essential step/);
    const rendered = article();
    expect(
      rendered.querySelector(
        'a[href="/docs/TEMPERATURE_MONITORING#check-existing-linux-readings-safely"]',
      ),
    ).not.toBeNull();
    expect(
      rendered.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]'),
    ).not.toBeNull();
  });
});
