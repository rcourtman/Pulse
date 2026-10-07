import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const guide = (): string =>
  readFileSync(
    process.env.PULSE_WEBHOOK_CLOSE_HELP_TEST_INPUT || path.join(root, 'docs/WEBHOOKS.md'),
    'utf8',
  );
const article = (): HTMLElement => {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(guide(), 'WEBHOOKS');
  return element;
};
const closeText = (): string => {
  const section = guide().split('### Resolved does not always mean recovered')[1]?.split('### ')[0];
  expect(
    section,
    'non-recovery guidance must be beside the copied integration contract',
  ).toBeDefined();
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(section!, 'WEBHOOKS');
  return element.textContent!.replace(/\s+/g, ' ').trim();
};

describe('webhook non-recovery close help', () => {
  it('keeps unsupported stable versions and failed renders out of blind batch retries', () => {
    const text = article().textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('v6.5.0 does not have these fields');
    expect(text).toContain('Consult the help bundled with your installed version');
    expect(text).toContain('leave failed deliveries retained rather than retrying the batch');
    expect(text).toContain('removing the reason fields and treating every close as recovery');
    expect(text).toContain(
      'A newer guide on the website does not establish that its supporting software has been released',
    );
  });

  it('ships the exact tested guidance and member-aware copied template', () => {
    expect(readFileSync(path.join(root, 'frontend-modern/public/docs/WEBHOOKS.md'), 'utf8')).toBe(
      guide(),
    );
    const codes = [...article().querySelectorAll('pre code')];
    const payload = codes.find((code) => code.textContent!.includes('"alerts": ['));
    expect(
      payload,
      'the full PSA template must survive the shipped Markdown renderer',
    ).toBeDefined();
    for (const field of ['resolutionReason', 'successorResourceId', 'successorName']) {
      expect(payload!.textContent).toContain(`"${field}"`);
    }
    expect(payload!.textContent).toContain('"notRecovered": {{.NotRecovered}}');
    expect(payload!.textContent).toContain('{{with .Resolution}}{{.Reason | jsonString}}{{end}}');
  });

  it('distinguishes an agent handover from measured recovery and a successor alert', () => {
    const text = closeText();
    expect(text).toContain('moved_to_agent even while its last reading is above the threshold');
    expect(text).toContain('this is not a recovery');
    expect(text).toContain('Record it as moved, not healthy');
    expect(text).toContain(
      "check the successor agent's current reading and alert policy separately",
    );
    expect(text).toContain(
      'Do not assume that the successor has already fired an equivalent alert',
    );
  });

  it('keeps mixed, unknown and older payloads from becoming blanket health verdicts', () => {
    const text = closeText();
    expect(text).toContain('Use the reason on each member');
    expect(text).toContain('A group can contain both moved alerts and ordinary recoveries');
    expect(text).toContain('false is not an independent health verdict');
    expect(text).toContain(
      'unknown non-empty reason or missing fields from an older/custom payload',
    );
    expect(text).toContain('do not silently map them to healthy');
    expect(text).toContain(
      'An absent successor identity does not justify guessing from a display name',
    );
  });

  it('does not reinterpret firing events, empty reasons or old member messages as health proof', () => {
    const text = closeText();
    expect(text).toContain('not proof that every metric or workload is healthy');
    expect(text).toContain('On a firing event they are not recovery evidence at all');
    const fullText = article().textContent!.replace(/\s+/g, ' ');
    expect(fullText).toContain('Member summary retains the original alert message');
    expect(fullText).toContain(
      'use the structured reason rather than parsing that text for recovery',
    );
    expect(fullText).toContain('old delayed recovery must not close a newer incident');
    expect(fullText).toContain('Close a moved occurrence as moved, not recovered');
  });

  it('connects the event contract, receiver and bridge entry points to the rendered guidance', () => {
    const doc = article();
    expect(doc.querySelector('#resolved-does-not-always-mean-recovered')).not.toBeNull();
    const labels = [
      ...doc.querySelectorAll('a[href="#resolved-does-not-always-mean-recovered"]'),
    ].map((link) => link.textContent);
    expect(labels).toEqual(['non-recovery closes', 'resolution reason', 'resolution reason']);
    expect(doc.querySelector('a[href="#sample-psa-payloads"]')?.textContent).toBe(
      'full PSA payload',
    );
    expect(doc.querySelector('#sample-psa-payloads')).not.toBeNull();
  });
});
