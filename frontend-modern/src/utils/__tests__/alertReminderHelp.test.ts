import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';
import {
  buildAlertsConfigurationPayload,
  createDefaultAlertsConfigurationSnapshot,
  readAlertsConfigurationSnapshot,
} from '@/features/alerts/alertsConfigurationModel';
import {
  ALERT_CONFIG_COOLDOWN_TITLE,
  ALERT_CONFIG_COOLDOWN_PERIOD_LABEL,
  ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL,
  ALERT_CONFIG_RECOVERY_TITLE,
} from '@/utils/alertConfigPresentation';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string): string => readFileSync(path.join(root, name), 'utf8');

function section(name: string, heading: string) {
  const source = read(`docs/${name}.md`)
    .split(heading)[1]
    ?.split(/\n#{1,6} /)[0];
  expect(source, `missing ${heading}`).toBeDefined();
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(heading + source!, name);
  return element;
}

const reminderHelp = () =>
  section('CONFIGURATION', '### Alert reminders and recovery notifications');
const text = (element: HTMLElement) => element.textContent!.replace(/\s+/g, ' ');

describe('existing alert reminders and recovery help', () => {
  it('distinguishes first delivery, minimum reminder intervals and once-per-occurrence off', () => {
    const guide = reminderHelp();
    const rows = [...guide.querySelectorAll('tbody tr')].map((row) => text(row as HTMLElement));
    expect(rows).toHaveLength(5);
    expect(rows[0]).toContain(`${ALERT_CONFIG_COOLDOWN_TITLE} on`);
    expect(rows[0]).toContain(ALERT_CONFIG_COOLDOWN_PERIOD_LABEL);
    expect(rows[0]).toContain('It does not delay the first eligible notification');
    expect(rows[1]).toContain('Stops ordinary reminders for the same alert occurrence');
    expect(rows[1]).toContain('Off does not mean “send on every poll” or “no rate limit”');
    expect(rows[1]).toContain('does not turn off monitoring or initial delivery');
    expect(text(guide)).toContain('not eligible for an ordinary reminder before 10:30');
    expect(text(guide)).toContain('not a promise of a message at 10:30');
    expect(text(guide)).toContain('cooldown off is not a way to silence them');
  });

  it('binds off and the independent hourly limit to the actual saved and reloaded payload', () => {
    const snapshot = createDefaultAlertsConfigurationSnapshot();
    snapshot.scheduleCooldown = { enabled: false, minutes: 30, maxAlerts: 3 };
    const saved = buildAlertsConfigurationPayload({
      snapshot,
      rawOverridesConfig: {},
      alertsActivationState: 'active',
      alertsActivationConfig: { enabled: true },
    });
    expect(saved.alertConfig?.schedule?.cooldown).toBe(0);
    expect(saved.alertConfig?.schedule?.maxAlertsHour).toBe(3);
    const loaded = readAlertsConfigurationSnapshot(saved.alertConfig!);
    expect(loaded.scheduleCooldown.enabled).toBe(false);
    expect(loaded.scheduleCooldown.maxAlerts).toBe(3);
    const guide = text(reminderHelp());
    expect(guide).toContain(ALERT_CONFIG_COOLDOWN_MAX_ALERTS_LABEL);
    expect(guide).toContain('over a rolling hour, not across the whole installation');
    expect(guide).toContain('retains this separate limit; it does not remove it');
    expect(guide).toContain('hourly limit is not a count of provider requests');
  });

  it('keeps recovery opt-in separate from occurrence, destination and human receipt evidence', () => {
    const guide = text(reminderHelp());
    expect(guide).toContain(`${ALERT_CONFIG_RECOVERY_TITLE} on`);
    expect(guide).toContain('recorded successful firing delivery for that same alert occurrence');
    expect(guide).toContain('does not send an all-clear to every configured destination');
    expect(guide).toContain('Stops recovery messages, not detection of recovery');
    expect(guide).toContain('Recovery follows the configured grouping window too');
    expect(guide).toContain('provider acceptance, not that a person read the message');
    expect(guide).toContain('enabling the toggle does not bypass those controls');
    expect(guide).toContain(
      'A missing reminder or recovery message is not a workload health check',
    );
    expect(guide).toContain('Do not lower thresholds, create load or stop a workload');
  });

  it('ships exact mirrors and navigable diagnosis, holds and retained-failure links', () => {
    for (const name of ['CONFIGURATION', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
    // Only render the linked sections: unrelated large recipes are not part of
    // these link assertions, and repeated whole-guide parsing obscures failures.
    const diagnosis = section('TROUBLESHOOTING', '#### Test succeeds but real alerts are missing');
    const reminder = reminderHelp();
    const backlink = diagnosis.querySelector(
      'a[href="/docs/CONFIGURATION#alert-reminders-and-recovery-notifications"]',
    );
    expect(backlink?.textContent).toBe('alert reminders and recovery notifications');
    expect(backlink?.hasAttribute('data-doc-link')).toBe(true);
    expect(reminder.querySelector('#alert-reminders-and-recovery-notifications')).not.toBeNull();
    expect(text(diagnosis)).toContain(
      'Their absence does not establish whether the workload recovered',
    );
    for (const [href, target, heading] of [
      [
        '#acknowledge-and-snooze-existing-alerts',
        'CONFIGURATION',
        '### Acknowledge and snooze existing alerts',
      ],
      [
        '#quiet-hours-and-notification-holds',
        'CONFIGURATION',
        '### Quiet hours and notification holds',
      ],
      [
        '/docs/TROUBLESHOOTING#test-succeeds-but-real-alerts-are-missing',
        'TROUBLESHOOTING',
        '#### Test succeeds but real alerts are missing',
      ],
      [
        '/docs/TROUBLESHOOTING#recover-retained-delivery-failures',
        'TROUBLESHOOTING',
        '#### Recover retained delivery failures',
      ],
    ]) {
      expect(reminder.querySelector(`a[href="${href}"]`)).not.toBeNull();
      expect(section(target, heading).querySelector(href.slice(href.indexOf('#')))).not.toBeNull();
    }
  });
});
