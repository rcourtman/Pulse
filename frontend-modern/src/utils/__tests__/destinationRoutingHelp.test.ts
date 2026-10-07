import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { renderDocMarkdown } from '@/features/docs/docMarkdown';
import {
  buildEmailConfigPayload,
  normalizeEmailConfigFromAPI,
} from '@/features/alerts/alertDestinationsModel';
import {
  ALERT_DESTINATION_ALL_SEVERITIES_LABEL,
  ALERT_DESTINATION_CRITICAL_ONLY_LABEL,
  ALERT_DESTINATION_MINIMUM_SEVERITY_LABEL,
  ALERT_DESTINATION_WARNING_AND_CRITICAL_LABEL,
} from '@/utils/alertDestinationsPresentation';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string): string => readFileSync(path.join(root, name), 'utf8');

function section(name: string, heading: string): HTMLElement {
  const source = read(`docs/${name}.md`)
    .split(heading)[1]
    ?.split(/\n#{1,6} /)[0];
  expect(source, `missing ${heading}`).toBeDefined();
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(heading + source!, name);
  return element;
}

const routingHelp = () => section('CONFIGURATION', '### Destination severity and tag routing');
const text = (element: Element): string => element.textContent!.replace(/\s+/g, ' ').trim();

describe('existing destination severity and tag routing help', () => {
  it('uses the current severity labels and does not promise exclusive channels', () => {
    const guide = routingHelp();
    const rows = [...guide.querySelectorAll('tbody tr')].map((row) =>
      [...row.querySelectorAll('td')].map(text),
    );
    expect(text(guide)).toContain(ALERT_DESTINATION_MINIMUM_SEVERITY_LABEL);
    expect(rows).toEqual([
      [
        ALERT_DESTINATION_ALL_SEVERITIES_LABEL,
        'Informational, warning and critical alerts; other policies still apply.',
      ],
      [
        ALERT_DESTINATION_WARNING_AND_CRITICAL_LABEL,
        'Warning and critical alerts, not informational alerts.',
      ],
      [
        ALERT_DESTINATION_CRITICAL_ONLY_LABEL,
        'Critical alerts, not warning or informational alerts.',
      ],
    ]);
    expect(text(guide)).toContain('minimums, not exclusive channels');
    expect(text(guide)).toContain('There is no warning-only severity setting');
    expect(text(guide)).toContain('one alert can go to more than one destination');
    expect(text(guide)).toContain('Grouped alerts are filtered member by member');
    expect(text(guide)).toContain(
      "a critical member does not make the group's warning members eligible",
    );
  });

  it('explains combined filters, exact matching and the critical-tag exclusion example', () => {
    const guide = text(routingHelp());
    expect(guide).toContain('must match both its Minimum alert severity and Resource tag routing');
    expect(guide).toContain('An empty tag filter removes the resource-tag restriction only');
    expect(guide).toContain('every selected tag on the same alert');
    expect(guide).toContain('Match any tag requires at least one');
    expect(guide).toContain('without usable routing tags does not match a nonempty filter');
    expect(guide).toContain('exact, ignoring case and surrounding whitespace');
    expect(guide).toContain('does not interpret wildcards, prefixes or regular expressions');
    expect(guide).toContain('critical is a resource tag, not an alert-severity rule');
    expect(guide).toContain('env=prod is matched by env:prod, not just prod');
    expect(guide).toContain('A warning or critical alert carrying both tags is eligible');
    expect(guide).toContain('An informational alert with both tags is not');
    expect(guide).toContain('critical severity does not bypass tag routing');
  });

  it('keeps empty resource filters independent of severity in the actual saved email settings', () => {
    const current = normalizeEmailConfigFromAPI({
      enabled: true,
      tagFilter: ['env:prod', 'team:ops'],
      tagFilterMode: 'all',
      minimumSeverity: 'critical',
    });
    current.tagFilter = [];
    const saved = buildEmailConfigPayload(current);
    expect(saved.tagFilter).toEqual([]);
    expect(saved.minimumSeverity).toBe('critical');
    const reloaded = normalizeEmailConfigFromAPI(saved);
    expect(reloaded.tagFilter).toEqual([]);
    expect(reloaded.minimumSeverity).toBe('critical');
    expect(text(routingHelp())).toContain('It does not override minimum severity');
  });

  it('keeps Test, recovery receipts and retained settings separate from firing eligibility', () => {
    const guide = text(routingHelp());
    expect(guide).toContain('A successful Test does not exercise ordinary alert routing');
    expect(guide).toContain('instead of creating an outage, removing filters');
    expect(guide).toContain('Missing delivery is not evidence that the workload is healthy');
    expect(guide).toContain('successful firing receipt for the same occurrence and destination');
    expect(guide).toContain('rather than reapplying changed tags or minimum severity');
    expect(guide).toContain('Retained queued work also keeps its saved destination settings');
  });

  it('ships identical guides with navigable diagnosis, receipt and recovery links', () => {
    for (const name of ['CONFIGURATION', 'WEBHOOKS', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
    const diagnosis = section('TROUBLESHOOTING', '#### Test succeeds but real alerts are missing');
    const contract = section('WEBHOOKS', '## 📦 Delivery Contract');
    const guide = routingHelp();
    for (const from of [diagnosis, contract]) {
      const link = from.querySelector(
        'a[href="/docs/CONFIGURATION#destination-severity-and-tag-routing"]',
      );
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
    }
    expect(guide.querySelector('#destination-severity-and-tag-routing')).not.toBeNull();
    expect(text(contract)).not.toContain('An empty filter receives every alert');
    expect(text(contract)).toContain('removes only the tag restriction, not minimum severity');
    expect(text(diagnosis)).toContain('Both must match');
    expect(text(diagnosis)).toContain('critical severity does not bypass a nonempty tag filter');
    for (const [href, target, heading] of [
      [
        '#alert-reminders-and-recovery-notifications',
        'CONFIGURATION',
        '### Alert reminders and recovery notifications',
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
      expect(guide.querySelector(`a[href="${href}"]`)).not.toBeNull();
      expect(section(target, heading).querySelector(href.slice(href.indexOf('#')))).not.toBeNull();
    }
  });
});
