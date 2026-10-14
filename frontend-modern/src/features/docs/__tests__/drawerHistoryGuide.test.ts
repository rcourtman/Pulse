import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const shipped = (name: string) => read(`frontend-modern/public/docs/${name}.md`);
const render = createGuideFixture(shipped);
const prose = (element: Element) => (element.textContent ?? '').replace(/\s+/g, ' ').trim();

function section(name: string, fragment: string) {
  const heading = render(name).querySelector(`#${fragment}`);
  expect(heading).not.toBeNull();
  const element = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (next.matches('h1, h2, h3')) break;
    element.appendChild(next.cloneNode(true));
  }
  return element;
}

const pbs = () => section('PBS', 'pbs-is-connected-but-history-stays-empty');
const history = () => section('METRICS_HISTORY', 'read-a-history-panel');

describe('stored-chart navigation and interpretation help', () => {
  it.each(['PBS', 'METRICS_HISTORY'])('ships the exact canonical %s guide', (name) => {
    expect(shipped(name)).toBe(read(`docs/${name}.md`));
  });

  it('routes PBS to its host Overview, not its datastore, VM or alert history', () => {
    const text = prose(pbs());
    expect(text).toContain('Proxmox → Backups, choose the Backup Server row');
    expect(text).toContain("drawer's Overview charts and History range selector");
    expect(text).toContain('not a datastore chart, Alerts → History');
    expect(text).toContain("PBS virtual machine's row in Proxmox Overview");
    expect(text).not.toContain('Backup Server → History');
    expect(text).toContain('resourceType, resourceId and range');
    expect(text).toContain('a request without metric is expected');
    expect(text).toContain('metrics.cpu');
    expect(text).toContain('points array for that single metric');
  });

  it('supports older installed tabs without making an upgrade part of diagnosis', () => {
    expect(prose(pbs())).toContain('If your installed version still has a separate History tab');
    expect(prose(pbs())).toContain('no upgrade or restart is needed just to find the chart');
    expect(prose(history())).toContain('If your installed version still has a History tab');
    expect(prose(history())).toContain('do not upgrade or restart just to follow this guide');
    expect(prose(history())).toContain(
      'A resource without a supported history target may have no chart',
    );
  });

  it('binds the new instructions to the current consuming drawer and control', () => {
    const overview = read(
      'frontend-modern/src/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx',
    );
    expect(overview).toContain('<GuestDrawerHistory');
    expect(overview).toContain("enabled={drawer.activeTab() === 'overview'}");
    expect(overview).toContain('onRangeChange={drawer.setMetricsHistoryRange}');
    const panel = read('frontend-modern/src/components/Workloads/GuestDrawerHistory.tsx');
    expect(panel).toContain('label="History range"');
    expect(panel).toContain('point.min ?? point.value');
    expect(panel).toContain('point.max ?? point.value');
    expect(panel).toContain('average, ${low} to ${high}');
  });

  it('explains stored buckets and peaks without inventing coverage or an incident time', () => {
    const text = prose(history());
    expect(text).toContain('22% average, 10% to 95%');
    expect(text).toContain('does not rule out the recorded 95% peak');
    expect(text).toContain('does not say how long that peak lasted');
    expect(text).toContain('does not prove that usage stayed constant');
    const rows = [...history().querySelectorAll('tbody tr')];
    const band = rows.find((row) => prose(row).startsWith('Shaded low–high band'))!;
    const peak = rows.find((row) => prose(row).startsWith('peak while inspecting'))!;
    expect(prose(band)).toContain('Not continuous coverage, every original sample');
    expect(prose(peak)).toContain('Not necessarily the highest value');
    expect(prose(peak)).toContain("alert's exact time");
    expect(text).toContain('Missing is not measured zero');
    expect(text).toContain('Window start and Window end bound the requested interval');
    expect(text).toContain('not the first and last successful observations');
  });

  it('keeps both existing bookmarks and the new cross-guide links usable', () => {
    for (const [element, name, fragment] of [
      [pbs(), 'METRICS_HISTORY', 'read-a-history-panel'],
      [history(), 'PBS', 'pbs-is-connected-but-history-stays-empty'],
    ] as const) {
      expect(element.querySelector(`a[href="/docs/${name}#${fragment}"]`)).not.toBeNull();
      expect(render(name).querySelector(`#${fragment}`)).not.toBeNull();
    }
  });

  it('retains the safe no-replay and private-evidence investigation path', () => {
    expect(prose(pbs())).toContain('Do not edit or replay the request');
    expect(prose(pbs())).toContain('Do not refresh or restart the server');
    expect(prose(pbs())).toContain('Do not share a HAR export');
    expect(prose(pbs())).toContain('Do not delete and re-add connections');
    expect(prose(history())).toContain('do not run Diagnostics or guest-agent probes');
    expect(prose(history())).toContain('not chart inspection');
    expect(
      history().querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]'),
    ).not.toBeNull();
  });
});
