import { describe, it, expect, afterEach } from 'vitest';
import { createSignal } from 'solid-js';
import { formatBytes } from '@/utils/format';
import { render, screen, cleanup, fireEvent } from '@solidjs/testing-library';
import type { Disk } from '@/types/api';
import type { AnomalyReport } from '@/types/aiIntelligence';

// Stub ResizeObserver for jsdom (StackedDiskBar uses it to measure container width)
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

import { StackedDiskBar } from '../StackedDiskBar';
import { buildStackedDiskBarPresentation } from '../stackedDiskBarModel';

// ── Helpers ──────────────────────────────────────────────────────────────────

function makeDisk(overrides: Partial<Disk> = {}): Disk {
  const total = overrides.total ?? 107374182400;
  const used = overrides.used ?? 53687091200;
  return {
    total: 107374182400, // 100 GiB
    used: 53687091200, // ~50 GiB
    free: 53687091200,
    usage: total > 0 ? (used / total) * 100 : 0,
    mountpoint: '/',
    type: 'ext4',
    device: '/dev/sda1',
    ...overrides,
  };
}

function makeAnomaly(overrides: Partial<AnomalyReport> = {}): AnomalyReport {
  return {
    resource_id: 'vm-100',
    resource_name: 'test-vm',
    resource_type: 'qemu',
    metric: 'disk',
    current_value: 95,
    baseline_mean: 40,
    baseline_std_dev: 5,
    z_score: 11,
    severity: 'high',
    description: 'Disk usage abnormally high',
    ...overrides,
  };
}

/** Get the bar trigger element (the element with mouse enter handler). */
function getBarTrigger(container: HTMLElement): HTMLElement {
  const trigger = container.querySelector('[data-stacked-disk-trigger]');
  if (!trigger) throw new Error('Bar trigger element not found');
  return trigger as HTMLElement;
}

/** Get single-bar fill elements (non-stacked mode). */
function getSingleBarFill(container: HTMLElement): SVGRectElement | null {
  return container.querySelector('rect[data-stacked-disk-fill="single"]');
}

/** Get stacked segment elements. */
function getStackedSegments(container: HTMLElement): SVGRectElement[] {
  return Array.from(
    container.querySelectorAll<SVGRectElement>('rect[data-stacked-disk-fill="segment"]'),
  );
}

/** Get inline per-disk bar fill elements. */
function getInlineBars(container: HTMLElement): SVGRectElement[] {
  return Array.from(
    container.querySelectorAll<SVGRectElement>('rect[data-stacked-disk-fill="inline"]'),
  );
}

afterEach(() => {
  cleanup();
});

// ── Rendering with no data ──────────────────────────────────────────────────

describe('StackedDiskBar', () => {
  describe('no data', () => {
    it('renders without crashing when no props given', () => {
      const { container } = render(() => <StackedDiskBar />);
      expect(container).toBeTruthy();
    });

    it('shows 0% when no disks or aggregate', () => {
      render(() => <StackedDiskBar />);
      expect(screen.getByText('0%')).toBeInTheDocument();
    });
  });

  // ── Single disk (no stacking) ───────────────────────────────────────────

  describe('single disk', () => {
    it('renders percent label for a single disk', () => {
      const disk = makeDisk({ used: 53687091200, total: 107374182400 }); // 50%
      render(() => <StackedDiskBar disks={[disk]} />);
      expect(screen.getByText('50%')).toBeInTheDocument();
    });

    it('sets bar fill width matching usage percent', () => {
      const disk = makeDisk({ used: 53687091200, total: 107374182400 }); // 50%
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      const bar = getSingleBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar).toHaveAttribute('width', '50');
    });

    it('does not show stacked segments for a single disk', () => {
      const disk = makeDisk();
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(0);
    });

    it('does not show disk count badge for single disk', () => {
      const disk = makeDisk();
      render(() => <StackedDiskBar disks={[disk]} />);
      expect(screen.queryByText('[1]')).not.toBeInTheDocument();
    });
  });

  // ── Aggregate disk fallback ─────────────────────────────────────────────

  describe('aggregate disk', () => {
    it('uses aggregateDisk when disks array is empty', () => {
      const agg = makeDisk({ used: 80530636800, total: 107374182400 }); // 75%
      render(() => <StackedDiskBar disks={[]} aggregateDisk={agg} />);
      expect(screen.getByText('75%')).toBeInTheDocument();
    });

    it('uses aggregateDisk when disks is undefined', () => {
      const agg = makeDisk({ used: 80530636800, total: 107374182400 }); // 75%
      render(() => <StackedDiskBar aggregateDisk={agg} />);
      expect(screen.getByText('75%')).toBeInTheDocument();
    });
  });

  it.each([Infinity, -Infinity, Number.NaN, -100, 0, undefined])(
    'renders the independent percentage without invalid capacity bytes (%s)',
    async (total) => {
      const { container } = render(() => (
        <StackedDiskBar aggregateDisk={{ total, used: 50, usage: 50 }} mode="aggregate" />
      ));
      expect(screen.getByText('50%')).toBeInTheDocument();
      expect(getSingleBarFill(container)).toHaveAttribute('width', '50');
      expect(container.textContent).not.toMatch(/NaN|Infinity|undefined|50.0 B\//);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(document.querySelector('[data-tooltip-portal]')?.textContent).not.toMatch(
        /NaN|Infinity|undefined/,
      );
    },
  );

  // ── Multiple disks (inline per-disk mode) ──────────────────────────────

  describe('inline per-disk mode (multiple disks, default mode)', () => {
    const disk1 = makeDisk({
      used: 21474836480, // 20 GiB
      total: 53687091200, // 50 GiB
      mountpoint: '/boot',
      device: '/dev/sda1',
    });
    const disk2 = makeDisk({
      used: 53687091200, // 50 GiB
      total: 107374182400, // 100 GiB
      mountpoint: '/data',
      device: '/dev/sdb1',
    });

    it('renders one compact inline bar for each disk by default', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(0);
      expect(getSingleBarFill(container)).not.toBeInTheDocument();
      expect(getInlineBars(container)).toHaveLength(2);
    });

    it('does not show a disk count badge when each disk is visible', () => {
      render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      expect(screen.queryByText('[2]')).not.toBeInTheDocument();
    });

    it('labels each inline disk with its own short name and percentage', () => {
      // disk1: 20 / 50 GiB = 40%
      // disk2: 50 / 100 GiB = 50%
      render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      expect(screen.getByText('boot 40%')).toBeInTheDocument();
      expect(screen.getByText('data 50%')).toBeInTheDocument();
    });

    it('sets each inline bar fill width to that disk usage percentage', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      const bars = getInlineBars(container);
      expect(bars[0]).toHaveAttribute('width', '40');
      expect(bars[1]).toHaveAttribute('width', '50');
    });
  });

  // ── Explicit stacked mode ──────────────────────────────────────────────

  describe('explicit stacked mode', () => {
    const disk1 = makeDisk({
      used: 21474836480, // 20 GiB
      total: 53687091200, // 50 GiB
      mountpoint: '/boot',
      device: '/dev/sda1',
    });
    const disk2 = makeDisk({
      used: 53687091200, // 50 GiB
      total: 107374182400, // 100 GiB
      mountpoint: '/data',
      device: '/dev/sdb1',
    });

    it('renders stacked segments when explicitly requested', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} mode="stacked" />);
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(2);
    });

    it('calculates overall percent across all disks', () => {
      // Total capacity: 50 GiB + 100 GiB = 150 GiB
      // Total used: 20 GiB + 50 GiB = 70 GiB
      // Overall: 70 / 150 = 46.67%  → 47%
      render(() => <StackedDiskBar disks={[disk1, disk2]} mode="stacked" />);
      expect(screen.getByText('47%')).toBeInTheDocument();
    });

    it('sets segment widths proportional to total capacity', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} mode="stacked" />);
      const segments = getStackedSegments(container);
      // Total capacity = 150 GiB
      // disk1 used = 20 GiB → 20/150 ≈ 13.33%
      // disk2 used = 50 GiB → 50/150 ≈ 33.33%
      const width1 = parseFloat(segments[0].getAttribute('width') ?? '0');
      const width2 = parseFloat(segments[1].getAttribute('width') ?? '0');
      expect(width1).toBeCloseTo(13.33, 0);
      expect(width2).toBeCloseTo(33.33, 0);
    });

    it('adds separators between stacked segments', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} mode="stacked" />);
      expect(container.querySelectorAll('line')).toHaveLength(1);
    });
  });

  // ── Aggregate mode with multiple disks ──────────────────────────────────

  describe('aggregate mode', () => {
    it('renders a single bar instead of stacked segments', () => {
      const disk1 = makeDisk({ used: 21474836480, total: 53687091200 });
      const disk2 = makeDisk({ used: 53687091200, total: 107374182400 });
      const { container } = render(() => (
        <StackedDiskBar disks={[disk1, disk2]} mode="aggregate" />
      ));
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(0);
      const bar = getSingleBarFill(container);
      expect(bar).toBeInTheDocument();
    });

    it('does not show disk count badge in aggregate mode', () => {
      const disk1 = makeDisk();
      const disk2 = makeDisk();
      render(() => <StackedDiskBar disks={[disk1, disk2]} mode="aggregate" />);
      expect(screen.queryByText('[2]')).not.toBeInTheDocument();
      expect(screen.queryByText('(2)')).not.toBeInTheDocument();
      expect(screen.queryByText('2 disks')).not.toBeInTheDocument();
    });

    it('uses total usage as the default aggregate summary', () => {
      const disk1 = makeDisk({
        used: 21474836480, // 20 GiB
        total: 53687091200, // 50 GiB
      });
      const disk2 = makeDisk({
        used: 53687091200, // 50 GiB
        total: 107374182400, // 100 GiB
      });
      render(() => <StackedDiskBar disks={[disk1, disk2]} mode="aggregate" />);
      expect(screen.getByText('47%')).toBeInTheDocument();
    });

    it('can summarize aggregate mode by the highest individual disk usage', () => {
      const disk1 = makeDisk({
        used: Math.round(107374182400 * 0.2),
        total: 107374182400,
        mountpoint: '/data',
      });
      const disk2 = makeDisk({
        used: Math.round(53687091200 * 0.92),
        total: 53687091200,
        mountpoint: '/var',
      });
      const { container } = render(() => (
        <StackedDiskBar
          disks={[disk1, disk2]}
          mode="aggregate"
          summaryStrategy="max"
          showDiskCount
        />
      ));
      const bar = getSingleBarFill(container);
      expect(screen.getByText('92%')).toBeInTheDocument();
      expect(screen.getByText('max')).toHaveAttribute('title', 'Highest usage: /var 92%');
      expect(screen.getByText('2 disks')).toHaveAttribute(
        'title',
        '2 operational disks in breakdown',
      );
      expect(bar).toHaveAttribute('width', '92');
      expect(bar?.getAttribute('fill')).toContain('239, 68, 68');
    });
  });

  // ── Mini mode ──────────────────────────────────────────────────────────

  describe('mini mode', () => {
    it('renders individual inline bars for each disk', () => {
      const disk1 = makeDisk({ mountpoint: '/boot' });
      const disk2 = makeDisk({ mountpoint: '/data' });
      render(() => <StackedDiskBar disks={[disk1, disk2]} mode="mini" />);
      expect(screen.getByText('boot 50%')).toBeInTheDocument();
      expect(screen.getByText('data 50%')).toBeInTheDocument();
    });

    it('uses device name when mountpoint is missing', () => {
      const disk = makeDisk({ mountpoint: undefined, device: '/dev/nvme0n1' });
      render(() => <StackedDiskBar disks={[disk]} mode="mini" />);
      expect(screen.getByText('nvme0n1 50%')).toBeInTheDocument();
    });

    it('falls back to "Disk N" label when both mountpoint and device are missing', () => {
      const disk = makeDisk({ mountpoint: undefined, device: undefined });
      render(() => <StackedDiskBar disks={[disk]} mode="mini" />);
      expect(screen.getByText('Disk 1 50%')).toBeInTheDocument();
    });

    it('renders one equal-width inline bar per disk', () => {
      const disk1 = makeDisk({ mountpoint: '/boot' });
      const disk2 = makeDisk({ mountpoint: '/data' });
      const disk3 = makeDisk({ mountpoint: '/var' });
      const { container } = render(() => (
        <StackedDiskBar disks={[disk1, disk2, disk3]} mode="mini" />
      ));
      expect(getInlineBars(container)).toHaveLength(3);
    });

    it('clamps inline bar fill width to exactly 100% when used exceeds total', () => {
      const disk = makeDisk({ used: 200000000000, total: 107374182400, mountpoint: '/full' });
      const { container } = render(() => <StackedDiskBar disks={[disk]} mode="mini" />);
      const miniBars = getInlineBars(container);
      const barFill = miniBars[miniBars.length - 1];
      expect(barFill).toHaveAttribute('width', '100');
    });
  });

  // ── Tooltip ──────────────────────────────────────────────────────────────

  describe('tooltip', () => {
    it('includes caller-owned freshness context beside unchanged disk evidence', async () => {
      const message = 'Using last known disk stats. Guest reads paused while a backup is running.';
      const { container } = render(() => (
        <StackedDiskBar disks={[makeDisk()]} statusMessage={message} />
      ));
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText(message)).toBeInTheDocument();
      expect(screen.getByText('/')).toBeInTheDocument();
      expect(screen.getByText('Disk Usage')).toBeInTheDocument();
    });

    it('shows tooltip on mouse enter with disk details', async () => {
      const disk = makeDisk({
        used: 53687091200,
        total: 107374182400,
        mountpoint: '/home',
      });
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('Disk Usage')).toBeInTheDocument();
      expect(screen.getByText('/home')).toBeInTheDocument();
    });

    it('shows "Disk Breakdown" title for multiple disks', async () => {
      const disk1 = makeDisk({ mountpoint: '/boot' });
      const disk2 = makeDisk({ mountpoint: '/data' });
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('Disk Breakdown')).toBeInTheDocument();
    });

    it('wraps long mountpoint labels in the tooltip instead of truncating them', async () => {
      const longMountpoint = '/Volumes/Development/projects/pulse/cache/runtime-images';
      const disk1 = makeDisk({ mountpoint: '/' });
      const disk2 = makeDisk({ mountpoint: longMountpoint });
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      await fireEvent.mouseEnter(getBarTrigger(container));

      expect(screen.getByText(longMountpoint)).toHaveClass('break-all');
    });

    it('hides tooltip on mouse leave', async () => {
      const disk = makeDisk({ mountpoint: '/home' });
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      const trigger = getBarTrigger(container);
      await fireEvent.mouseEnter(trigger);
      expect(screen.getByText('Disk Usage')).toBeInTheDocument();
      await fireEvent.mouseLeave(trigger);
      expect(screen.queryByText('Disk Usage')).not.toBeInTheDocument();
    });

    it('shows aggregate disk tooltip when no disks array', async () => {
      const agg = makeDisk({ used: 80530636800, total: 107374182400 });
      const { container } = render(() => <StackedDiskBar aggregateDisk={agg} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('Total')).toBeInTheDocument();
    });

    it('does not show tooltip when there is no data', async () => {
      const { container } = render(() => <StackedDiskBar />);
      const trigger = getBarTrigger(container);
      await fireEvent.mouseEnter(trigger);
      // No tooltip title should appear (empty tooltipContent)
      expect(screen.queryByText('Disk Usage')).not.toBeInTheDocument();
      expect(screen.queryByText('Disk Breakdown')).not.toBeInTheDocument();
    });
  });

  // ── Color thresholds ──────────────────────────────────────────────────────

  describe('color thresholds', () => {
    it('uses normal color for disk under 80%', () => {
      const disk = makeDisk({
        used: Math.round(107374182400 * 0.5),
        total: 107374182400,
      });
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      const bar = getSingleBarFill(container);
      expect(bar).toBeInTheDocument();
      // Normal green: rgba(34, 197, 94, 0.6)
      expect(bar?.getAttribute('fill')).toContain('34, 197, 94');
    });

    it('uses warning color for disk at 80-89%', () => {
      const disk = makeDisk({
        used: Math.round(107374182400 * 0.85),
        total: 107374182400,
      });
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      const bar = getSingleBarFill(container);
      expect(bar).toBeInTheDocument();
      // Warning yellow: rgba(234, 179, 8, 0.6)
      expect(bar?.getAttribute('fill')).toContain('234, 179, 8');
    });

    it('uses critical color for disk at 90%+', () => {
      const disk = makeDisk({
        used: Math.round(107374182400 * 0.95),
        total: 107374182400,
      });
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      const bar = getSingleBarFill(container);
      expect(bar).toBeInTheDocument();
      // Critical red: rgba(239, 68, 68, 0.6)
      expect(bar?.getAttribute('fill')).toContain('239, 68, 68');
    });

    it('uses warning color for stacked segment at 80-89%', () => {
      const normalDisk = makeDisk({
        used: Math.round(107374182400 * 0.3),
        total: 107374182400,
        mountpoint: '/data',
      });
      const warningDisk = makeDisk({
        used: Math.round(107374182400 * 0.85),
        total: 107374182400,
        mountpoint: '/warn',
      });
      const { container } = render(() => (
        <StackedDiskBar disks={[normalDisk, warningDisk]} mode="stacked" />
      ));
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(2);
      // First segment: normal → palette color (green)
      expect(segments[0].getAttribute('fill')).toContain('34, 197, 94');
      // Second segment: warning → yellow
      expect(segments[1].getAttribute('fill')).toContain('234, 179, 8');
    });

    it('uses critical color for stacked segment at 90%+', () => {
      const normalDisk = makeDisk({
        used: Math.round(107374182400 * 0.3),
        total: 107374182400,
        mountpoint: '/data',
      });
      const criticalDisk = makeDisk({
        used: Math.round(107374182400 * 0.95),
        total: 107374182400,
        mountpoint: '/full',
      });
      const { container } = render(() => (
        <StackedDiskBar disks={[normalDisk, criticalDisk]} mode="stacked" />
      ));
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(2);
      // First segment: normal → palette color
      expect(segments[0].getAttribute('fill')).not.toContain('239, 68, 68');
      // Second segment: critical → red
      expect(segments[1].getAttribute('fill')).toContain('239, 68, 68');
    });
  });

  // ── Bar percent clamping ──────────────────────────────────────────────────

  describe('bar percent clamping', () => {
    it('clamps bar width to exactly 100% when usage exceeds capacity', () => {
      const disk = makeDisk({
        used: 200000000000, // 186 GiB used of 100 GiB
        total: 107374182400,
      });
      const { container } = render(() => <StackedDiskBar disks={[disk]} />);
      const bar = getSingleBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar).toHaveAttribute('width', '100');
    });
  });

  // ── Zero capacity edge case ─────────────────────────────────────────────

  describe('zero capacity', () => {
    it('shows unavailable rather than empty utilization for zero total capacity', () => {
      const disk = makeDisk({ used: 0, total: 0, free: 0 });
      render(() => <StackedDiskBar disks={[disk]} />);
      expect(screen.getByText('N/A')).toBeInTheDocument();
      expect(screen.queryByText('0%')).not.toBeInTheDocument();
    });

    it('handles zero total in stacked mode without NaN segments', () => {
      const disk1 = makeDisk({ used: 0, total: 0 });
      const disk2 = makeDisk({ used: 0, total: 0 });
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} mode="stacked" />);
      const segments = getStackedSegments(container);
      // With zero total capacity, segments() returns [] early
      expect(segments.length).toBe(0);
    });
  });

  // ── Anomaly indicator ──────────────────────────────────────────────────

  describe('anomaly indicator', () => {
    it('renders anomaly ratio when anomaly is present', () => {
      const disk = makeDisk();
      const anomaly = makeAnomaly({
        current_value: 95,
        baseline_mean: 40,
      });
      render(() => <StackedDiskBar disks={[disk]} anomaly={anomaly} />);
      // 95/40 = 2.375 → "2.4x"
      expect(screen.getByText('2.4x')).toBeInTheDocument();
    });

    it('applies severity class to anomaly indicator', () => {
      const disk = makeDisk();
      const anomaly = makeAnomaly({
        severity: 'critical',
        current_value: 95,
        baseline_mean: 40,
      });
      render(() => <StackedDiskBar disks={[disk]} anomaly={anomaly} />);
      const ratioEl = screen.getByText('2.4x');
      expect(ratioEl.className).toContain('text-red-400');
    });

    it('does not render anomaly indicator when anomaly is null', () => {
      const disk = makeDisk();
      render(() => <StackedDiskBar disks={[disk]} anomaly={null} />);
      expect(screen.queryByText(/\dx$/)).not.toBeInTheDocument();
    });

    it('shows anomaly description as title attribute', () => {
      const disk = makeDisk();
      const anomaly = makeAnomaly({
        description: 'Disk usage abnormally high',
        current_value: 95,
        baseline_mean: 40,
      });
      render(() => <StackedDiskBar disks={[disk]} anomaly={anomaly} />);
      const ratioEl = screen.getByText('2.4x');
      expect(ratioEl).toHaveAttribute('title', 'Disk usage abnormally high');
    });
  });

  // ── maxDiskInfo for aggregate mode ────────────────────────────────────────

  describe('aggregate mode max disk info', () => {
    it('colors aggregate bar by the max individual disk percent', () => {
      // disk1: 50% usage, disk2: 92% usage (critical)
      const disk1 = makeDisk({
        used: Math.round(107374182400 * 0.5),
        total: 107374182400,
        mountpoint: '/data',
      });
      const disk2 = makeDisk({
        used: Math.round(53687091200 * 0.92),
        total: 53687091200,
        mountpoint: '/boot',
      });
      const { container } = render(() => (
        <StackedDiskBar disks={[disk1, disk2]} mode="aggregate" />
      ));
      const bar = getSingleBarFill(container);
      expect(bar).toBeInTheDocument();
      // Bar color should be critical (based on max disk at 92%)
      expect(bar?.getAttribute('fill')).toContain('239, 68, 68');
    });
  });

  describe('vertical-bars mode', () => {
    it('labels the micro-bars with the fullest disk so the cell reads as a number', () => {
      const { container } = render(() => (
        <StackedDiskBar
          mode="vertical-bars"
          disks={[
            makeDisk({ mountpoint: '/', usage: 24, used: 24, total: 100 }),
            makeDisk({ mountpoint: '/data', usage: 52, used: 52, total: 100 }),
          ]}
        />
      ));

      const label = container.querySelector('[data-stacked-disk-max-label]');
      expect(label).toHaveTextContent('52%');
      expect(label).toHaveAttribute('title', 'Highest usage: /data 52%');
      // Every disk keeps its own bar with equal weight; the label is derived.
      expect(container.querySelectorAll('[data-stacked-disk-fill="vertical"]')).toHaveLength(2);
    });

    it('omits the label when no disk has measured usage', () => {
      const { container } = render(() => (
        <StackedDiskBar
          mode="vertical-bars"
          disks={[makeDisk({ mountpoint: '/', usage: -1 })]}
          aggregateDisk={makeDisk({ usage: 30 })}
        />
      ));

      expect(container.querySelector('[data-stacked-disk-max-label]')).toBeNull();
    });
  });
});

describe('reactive disk reading decorations', () => {
  it('updates an open last-known tooltip and mounted shell through fresh and unavailable replacement', async () => {
    const message = 'Using last known disk stats. Guest reads paused during a backup.';
    const [reading, setReading] = createSignal<{
      state: 'last-known' | 'current' | 'unavailable';
      message: string | null;
    }>({ state: 'last-known', message });
    const [disk, setDisk] = createSignal(
      makeDisk({ used: 102005473280, free: 5368709120, usage: 95 }),
    );
    const { container } = render(() => (
      <StackedDiskBar disks={[disk()]} reading={reading()} anomaly={makeAnomaly()} />
    ));
    const shell = container.firstElementChild;
    expect(getSingleBarFill(container)).toHaveAttribute('fill', 'rgba(148, 163, 184, 0.5)');
    expect(container.querySelector('.animate-pulse')).toBeNull();
    fireEvent.mouseEnter(getBarTrigger(container));
    expect(await screen.findByText(message)).toBeVisible();
    expect(screen.getByText('Last known disk usage')).toHaveClass('text-base-content');
    expect(document.querySelector('[data-stacked-disk-fill="tooltip"]')).toHaveAttribute(
      'fill',
      'rgba(148, 163, 184, 0.5)',
    );
    setReading({ state: 'current', message: null });
    expect(container.firstElementChild).toBe(shell);
    expect(screen.queryByText(message)).toBeNull();
    expect(getSingleBarFill(container)).toHaveAttribute('fill', 'rgba(239, 68, 68, 0.6)');
    expect(container.querySelector('.animate-pulse')).not.toBeNull();
    setReading({ state: 'unavailable', message: 'No filesystem usage reading.' });
    expect(container.firstElementChild).toBe(shell);
    expect(container.querySelector('[data-stacked-disk-unavailable]')).toHaveTextContent('N/A');
    expect(container.querySelector('[data-stacked-disk-fill]')).toBeNull();
    expect(container.querySelector('.animate-pulse')).toBeNull();
    expect(screen.queryByText('95%')).toBeNull();
    expect(screen.getByText(`?/${formatBytes(disk().total ?? 0)}`)).toBeVisible();
    setDisk(makeDisk({ used: 0, usage: 0 }));
    setReading({ state: 'current', message: null });
    expect(container.firstElementChild).toBe(shell);
    expect(getSingleBarFill(container)).toHaveAttribute('width', '0');
    expect(getSingleBarFill(container)).toHaveAttribute('fill', 'rgba(34, 197, 94, 0.6)');
    fireEvent.mouseLeave(getBarTrigger(container));
  });
});

const mounts = (): Disk[] => [
  { mountpoint: '/', device: '/dev/vda1', total: 1000, used: 200, usage: 20 },
  { mountpoint: '/var/lib/containers', device: '/dev/vdb1', total: 9000, used: 8820, usage: 98 },
  { mountpoint: '/data-alias', device: '/dev/vdb1', total: 9000, used: 6000, usage: 66.67 },
];
const aggregate = (): Disk => ({ total: 10_000, used: 9020, usage: 90.2 });
const present = (aggregateDisk: Disk | undefined, disks = mounts()) =>
  buildStackedDiskBarPresentation({ disks, aggregateDisk, mode: 'aggregate' }, 400);

describe('source-owned aggregate disk summary', () => {
  it.each([false, true])(
    'uses the supplied aggregate without recounting aliases (reorder=%s)',
    (reverse) => {
      const disks = reverse ? mounts().reverse() : mounts();
      const p = present(aggregate(), disks);
      expect(p.displayPercentValue).toBe(90.2);
      expect(p.barPercent).toBe(90.2);
      expect(p.displaySublabel).toBe('8.81 KB/9.77 KB');
      expect(p.barColor).toBe('rgba(239, 68, 68, 0.6)');
      expect(p.tooltipContent.map((row) => row.label)).toEqual(
        disks.map((disk) => disk.mountpoint),
      );
      expect(p.tooltipContent.find((row) => row.label === '/var/lib/containers')?.percent).toBe(
        '98%',
      );
      expect(disks).toEqual(reverse ? mounts().reverse() : mounts());
    },
  );

  it('preserves source zero rather than counting retained mount readings', () => {
    expect(present({ total: 10_000, used: 0, usage: 0 }).displayPercentValue).toBe(0);
    expect(present({ total: 10_000, used: 0, usage: 0 }).displaySublabel).toBe('0 B/9.77 KB');
    expect(present({ total: 10_000, used: 200, usage: 2 }).displayPercentValue).toBe(2);
  });

  it.each([undefined, { total: 10_000 }, { total: 10_000, usage: -1 }, { usage: NaN }])(
    'falls back to measured rows when the aggregate has no usable usage (%j)',
    (value) => {
      const p = present(value);
      expect(p.displayPercentValue).toBeCloseTo((15_020 / 19_000) * 100);
      expect(p.displaySublabel).toBe('14.7 KB/18.6 KB');
    },
  );

  it.each([{ usage: 0.5 }, { total: Infinity, used: 50, usage: 50 }, { total: 100, usage: 25 }])(
    'preserves independent aggregate percentages without borrowing mount bytes (%j)',
    (value) => {
      const p = present(value);
      expect(p.displayPercentValue).toBe(value.usage);
      expect(p.displaySublabel).toBe('');
      expect(p.showSublabel).toBe(false);
      expect(p.tooltipContent).toHaveLength(3);
    },
  );

  it('retains max summary, per-filesystem colours and non-aggregate geometry', () => {
    const props = { disks: mounts(), aggregateDisk: aggregate() };
    const max = buildStackedDiskBarPresentation(
      { ...props, mode: 'aggregate', summaryStrategy: 'max' },
      400,
    );
    expect(max.displayPercentValue).toBe(98);
    expect(max.displaySublabel).toBe('');
    expect(max.maxLabelFull).toBe('Highest usage: /var/lib/containers 98%');
    for (const mode of ['stacked', 'mini', 'vertical-bars'] as const) {
      expect(buildStackedDiskBarPresentation({ ...props, mode }, 400)).toEqual(
        buildStackedDiskBarPresentation({ disks: mounts(), mode }, 400),
      );
    }
  });

  it('does not confer freshness or usable bytes on unknown filesystems', () => {
    const disks = mounts().map((disk) => ({ ...disk, usage: -1 }));
    const retained = buildStackedDiskBarPresentation(
      {
        disks,
        aggregateDisk: aggregate(),
        mode: 'aggregate',
        reading: { state: 'last-known', message: 'Retained, not live' },
      },
      400,
    );
    expect(retained.displayPercentValue).toBe(90.2);
    expect(retained.barColor).toBe('rgba(148, 163, 184, 0.5)');
    expect(retained.tooltipTitle).toBe('Last known disk breakdown');
    expect(retained.tooltipContent.every((row) => row.used === '?' && row.percent === '—')).toBe(
      true,
    );
    const missing = buildStackedDiskBarPresentation(
      { disks, aggregateDisk: { total: 10_000 }, mode: 'aggregate' },
      400,
    );
    expect(missing.displayLabel).toBe('N/A');
  });

  it('updates the mounted bar and open mount breakdown through source replacements', async () => {
    const [disk, setDisk] = createSignal(aggregate());
    const [disks, setDisks] = createSignal(mounts());
    const [state, setState] = createSignal<'current' | 'last-known' | 'unavailable'>('current');
    const { container } = render(() => (
      <StackedDiskBar
        mode="aggregate"
        aggregateDisk={disk()}
        disks={disks()}
        reading={{ state: state(), message: null }}
      />
    ));
    const shell = container.firstElementChild;
    const fill = () => container.querySelector('[data-stacked-disk-fill="single"]');
    expect(fill()).toHaveAttribute('width', '90.2');
    await fireEvent.mouseEnter(container.querySelector('[data-stacked-disk-trigger]')!);
    expect(await screen.findByText('/var/lib/containers')).toBeVisible();
    expect(screen.getByText('/data-alias')).toBeVisible();
    expect(screen.getByText('98%')).toBeVisible();
    setDisks(mounts().reverse());
    expect(fill()).toHaveAttribute('width', '90.2');
    setDisk({ total: 10_000, used: 200, usage: 2 });
    expect(fill()).toHaveAttribute('width', '2');
    setState('last-known');
    expect(fill()).toHaveAttribute('fill', 'rgba(148, 163, 184, 0.5)');
    setState('unavailable');
    expect(container.querySelector('[data-stacked-disk-unavailable]')).toHaveTextContent('N/A');
    expect(fill()).toBeNull();
    setState('current');
    setDisk({ total: 10_000, used: 0, usage: 0 });
    expect(fill()).toHaveAttribute('width', '0');
    expect(container.firstElementChild).toBe(shell);
    expect(screen.getByText('/data-alias')).toBeVisible();
  });
});
