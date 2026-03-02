import { describe, it, expect, afterEach } from 'vitest';
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

// ── Helpers ──────────────────────────────────────────────────────────────────

function makeDisk(overrides: Partial<Disk> = {}): Disk {
  return {
    total: 107374182400, // 100 GiB
    used: 53687091200, // ~50 GiB
    free: 53687091200,
    usage: 50,
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
  const trigger = container.querySelector('.bg-surface-hover');
  if (!trigger) throw new Error('Bar trigger element not found');
  return trigger as HTMLElement;
}

/** Get single-bar fill elements (non-stacked mode). */
function getSingleBarFill(container: HTMLElement): HTMLElement | null {
  // Single bar is .absolute.top-0.left-0.h-full that is a direct child (not inside a flex container)
  const fills = container.querySelectorAll<HTMLElement>('.absolute.top-0.left-0.h-full');
  // In non-stacked mode, the single bar is not inside a .flex container
  for (const fill of fills) {
    if (!fill.parentElement?.classList.contains('flex')) {
      return fill;
    }
  }
  return fills[0] ?? null;
}

/** Get stacked segment elements. */
function getStackedSegments(container: HTMLElement): HTMLElement[] {
  const flexContainer = container.querySelector('.absolute.top-0.left-0.h-full.w-full.flex');
  if (!flexContainer) return [];
  return Array.from(flexContainer.children) as HTMLElement[];
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
      expect(bar).toHaveStyle({ width: '50%' });
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

  // ── Multiple disks (stacked mode) ──────────────────────────────────────

  describe('stacked mode (multiple disks, default mode)', () => {
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

    it('renders stacked segments for multiple disks', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(2);
    });

    it('shows disk count badge', () => {
      render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      expect(screen.getByText('[2]')).toBeInTheDocument();
    });

    it('calculates overall percent across all disks', () => {
      // Total capacity: 50 GiB + 100 GiB = 150 GiB
      // Total used: 20 GiB + 50 GiB = 70 GiB
      // Overall: 70 / 150 = 46.67%  → 47%
      render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      expect(screen.getByText('47%')).toBeInTheDocument();
    });

    it('sets segment widths proportional to total capacity', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      const segments = getStackedSegments(container);
      // Total capacity = 150 GiB
      // disk1 used = 20 GiB → 20/150 ≈ 13.33%
      // disk2 used = 50 GiB → 50/150 ≈ 33.33%
      const width1 = parseFloat(segments[0].style.width);
      const width2 = parseFloat(segments[1].style.width);
      expect(width1).toBeCloseTo(13.33, 0);
      expect(width2).toBeCloseTo(33.33, 0);
    });

    it('adds border-right separator between segments', () => {
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
      const segments = getStackedSegments(container);
      expect(segments[0].style.borderRight).toContain('1px solid');
      // Last segment has border-right: 'none' in source, which jsdom omits from style
      expect(segments[1].style.borderRight).not.toContain('1px solid');
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
    });
  });

  // ── Mini mode ──────────────────────────────────────────────────────────

  describe('mini mode', () => {
    it('renders individual mini bars for each disk', () => {
      const disk1 = makeDisk({ mountpoint: '/boot' });
      const disk2 = makeDisk({ mountpoint: '/data' });
      render(() => <StackedDiskBar disks={[disk1, disk2]} mode="mini" />);
      expect(screen.getByText('/boot')).toBeInTheDocument();
      expect(screen.getByText('/data')).toBeInTheDocument();
    });

    it('uses device name when mountpoint is missing', () => {
      const disk = makeDisk({ mountpoint: undefined, device: '/dev/nvme0n1' });
      render(() => <StackedDiskBar disks={[disk]} mode="mini" />);
      expect(screen.getByText('/dev/nvme0n1')).toBeInTheDocument();
    });

    it('falls back to "Disk N" label when both mountpoint and device are missing', () => {
      const disk = makeDisk({ mountpoint: undefined, device: undefined });
      render(() => <StackedDiskBar disks={[disk]} mode="mini" />);
      expect(screen.getByText('Disk 1')).toBeInTheDocument();
    });

    it('renders grid layout with correct column count', () => {
      const disk1 = makeDisk({ mountpoint: '/boot' });
      const disk2 = makeDisk({ mountpoint: '/data' });
      const disk3 = makeDisk({ mountpoint: '/var' });
      const { container } = render(() => (
        <StackedDiskBar disks={[disk1, disk2, disk3]} mode="mini" />
      ));
      const grid = container.querySelector('.grid');
      expect(grid).toBeInTheDocument();
      expect(grid).toHaveStyle({
        'grid-template-columns': 'repeat(3, minmax(0, 1fr))',
      });
    });

    it('clamps mini bar fill width to exactly 100% when used exceeds total', () => {
      const disk = makeDisk({ used: 200000000000, total: 107374182400, mountpoint: '/full' });
      const { container } = render(() => <StackedDiskBar disks={[disk]} mode="mini" />);
      const miniBars = container.querySelectorAll('.h-full');
      const barFill = miniBars[miniBars.length - 1] as HTMLElement;
      expect(barFill.style.width).toBe('100%');
    });
  });

  // ── Tooltip ──────────────────────────────────────────────────────────────

  describe('tooltip', () => {
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
      expect(bar!.style.backgroundColor).toContain('34, 197, 94');
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
      expect(bar!.style.backgroundColor).toContain('234, 179, 8');
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
      expect(bar!.style.backgroundColor).toContain('239, 68, 68');
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
        <StackedDiskBar disks={[normalDisk, warningDisk]} />
      ));
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(2);
      // First segment: normal → palette color (green)
      expect(segments[0].style.backgroundColor).toContain('34, 197, 94');
      // Second segment: warning → yellow
      expect(segments[1].style.backgroundColor).toContain('234, 179, 8');
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
        <StackedDiskBar disks={[normalDisk, criticalDisk]} />
      ));
      const segments = getStackedSegments(container);
      expect(segments.length).toBe(2);
      // First segment: normal → palette color
      expect(segments[0].style.backgroundColor).not.toContain('239, 68, 68');
      // Second segment: critical → red
      expect(segments[1].style.backgroundColor).toContain('239, 68, 68');
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
      expect(bar!.style.width).toBe('100%');
    });
  });

  // ── Zero capacity edge case ─────────────────────────────────────────────

  describe('zero capacity', () => {
    it('handles zero total capacity gracefully', () => {
      const disk = makeDisk({ used: 0, total: 0, free: 0 });
      render(() => <StackedDiskBar disks={[disk]} />);
      expect(screen.getByText('0%')).toBeInTheDocument();
    });

    it('handles zero total in stacked mode without NaN segments', () => {
      const disk1 = makeDisk({ used: 0, total: 0 });
      const disk2 = makeDisk({ used: 0, total: 0 });
      const { container } = render(() => <StackedDiskBar disks={[disk1, disk2]} />);
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
      expect(bar!.style.backgroundColor).toContain('239, 68, 68');
    });
  });
});
