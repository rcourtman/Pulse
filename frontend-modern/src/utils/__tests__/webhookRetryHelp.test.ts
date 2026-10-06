import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (relative: string): string => readFileSync(path.join(repoRoot, relative), 'utf8');
const article = (name: string): HTMLElement => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return element;
};
const text = (element: Element): string => element.textContent!.replace(/\s+/g, ' ').trim();
const retryText = (): string => {
  const start = read('docs/WEBHOOKS.md').indexOf('**Retries');
  const end = read('docs/WEBHOOKS.md').indexOf('**Correlation header.**');
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(read('docs/WEBHOOKS.md').slice(start, end), 'WEBHOOKS');
  return text(element);
};
const recoveryText = (): string => {
  const guide = read('docs/TROUBLESHOOTING.md');
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(
    guide.split('#### Recover retained delivery failures')[1].split('#### Emails not sending')[0],
    'TROUBLESHOOTING',
  );
  return text(element);
};

describe('webhook retry help', () => {
  it('ships the exact tested retry and recovery guidance', () => {
    for (const name of ['WEBHOOKS', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
  });

  it('counts the initial normal queued attempt without promising extra transport retries', () => {
    expect(retryText()).toContain('Normal queued firing and recovery webhooks');
    expect(retryText()).toContain(
      'up to three queue delivery attempts, including the initial attempt',
    );
    expect(retryText()).toContain(
      'not a promise to retry every failure or a guarantee of delivery',
    );
    expect(retryText()).toContain('queued webhook sender has no extra transport retry loop');
    expect(retryText()).toContain(
      "do not assume three extra HTTP retries or a provider's Retry-After wait on this path",
    );
  });

  it('separates terminal request failures, temporary HTTP exceptions and TLS repair', () => {
    const table = [...article('WEBHOOKS').querySelectorAll('table')].find((element) =>
      text(element).startsWith('Observed failure Automatic queue behaviour'),
    );
    expect(table, 'retry policy must be a readable two-column table').toBeDefined();
    const rows = [...table!.querySelectorAll('tbody tr')].map((row) =>
      [...row.querySelectorAll('td')].map(text),
    );
    expect(rows).toEqual([
      [
        'Authentication, configuration or rejected request (most HTTP 4xx, including 400, 401 and 403)',
        'Stops as soon as this failure is classified, even with attempts left; retains the delivery as a terminal failure.',
      ],
      [
        'HTTP 408, 421, 423, 425 or 429; HTTP 5xx; connectivity or unknown failure',
        'Can retry with backoff while the saved attempt budget remains; exhaustion retains a terminal failure.',
      ],
      [
        'TLS failure',
        'Can retry within the saved budget, but retrying does not repair certificate trust, expiry or hostname errors. Do not disable verification.',
      ],
    ]);
  });

  it('distinguishes terminal failure from success, history removal and held delivery', () => {
    expect(retryText()).toContain('A terminal failure means no further automatic retry');
    expect(retryText()).toContain(
      'not that the delivery was successful or its history was deleted',
    );
    expect(retryText()).toContain('destination, timestamp, failure class and HTTP status');
    expect(retryText()).toContain('A pending or held delivery is not a terminal failure');
    expect(retryText()).toContain(
      'There is no fixed delivery deadline promised by the attempt count',
    );
  });

  it('does not let a settings Test or blind batch retry stand in for saved delivery acceptance', () => {
    expect(retryText()).toContain('all retained terminal failures, not just one webhook');
    expect(retryText()).toContain('keeps their original destination settings');
    expect(retryText()).toContain(
      'A successful Test uses current settings and does not validate or resend those saved deliveries',
    );
    expect(retryText()).toContain('Do not repeat tests or batch retries to diagnose rate limiting');
    expect(retryText()).toContain(
      'A receiver can see a duplicate if an earlier request was accepted but Pulse did not receive its response',
    );
  });

  it('explains an unused retry budget at the recovery entry point', () => {
    expect(recoveryText()).toContain(
      'Authentication, configuration and rejected failures stop automatic retries as soon as they are classified, even with attempts left',
    );
    expect(recoveryText()).toContain(
      'A terminal failure can therefore appear without exhausting the retry budget; waiting alone will not resend it',
    );
    // Preserve the existing operator safety constraints rather than suggesting
    // that changing a credential silently rewrites an old delivery.
    expect(recoveryText()).toContain('does not replace that saved configuration');
    expect(recoveryText()).toContain('all retained terminal failures');
    expect(recoveryText()).toContain('leave the failures retained');
    expect(recoveryText()).toContain('Do not delete notification_queue.db');
  });

  it('routes recovery and retry links to actual shipped sections', () => {
    const cases = [
      [
        'WEBHOOKS',
        'TROUBLESHOOTING',
        'recover-retained-delivery-failures',
        'retained-failure recovery',
      ],
      ['TROUBLESHOOTING', 'WEBHOOKS', '-delivery-contract', 'webhook retry behaviour'],
    ];
    for (const [from, to, fragment, label] of cases) {
      const link = article(from).querySelector(`a[href="/docs/${to}#${fragment}"]`);
      expect(link?.textContent).toBe(label);
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(article(to).querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
    const link = article('WEBHOOKS').querySelector(
      'a[href="#receiver-correlation-and-deduplication"]',
    );
    expect(link).not.toBeNull();
    expect(
      article('WEBHOOKS').querySelector('#receiver-correlation-and-deduplication'),
    ).not.toBeNull();
  });
});
