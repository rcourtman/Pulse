import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';
import {
  getAlertResourceMetricDisplayValue,
  isAlertResourceMetricOff,
} from '@/components/Alerts/alertResourceTableModel';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const source = readFileSync(path.join(repoRoot, 'docs/CONFIGURATION.md'), 'utf8');

function article() {
  const section = source
    .split('### Metric thresholds, Off and inheritance')[1]
    ?.split('### VM and container powered-off tolerance')[0];
  expect(section, 'metric threshold help must be present').toBeDefined();
  const element = document.createElement('article');
  element.innerHTML = renderDocMarkdown(section!, 'CONFIGURATION');
  return element;
}

describe('existing metric threshold help', () => {
  it('distinguishes a per-VM memory alert override from hiding or ignoring a reading', () => {
    const text = article().textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('Metric Off is not a metric-hiding control');
    expect(text).toContain('The reading remains visible');
    expect(text).toContain('does not mark it ignored or N/A');
    expect(text).toContain("turn off only that VM's Memory metric rule");
    expect(text).toContain('Keep its CPU, disk, backup and uptime monitoring in place');
    expect(text).toContain('do not disable the whole resource or global alerts');
    expect(text).toContain('alone does not establish guest memory pressure or an appliance fault');
    expect(text).toContain('Do not install an agent, restart the VM or change its configuration');
    expect(text).toContain('no per-VM control here to replace one metric');

    const resource = {
      id: 'appliance-vm',
      name: 'Appliance VM',
      thresholds: { memory: -1 },
      defaults: { memory: 85, cpu: 80 },
    };
    expect(
      isAlertResourceMetricOff(getAlertResourceMetricDisplayValue(resource, 'memory', {}, false)),
    ).toBe(true);
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu', {}, false)).toBe(80);
    expect(article().querySelector('pre')).toBeNull();
  });

  it('renders Off and inheritance separately, matching the live editor model', () => {
    const rows = [...article().querySelectorAll('tbody tr')].map((row) =>
      row.textContent?.replace(/\s+/g, ' '),
    );
    expect(rows).toHaveLength(4);
    expect(rows[1]).toContain('Disables that metric rule, not collection');
    expect(rows[2]).toContain('Inherits the group default; it does not mean Off');
    expect(rows[3]).toContain('Zero does not mean “alert on any usage”');
    for (const off of [0, -1]) {
      expect(isAlertResourceMetricOff(off)).toBe(true);
    }
    const resource = {
      id: 'help-vm',
      name: 'Help VM',
      thresholds: { cpu: 95 },
      defaults: { cpu: 80 },
    };
    expect(getAlertResourceMetricDisplayValue(resource, 'cpu', { cpu: undefined }, true)).toBe(80);
    expect(
      isAlertResourceMetricOff(
        getAlertResourceMetricDisplayValue(
          { ...resource, defaults: { cpu: 0 } },
          'cpu',
          { cpu: undefined },
          true,
        ),
      ),
    ).toBe(true);
  });

  it('renders inclusive hysteresis boundaries without promising immediate delivery', () => {
    const element = article();
    const text = element.textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('90% or higher');
    expect(text).toContain('above 80%, even at 85%');
    expect(text).toContain('80% or lower');
    expect(text).toContain('evaluation windows and activation/recovery delays still apply');
    expect(text).toContain('not measured recovery');
    expect(text).toContain('Missing or stale readings are not a healthy zero');
    expect(text).toContain('do not lower thresholds, create load or stop a workload');
    expect(text).toContain('zero powered-off tolerance below means immediate eligibility');
    expect(element.querySelector('pre')).toBeNull();
  });

  it('keeps save readback and both safe delivery alternatives usable in bundled help', () => {
    const alerts = source.split('## 🔔 Alerts (`alerts.json`)')[1];
    expect(alerts).toContain('use **Save Changes**');
    expect(alerts).toContain('Reload after a successful save');
    const element = article();
    expect(element.querySelector('a[href="#quiet-hours-and-notification-holds"]')).not.toBeNull();
    expect(
      element.querySelector(
        'a[href="/docs/TROUBLESHOOTING#test-succeeds-but-real-alerts-are-missing"]',
      ),
    ).not.toBeNull();
    expect(
      readFileSync(path.join(repoRoot, 'frontend-modern/public/docs/CONFIGURATION.md'), 'utf8'),
    ).toBe(source);
  });
});
