import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';
import { snoozePresetExpiry } from '@/features/alerts/AlertSnoozeAction';
import {
  getAlertOverviewAcknowledgedToggleLabel,
  getAlertOverviewPrimaryActionLabel,
  getAlertOverviewResumeLabel,
  getAlertOverviewSnoozeOptionLabel,
} from '@/utils/alertOverviewPresentation';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string): string => readFileSync(path.join(root, name), 'utf8');

function article(name: string) {
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(read(`docs/${name}.md`), name);
  return element;
}

function actions() {
  const source = read('docs/CONFIGURATION.md')
    .split('### Acknowledge and snooze existing alerts')[1]
    ?.split('### Quiet hours and notification holds')[0];
  expect(source, 'existing incident controls need an explanation of their holds').toBeDefined();
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(source!, 'CONFIGURATION');
  return element;
}

describe('existing alert incident actions help', () => {
  it('renders the actual control names and keeps notification holds separate from recovery', () => {
    const rows = [...actions().querySelectorAll('tbody tr')].map((row) =>
      row.textContent?.replace(/\s+/g, ' '),
    );
    expect(rows).toHaveLength(4);
    expect(rows[0]).toContain(
      getAlertOverviewPrimaryActionLabel({ acknowledged: false, processing: false }),
    );
    expect(rows[0]).toContain('firing notifications, escalation and its recovery notification');
    expect(rows[0]).toContain('underlying alert can remain active');
    expect(rows[1]).toContain(
      getAlertOverviewPrimaryActionLabel({ acknowledged: true, processing: false }),
    );
    expect(rows[1]).toContain('not a guaranteed immediate resend');
    expect(rows[2]).toContain('including critical notifications');
    expect(rows[3]).toContain(getAlertOverviewResumeLabel());
    expect(rows[3]).toContain('does not remove an acknowledgement');
  });

  it('explains hidden incidents and bulk scope rather than an all-clear', () => {
    const text = actions().textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('neither action repairs the workload or confirms recovery');
    expect(text).toContain(
      'hidden from the default active list and excluded from its Active count',
    );
    expect(text).toContain(getAlertOverviewAcknowledgedToggleLabel(false));
    expect(text).toContain('does not mean every workload recovered');
    expect(text).toContain('toolbar applies to all unacknowledged active alerts');
    expect(text).toContain('including its collapsed related alerts');
    expect(text).toContain('Use the individual action when only one incident has been reviewed');
  });

  it('binds tomorrow morning to the real local-calendar preset and preserves separate holds', () => {
    const text = actions().textContent!.replace(/\s+/g, ' ');
    expect(text).toContain(getAlertOverviewSnoozeOptionLabel('tomorrowMorning'));
    expect(text).toContain("browser's local timezone, not the quiet-hours timezone");
    // Deliberately use local constructors: the preset uses the browser's date,
    // not a UTC duration or the alert schedule's independently configured zone.
    const expiry = snoozePresetExpiry('tomorrowMorning', new Date(2026, 9, 7, 22, 30));
    expect(expiry.getFullYear()).toBe(2026);
    expect(expiry.getMonth()).toBe(9);
    expect(expiry.getDate()).toBe(8);
    expect(expiry.getHours()).toBe(9);
    expect(expiry.getMinutes()).toBe(0);
    expect(text).toContain('an acknowledged incident remains acknowledged');
    expect(text).toContain('not proof of delivery or a promise to replay missed notifications');
    expect(text).toContain('They do not retry a failed notification');
    expect(text).toContain('Do not create an outage or lower thresholds');
  });

  it('ships exact guidance with working diagnosis and schedule links', () => {
    for (const name of ['CONFIGURATION', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
    // Render each immutable guide once instead of reparsing it for every link.
    const guides: Record<string, HTMLElement> = {
      CONFIGURATION: article('CONFIGURATION'),
      TROUBLESHOOTING: article('TROUBLESHOOTING'),
    };
    for (const [from, to, fragment, label] of [
      [
        'TROUBLESHOOTING',
        'CONFIGURATION',
        'acknowledge-and-snooze-existing-alerts',
        'acknowledgement or snooze',
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
    expect(actions().querySelector('a[href="#quiet-hours-and-notification-holds"]')).not.toBeNull();
    expect(
      guides.CONFIGURATION.querySelector('#quiet-hours-and-notification-holds'),
    ).not.toBeNull();
  });
});
