import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';
import { getHistoryReadoutLabel } from '@/components/shared/historyChartModel';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const guide = read('docs/METRICS_HISTORY.md');
const render = (markdown: string, name: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
};
const article = () => render(guide, 'METRICS_HISTORY');
const interpretation = () =>
  render(guide.split('## Read a History panel')[1].split('\n## ')[0], 'METRICS_HISTORY');
const text = (element: Element) => element.textContent?.replace(/\s+/g, ' ');

describe('History interpretation help', () => {
  it('ships both changed guides and puts safe interpretation before storage tuning', () => {
    for (const name of ['METRICS_HISTORY', 'TROUBLESHOOTING']) {
      expect(read(`frontend-modern/public/docs/${name}.md`)).toBe(read(`docs/${name}.md`));
    }
    const headings = [...article().querySelectorAll('h2')].map((heading) => heading.id);
    expect(headings.slice(0, 3)).toEqual([
      'read-a-history-panel',
      'retention-and-accessible-range',
      'storage-location',
    ]);
    expect(interpretation().querySelector('pre')).toBeNull();
  });

  it('explains real drawer labels without treating older or fallback readings as current', () => {
    const rows = [...interpretation().querySelectorAll('tbody tr')].map((row) => text(row));
    expect(rows).toHaveLength(7);
    expect(rows[0]).toContain("Not necessarily now, every metric's newest value");
    expect(rows[1]).toContain('Not a live reading or an interpolated observation');
    expect(rows[2]).toContain('Not proof that the value was persisted');
    expect(rows[3]).toContain('Not a flat trend or sustained recovery');
    expect(rows[4]).toContain('Not the cause or duration of a monitoring outage');
    expect(rows[5]).toContain('Shaded low–high band');
    expect(rows[5]).toContain(
      'Not continuous coverage, every original sample, or the duration of a spike',
    );
    expect(rows[6]).toContain('peak while inspecting');
    expect(rows[6]).toContain(
      "Not necessarily the highest value in the whole selected range or the value at an alert's exact time",
    );
    const renderer = read('frontend-modern/src/components/Workloads/GuestDrawerHistory.tsx');
    expect(renderer).toContain('getHistoryReadoutLabel(props.source, activeTimestamp() !== null)');
    expect(getHistoryReadoutLabel('store', false)).toBe('Latest stored');
    expect(getHistoryReadoutLabel('store', true)).toBe('Stored');
    for (const label of ['Latest stored', 'Stored']) {
      expect(text(interpretation())).toContain(label);
    }
    for (const label of ['Inspect history', 'Single observation. No trend yet.']) {
      expect(renderer).toContain(label);
      expect(text(interpretation())).toContain(label);
    }
    expect(text(interpretation())).toContain('Other charts and older Pulse versions may differ');
    expect(text(interpretation())).toContain('keep it unknown rather than borrowing one');
  });

  it('keeps same-time values, missing observations, panel-local times and access to earlier points distinct', () => {
    expect(text(interpretation())).toContain(
      'CPU 22% at 12:02 and memory 95% at 12:01 are not one observation',
    );
    expect(text(interpretation())).toContain('the earlier 95% remains inspectable');
    expect(text(interpretation())).toContain('Network and disk I/O panels have their own times');
    expect(text(interpretation())).toContain('Missing is not measured zero');
    const steps = [...interpretation().querySelectorAll('ol > li')].map((step) => text(step));
    expect(steps).toHaveLength(3);
    expect(steps[0]).toContain('arrow keys, Home or End, or move it by touch');
    expect(steps[1]).toContain('aggregated or downsampled');
    expect(steps[2]).toContain("Today's source cannot identify the source of a past warning");
  });

  it('links from the existing memory workflow to working interpretation and safety headings', () => {
    const troubleshooting = render(read('docs/TROUBLESHOOTING.md'), 'TROUBLESHOOTING');
    const href = '/docs/METRICS_HISTORY#read-a-history-panel';
    expect(troubleshooting.querySelector(`a[href="${href}"]`)?.textContent).toBe(
      'History panel interpretation',
    );
    expect(article().querySelector('#read-a-history-panel')).not.toBeNull();
    const safety = '/docs/VM_DISK_MONITORING#backup-safety';
    expect(interpretation().querySelector(`a[href="${safety}"]`)?.textContent).toBe(
      'Backup safety',
    );
    expect(
      render(read('docs/VM_DISK_MONITORING.md'), 'VM_DISK_MONITORING').querySelector(
        '#backup-safety',
      ),
    ).not.toBeNull();
    expect(text(interpretation())).toContain('do not run Diagnostics or guest-agent probes');
    expect(text(interpretation())).toContain('clear History or change retention');
    expect(text(interpretation())).toContain(
      'do not share raw responses, credentials or a full network export',
    );
  });

  it('separates storage from runtime access and does not sell the retired Relay compatibility tier', () => {
    const section = guide.split('## Retention and accessible range')[1].split('\n## ')[0];
    const rendered = render(section, 'METRICS_HISTORY');
    expect(text(rendered)).toContain('Community supports up to 7 days');
    expect(text(rendered)).toContain('Pro and legacy Pro+ entitlements support up to 90 days');
    expect(text(rendered)).toContain('Relay is no longer sold');
    expect(text(rendered)).toContain('Continuing Relay subscribers receive Pro features');
    expect(text(rendered)).toContain('14-day compatibility limit');
    expect(text(rendered)).toContain('not a plan to buy');
    expect(text(rendered)).toContain('402 Payment Required, not an empty history');
    expect(text(rendered)).toContain('do not buy another licence, reset history or reinstall');
    expect(text(rendered)).toContain('cannot recover observations that were never stored');
    expect(guide).not.toContain('Relay unlocks');
    const link = rendered.querySelector('a[href="/docs/PULSE_PRO#self-hosted-commercial-model"]');
    expect(link?.textContent).toBe('plans and legacy subscriptions');
    expect(
      render(read('docs/PULSE_PRO.md'), 'PULSE_PRO').querySelector('#self-hosted-commercial-model'),
    ).not.toBeNull();
    const limits = read('pkg/licensing/features.go');
    expect(limits).toMatch(/TierFree:\s+7,/);
    expect(limits).toMatch(/TierRelay:\s+14,/);
    expect(limits).toMatch(/TierPro:\s+90,/);
    expect(limits).toMatch(/TierProPlus:\s+90,/);
    expect(read('frontend-modern/src/stores/license.ts')).toContain(
      'runtimeCapabilities()?.max_history_days',
    );
  });
});
