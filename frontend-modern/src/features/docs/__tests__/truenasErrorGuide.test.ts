import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');

const renderGuide = createGuideFixture((name) => {
  // An exact-parent input proves that the former advice fails these checks
  // without altering the committed source used by the proof.
  const input = process.env[`PULSE_${name}_GUIDE_TEST_INPUT`];
  return readFileSync(
    input ?? path.join(repoRoot, 'frontend-modern/public/docs', `${name}.md`),
    'utf8',
  );
});

function sectionAt(article: HTMLElement, id: string): HTMLElement {
  const heading = article.querySelector(id);
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (let next = heading!.nextElementSibling; next; next = next.nextElementSibling) {
    if (/^H[1-4]$/.test(next.tagName)) break;
    section.appendChild(next.cloneNode(true));
  }
  return section;
}

const unavailableSection = (article: HTMLElement): HTMLElement =>
  sectionAt(article, '#truenas-service-unavailable');

const prose = (section: HTMLElement): string =>
  (section.textContent ?? '').replace(/\s+/g, ' ').trim();

describe('shipped TrueNAS error diagnosis', () => {
  it.each(['TRUENAS', 'TROUBLESHOOTING'])(
    '%s routes local service errors away from appliance credentials and live tests',
    (name) => {
      const section = unavailableSection(renderGuide(name));
      const text = prose(section);
      expect(text).toContain('truenas_unavailable');
      expect(text).toContain('500');
      expect(text).toContain('503');
      expect(text).toContain('connection-management handler or configuration persistence');
      expect(text).toContain('bounded Pulse');
      expect(text).toMatch(/Do not rotate the (?:TrueNAS )?key, recreate/);
      expect(text).toContain('weaken TLS verification');
      expect(section.querySelector('pre')).toBeNull();
      expect(text).not.toContain('then use Test Connection');
      expect(text).not.toContain('Use Test Connection in Pulse.');
    },
  );

  it('keeps explicit opt-out and an actual failed probe distinct from unavailable storage', () => {
    const text = prose(unavailableSection(renderGuide('TRUENAS')));
    expect(text).toContain('truenas_disabled / HTTP 404');
    expect(text).toContain('PULSE_ENABLE_TRUENAS');
    expect(text).toContain('not an appliance outage');
    expect(text).toContain('override an intentional opt-out');
    expect(text).toContain('truenas_connection_failed / HTTP 400');
    expect(text).toContain('a live connection test failed');
    expect(text).toContain('Keep HTTPS and certificate verification enabled');
    expect(text).toContain('recognized CORE 13 systems use legacy REST');
  });

  it('uses existing browser evidence and does not require replay or public credentials', () => {
    const text = prose(unavailableSection(renderGuide('TRUENAS')));
    expect(text).toContain('existing failed request');
    expect(text).toContain('Developer tools → Network');
    expect(text).toContain('do not repeat a save, delete or test');
    expect(text).toContain('request path, status/code and a manually redacted error');
    expect(text).toContain('not the full response, configuration, key or session cookie');
    expect(text).toContain('on a separate connection, not inventory or metric collection');
    expect(text).toContain('instead of changing credentials');
  });

  it('takes both entry points to real error and polling headings in the shipped viewer', () => {
    const guide = renderGuide('TRUENAS');
    const troubleshooting = unavailableSection(renderGuide('TROUBLESHOOTING'));
    const errorLink = troubleshooting.querySelector<HTMLAnchorElement>(
      'a[href="/docs/TRUENAS#truenas-service-unavailable"]',
    );
    expect(errorLink?.textContent).toBe('TrueNAS error diagnosis');
    expect(guide.querySelector(new URL(errorLink!.href).hash)?.textContent).toBe(
      '"TrueNAS service unavailable"',
    );
    expect(
      troubleshooting.querySelector('a[href="/docs/TRUENAS#stale-truenas-data"]'),
    ).not.toBeNull();
    const pollingLink = unavailableSection(guide).querySelector('a[href="#stale-truenas-data"]');
    expect(pollingLink?.textContent).toBe('polling checks');
    expect(guide.querySelector('#stale-truenas-data')?.textContent).toBe('Stale TrueNAS data');
  });

  it('takes NAS diagnosis to the shared bounded readers, without an unsafe copied alternative', () => {
    const nas = sectionAt(renderGuide('TRUENAS'), '#no-data-appearing-after-adding-connection');
    const link = nas.querySelector<HTMLAnchorElement>(
      'a[href="/docs/TROUBLESHOOTING#inspect-notification-logs"]',
    );
    expect(link?.textContent).toBe('bounded Pulse log readers');
    expect(nas.querySelector('pre')).toBeNull();
    const target = sectionAt(renderGuide('TROUBLESHOOTING'), new URL(link!.href).hash);
    const commands = [...target.querySelectorAll('pre code')].map((node) => node.textContent!);
    expect(commands).toHaveLength(2);
    for (const command of commands) {
      // The deadline covers the entire reader/byte-limiter pipeline, including
      // readers that ignore TERM. Do not restore the old termination grace.
      expect(command).toContain('timeout --signal=KILL 8s bash -o pipefail');
      expect(command).not.toMatch(/--signal=TERM|--kill-after/);
      expect(command).toContain('head -c 262145');
      expect(command).toContain('"$pulse_log_bytes" -gt 262144');
      expect(command).toContain('excerpt exceeds 256 KiB');
      expect(command).toContain('no partial excerpt shown');
      expect(command).toContain('no unbounded fallback');
      expect(command).not.toMatch(/grep|--follow|restart|curl/);
    }
    expect(commands[0]).toContain('journalctl -u pulse');
    expect(commands[1]).toContain('docker logs');
  });

  it('keeps NAS-specific host, deadline and disclosure boundaries beside the reader link', () => {
    const text = prose(
      sectionAt(renderGuide('TRUENAS'), '#no-data-appearing-after-adding-connection'),
    );
    for (const boundary of [
      'no request ID is required',
      'inside the Pulse container',
      'not on the Proxmox host or the TrueNAS appliance',
      'eight-second hard deadline',
      '256 KiB byte limit',
      'record limit alone does not bound a hung reader',
      'do not use an unbounded substitute',
      'only after a successful read',
      'withholds partial output',
      'successful empty read is inconclusive',
      'Do not enable Debug or repeat the failing action',
      'not sanitised',
      'not the whole excerpt',
      'makes live API and guest-agent requests',
    ]) {
      expect(text).toContain(boundary);
    }
  });

  it('distinguishes CORE request rejection from credentials, inventory and measured zero', () => {
    const section = sectionAt(renderGuide('TRUENAS'), '#core-13-rejects-the-reporting-request');
    const text = prose(section);
    for (const fact of [
      'TrueNAS CORE 13',
      'POST /reporting/get_data',
      '400',
      'The following attributes are not expected: query',
      'not an invalid API key',
      'observed.collectedAt',
      'not a measured idle value',
      "Use TrueNAS's own Reporting graphs",
      'Do not rotate the key, recreate the connection, install an agent, switch transport or weaken TLS verification',
      'do not collect them again',
      'contains the relevant correction',
      'does not diagnose every CORE or SCALE problem',
    ]) {
      expect(text).toContain(fact);
    }
    expect(section.querySelector('pre, form, input, button')).toBeNull();
    expect(section.querySelector('a')?.getAttribute('href')).toBe(
      'https://github.com/rcourtman/Pulse/issues/2964',
    );
  });

  it('routes missing-telemetry readers to the specific rejection before new graph collection', () => {
    const article = renderGuide('TRUENAS');
    const missing = sectionAt(article, '#inventory-works-but-cpu-memory-or-history-is-missing');
    const link = missing.querySelector<HTMLAnchorElement>(
      'a[href="#core-13-rejects-the-reporting-request"]',
    );
    expect(link?.textContent).toBe('CORE 13 request-format rejection');
    // Fragment-only links use the browser's native anchor navigation.
    expect(link?.hasAttribute('data-doc-link')).toBe(false);
    expect(article.querySelector(link!.getAttribute('href')!)?.tagName).toBe('H3');
    expect(prose(missing)).toContain('rather than collecting another graph response');
    expect(readFileSync(path.join(repoRoot, 'docs/TRUENAS.md'), 'utf8')).toBe(
      readFileSync(path.join(repoRoot, 'frontend-modern/public/docs/TRUENAS.md'), 'utf8'),
    );
  });

  it('links from system inventory to the existing touch and keyboard memory breakdown', () => {
    const article = renderGuide('TRUENAS');
    const link = article.querySelector<HTMLAnchorElement>(
      'table a[href="#memory-usage-and-arc-cache"]',
    );
    expect(link?.textContent).toBe('memory and ARC cache');
    expect(prose(link!.closest('tr')!)).toContain('System info');
    const anchor = link!.getAttribute('href')!;
    expect(article.querySelector(anchor)?.textContent).toBe('Memory usage and ARC cache');

    const section = sectionAt(article, anchor);
    const text = prose(section);
    expect(text).toContain('TrueNAS → Overview');
    expect(text).toContain('Click or tap its Memory bar (Mem on a narrow screen)');
    expect(text).toContain('Memory composition for your appliance, then press Enter');
    expect(text).toContain('Close returns you to the bar');
    expect(text).toContain("you don't need a Pulse agent");
    expect(text).toContain('separate from the reading for its VM on the Proxmox page');
    expect(text).toContain('use Reporting in TrueNAS itself');
    expect(text).toContain('A missing cache row does not mean ARC is zero');
    expect(section.querySelector('pre, form, input, button')).toBeNull();
  });

  it('takes readers from the pool row to a read-only comparison of missing capacity', () => {
    const article = renderGuide('TRUENAS');
    const pool = [...article.querySelectorAll<HTMLTableRowElement>('tbody tr')].find(
      (row) => row.querySelector('td')?.textContent === 'ZFS Pools',
    );
    const link = pool?.querySelector('a');
    expect(link?.textContent).toBe('Total/used/free capacity');
    expect(link?.getAttribute('href')).toBe('#pool-health-without-capacity');
    expect(prose(pool!)).toContain('ONLINE/DEGRADED/FAULTED');

    const section = sectionAt(article, link!.getAttribute('href')!);
    const text = prose(section);
    expect(text).toContain('Usage / Size (Usage on a phone)');
    expect(text).toContain('The dash is not 0% used');
    expect(text).toContain("same data pool in TrueNAS's own Storage view");
    expect(text).toContain('The boot pool is read separately');
    expect(text).toContain('Dataset figures can reflect quotas');
    expect(text).toContain('Keep the connection and key unchanged');
    expect(text).toContain('connection test, restart or wider permissions');
    expect(section.querySelector('pre, form, input, button')).toBeNull();
  });

  it('takes snapshot readers to the per-connection scope and the older-build limit', () => {
    const article = renderGuide('TRUENAS');
    const link = article.querySelector<HTMLAnchorElement>(
      'table a[href="/docs/TRUENAS#collection-scope-and-snapshot-polling"]',
    );
    expect(link).not.toBeNull();
    expect(link).toHaveAttribute('data-doc-link');
    expect(prose(link!.closest('tr')!)).toContain(
      'ZFS Snapshots TrueNAS → Protection Dataset, creation time, size, referenced data',
    );

    const scope = sectionAt(article, new URL(link!.href).hash);
    const text = prose(scope);
    expect(text).toContain('untick Datasets and select Update connection');
    expect(text).toContain('Leave Datasets on if you need fresh snapshot records');
    expect(text).toContain('system information, disks and native alerts');
    expect(text).toContain('They do not delete snapshots or stop TrueNAS replication');
    expect(text).toContain('v6.8.0 still polls snapshots with Datasets off');
    expect(text).toContain('v6.9.0-beta.3 can skip them, but leave Pools or Replication on');
    expect(text).toContain('turns the optional reads back on');
    expect(scope.querySelector('pre, form, input, button')).toBeNull();
    const ordinaryPoll = scope.querySelector<HTMLAnchorElement>(
      'a[href="/docs/TRUENAS#stale-truenas-data"]',
    );
    expect(ordinaryPoll?.textContent).toBe('ordinary poll');
    expect(article.querySelector(new URL(ordinaryPoll!.href).hash)?.textContent).toBe(
      'Stale TrueNAS data',
    );
  });
});
