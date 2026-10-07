import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string): string => readFileSync(path.join(root, name), 'utf8');
const article = (name: string): HTMLElement => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return element;
};
const sectionText = (name: string, start: string, end: string): string => {
  const source = read(`docs/${name}.md`).split(start)[1]?.split(end)[0];
  expect(
    source,
    'privacy precautions must appear at the delivery diagnosis entry point',
  ).toBeDefined();
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(source!, name);
  return element.textContent!.replace(/\s+/g, ' ').trim();
};
const recoveryText = (): string =>
  sectionText(
    'TROUBLESHOOTING',
    '#### Recover retained delivery failures',
    '#### Emails not sending',
  );
const retryText = (): string =>
  sectionText('WEBHOOKS', '**Retries and retained failures.**', '**Correlation header.**');

describe('notification evidence privacy help', () => {
  it('ships the exact tested guidance instead of an older sanitisation promise', () => {
    for (const name of ['TROUBLESHOOTING', 'WEBHOOKS']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
    expect(recoveryText()).not.toContain('safely redacted provider errors');
    expect(recoveryText()).toContain(
      'masks recognised credentials in URLs, not all private information',
    );
  });

  it('keeps URL masking separate from free text, identity and screenshot privacy', () => {
    const text = recoveryText();
    expect(text).toContain(
      'email addresses, private destinations or credentials echoed outside a URL',
    );
    expect(text).toContain('alert identifiers and resource names');
    expect(text).toContain('Keep full entries, copied responses and screenshots private');
    expect(text).toContain('A REDACTED marker does not make the rest safe to share');
    expect(text).toContain(
      'extract only the relevant timestamp, delivery method, failure class, HTTP status or SMTP error code and a manually redacted error',
    );
    // Privacy guidance must not erase the retained-failure safety precautions.
    expect(text).toContain('does not replace that saved configuration');
    expect(text).toContain('all retained terminal failures');
    expect(text).toContain('Do not delete notification_queue.db');
  });

  it('warns at the webhook retry entry point without promising a fully safe error', () => {
    const text = retryText();
    expect(text).toContain(
      'masks recognised URL credentials, not arbitrary provider text or private infrastructure details',
    );
    expect(text).toContain('Keep full errors and screenshots private');
    expect(text).toContain('before sharing a manually redacted excerpt');
    expect(text).toContain(
      'A REDACTED marker is not proof that the remaining text is safe to post',
    );
  });

  it('connects both entry points to existing shipped privacy precautions', () => {
    const cases = [
      [
        'TROUBLESHOOTING',
        'TROUBLESHOOTING',
        '#inspect-notification-logs',
        'notification log precautions',
      ],
      [
        'WEBHOOKS',
        'TROUBLESHOOTING',
        '/docs/TROUBLESHOOTING#recover-retained-delivery-failures',
        'delivery evidence precautions',
      ],
    ];
    for (const [from, to, href, label] of cases) {
      const link = article(from).querySelector(`a[href="${href}"]`);
      expect(link?.textContent).toBe(label);
      expect(article(to).querySelector(href.slice(href.indexOf('#')))).not.toBeNull();
    }
  });
});
