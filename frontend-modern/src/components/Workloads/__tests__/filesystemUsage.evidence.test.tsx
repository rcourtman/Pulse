import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import type { Disk, VM } from '@/types/api';
import { DiskList } from '../DiskList';
import { GuestDrawerOverview } from '../GuestDrawerOverview';
import { buildWorkloadsDiskPresentation, getWorkloadsDiskUsagePercent } from '../diskListModel';

const capacity = 10 * 1024 ** 3;
const disk = (overrides: Partial<Disk> = {}): Disk => ({
  mountpoint: '/data',
  type: 'ext4',
  total: capacity,
  used: capacity / 2,
  usage: 50,
  ...overrides,
});

const unavailable: [string, Partial<Disk>][] = [
  ['missing used bytes', { used: undefined }],
  ['negative used bytes', { used: -1 }],
  ['NaN used bytes', { used: NaN }],
  ['infinite used bytes', { used: Infinity }],
  ['negative infinite used bytes', { used: -Infinity }],
  ['missing capacity', { total: undefined }],
  ['zero capacity', { total: 0 }],
  ['negative capacity', { total: -1 }],
  ['NaN capacity', { total: NaN }],
  ['infinite capacity', { total: Infinity }],
  ['negative infinite capacity', { total: -Infinity }],
  ['unknown usage sentinel', { usage: -1, used: 0 }],
  ['NaN usage', { usage: NaN }],
  ['infinite usage', { usage: Infinity }],
  ['negative infinite usage', { usage: -Infinity }],
  ['overflowing derived percentage', { total: 1, used: Number.MAX_VALUE }],
];

afterEach(cleanup);

describe('filesystem readings need measured bytes, not zero defaults', () => {
  it.each(unavailable)('withdraws a numeric reading for %s', (_name, values) => {
    const input = disk(values);
    const presentation = buildWorkloadsDiskPresentation(input, 0);
    expect(getWorkloadsDiskUsagePercent(input)).toBeNull();
    expect(presentation.progressValue).toBeNull();
    expect(presentation.progressWidth).toBe('0%');
    expect(presentation.progressClass).toBe('bg-surface-hover');
    expect(presentation.usagePercentLabel).toBe('—');
    expect(presentation.usageText).not.toMatch(/NaN|Infinity|^0 B\//);
    expect(presentation.label).toBe('/data');
    expect(presentation.typeLabel).toBe('EXT4');
  });

  it('retains known capacity without inventing used bytes from a percentage or free bytes', () => {
    const input = disk({ used: undefined, usage: 50, free: capacity / 2 });
    expect(buildWorkloadsDiskPresentation(input, 0).usageText).toBe('?/10.0 GB');
    expect(getWorkloadsDiskUsagePercent(input)).toBeNull();
  });

  it('keeps an explicit measured zero, including when the redundant usage field is omitted', () => {
    const presentation = buildWorkloadsDiskPresentation(disk({ used: 0, usage: undefined }), 0);
    expect(presentation.progressValue).toBe(0);
    expect(presentation.usagePercentLabel).toBe('0%');
    expect(presentation.usageText).toBe('0 B/10.0 GB');
    expect(presentation.progressClass).toContain('metric-normal');
  });

  it('derives ordinary and over-capacity percentages from the measured bytes', () => {
    const input = disk({ used: capacity * 1.25, usage: 0 });
    const presentation = buildWorkloadsDiskPresentation(input, 0);
    expect(getWorkloadsDiskUsagePercent(disk({ usage: undefined }))).toBe(50);
    expect(presentation.progressValue).toBe(125);
    expect(presentation.usagePercentLabel).toBe('125%');
    expect(presentation.progressWidth).toBe('100%');
    expect(presentation.progressClass).toContain('metric-critical');
  });
});

function renderPanels(initial: Disk) {
  const [observation, setObservation] = createSignal(initial);
  const guest = (): VM => ({
    id: 'fixture:pve-a:101',
    vmid: 101,
    node: 'pve-a',
    instance: 'fixture',
    name: 'filesystem-evidence',
    status: 'running',
    type: 'qemu',
    cpu: 0.1,
    cpus: 2,
    memory: { total: capacity, used: capacity / 2, free: capacity / 2, usage: 50 },
    disk: observation(),
    disks: [observation()],
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: '2026-10-04T04:00:00Z',
  });
  render(() => (
    <>
      <section data-testid="disk-list-panel">
        <DiskList disks={[observation()]} />
      </section>
      <GuestDrawerOverview
        guest={guest()}
        guestOsSummary=""
        agentHeading="Guest agent"
        agentLabel=""
        agentTitle=""
        hasAgentInfo={false}
        hasFilesystemDetails={true}
        hasNetworkInterfaces={false}
        hasOsInfo={false}
        hasWorkloadActionAgent={false}
        showInGuestAgentInstallCue={false}
        ipAddresses={[]}
        networkInterfaces={[]}
        normalizedTags={[]}
        backupPresentation={null}
        workloadActionAgentTitle=""
      />
    </>
  ));
  return setObservation;
}

describe('both filesystem panels preserve unavailable versus measured zero', () => {
  it.each([
    ['missing used bytes', { used: undefined }],
    ['invalid used bytes', { used: NaN }],
    ['invalid capacity', { total: Infinity }],
    ['unknown usage sentinel', { usage: -1, used: 0 }],
  ] satisfies [string, Partial<Disk>][])(
    'does not show an empty healthy disk for %s',
    (_name, values) => {
      renderPanels(disk(values));
      for (const id of ['disk-list-panel', 'guest-technical-details']) {
        const panel = screen.getByTestId(id);
        expect(panel.textContent).toContain('—');
        expect(panel.textContent).not.toMatch(/0%|NaN|Infinity/);
        expect(within(panel).queryByRole('progressbar')).not.toBeInTheDocument();
      }
    },
  );

  it('withdraws a prior measurement and restores zero only after explicit same-filesystem evidence', () => {
    const update = renderPanels(disk());
    const details = screen.getByTestId('guest-technical-details');
    expect(within(details).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '50');
    update(disk({ used: undefined }));
    expect(within(details).queryByRole('progressbar')).not.toBeInTheDocument();
    expect(details).toHaveTextContent('— · ?/10.0 GB · EXT4');
    update(disk({ used: 0, usage: undefined }));
    expect(within(details).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0');
    expect(details).toHaveTextContent('0% · 0 B/10.0 GB · EXT4');
  });
});
