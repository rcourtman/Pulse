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
const sectionArticle = (name: string, start: string, end: string): HTMLElement => {
  const source = read(`docs/${name}.md`);
  const startAt = source.indexOf(start);
  const endAt = source.indexOf(end, startAt + start.length);
  expect(startAt, 'linked section must exist in the actual shipped guide').toBeGreaterThan(-1);
  expect(endAt).toBeGreaterThan(startAt);
  const element = document.createElement('article');
  // Keep the real heading as well as its contents, so target IDs still come
  // from the production Markdown renderer; unrelated long recipes are not
  // part of this link contract and need not be rendered in this case.
  element.innerHTML = renderDocMarkdown(source.slice(startAt, endAt), name);
  return element;
};
const quietHoursText = (): string => {
  const guide = read('docs/CONFIGURATION.md');
  const section = guide.split('### Quiet hours and notification holds')[1]?.split('<details>')[0];
  expect(section, 'quiet-hours setup instructions must accompany the copied JSON').toBeDefined();
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(section!, 'CONFIGURATION');
  return element.textContent!.replace(/\s+/g, ' ').trim();
};

describe('quiet-hours setup help', () => {
  it('ships both exact guides and an expandable, complete JSON example', () => {
    for (const name of ['CONFIGURATION', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
    const details = [...article('CONFIGURATION').querySelectorAll('details')].find((element) =>
      element
        .querySelector('summary')
        ?.textContent?.includes('Manual Configuration (JSON excerpt)'),
    );
    expect(details).toBeDefined();
    const example = JSON.parse(details!.querySelector('code')!.textContent!);
    expect(example.schedule.quietHours).toEqual({
      enabled: true,
      start: '22:00',
      end: '06:00',
      timezone: 'Europe/London',
      days: {
        monday: true,
        tuesday: true,
        wednesday: true,
        thursday: true,
        friday: true,
        saturday: true,
        sunday: true,
      },
      suppress: { performance: false, storage: false, offline: false },
    });
  });

  it('explains day, timezone and midnight boundaries rather than enabled alone', () => {
    const text = quietHoursText();
    expect(text).toContain('Alerts → Schedule → Quiet hours');
    expect(text).toContain('No selected days means no quiet period, even when enabled');
    expect(text).toContain("not your browser's timezone");
    expect(text).toContain('current local calendar day');
    expect(text).toContain("not Tuesday's early morning");
    expect(text).toContain('Select Tuesday too');
    expect(text).toContain('06:00:59');
  });

  it('keeps critical delivery, monitoring and current policy separate from held notifications', () => {
    const text = quietHoursText();
    expect(text).toContain('hold non-critical notifications');
    expect(text).toContain('do not stop monitoring, clear the alert or confirm recovery');
    expect(text).toContain('Critical notifications remain eligible unless');
    expect(text).toContain('including urgent failures');
    expect(text).toContain('not a promise to send every held item at that instant');
    expect(text).toContain('a settings Test skips the queue and does not validate this schedule');
    expect(text).toContain('Do not create an outage');
    expect(text).toContain('off by default');
    expect(text).toContain('not a complete alerts.json');
    expect(text).toContain('preserve existing rules and settings');
  });

  it('connects troubleshooting and setup through real shipped section links', () => {
    const guides: Record<string, HTMLElement> = {
      CONFIGURATION: sectionArticle(
        'CONFIGURATION',
        '### Quiet hours and notification holds',
        '## Availability Checks',
      ),
      TROUBLESHOOTING: sectionArticle(
        'TROUBLESHOOTING',
        '#### Test succeeds but real alerts are missing',
        '#### Recover retained delivery failures',
      ),
    };
    for (const [from, to, fragment, label] of [
      [
        'TROUBLESHOOTING',
        'CONFIGURATION',
        'quiet-hours-and-notification-holds',
        'quiet-hours schedule',
      ],
      [
        'CONFIGURATION',
        'TROUBLESHOOTING',
        'test-succeeds-but-real-alerts-are-missing',
        'Recent delivery activity',
      ],
    ]) {
      const link = guides[from].querySelector(`a[href="/docs/${to}#${fragment}"]`);
      expect(link?.textContent).toBe(label);
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(guides[to].querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
  });
});
