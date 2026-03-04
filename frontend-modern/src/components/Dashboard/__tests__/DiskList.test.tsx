import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { DiskList } from '../DiskList';
import type { Disk } from '@/types/api';

function makeDisk(overrides: Partial<Disk> = {}): Disk {
  return {
    total: 107374182400, // 100 GB
    used: 53687091200, // 50 GB
    free: 53687091200,
    usage: 50,
    mountpoint: '/',
    type: 'ext4',
    device: '/dev/sda1',
    ...overrides,
  };
}

/** Select the progress bar fill element (the absolutely-positioned inner div with an inline width style). */
function getBarFill(container: HTMLElement): HTMLElement | null {
  return container.querySelector('.absolute.inset-y-0.left-0[style]');
}

afterEach(() => {
  cleanup();
});

describe('DiskList', () => {
  describe('fallback when no disks', () => {
    it('renders fallback "-" when disks array is empty', () => {
      render(() => <DiskList disks={[]} />);
      expect(screen.getByText('-')).toBeInTheDocument();
    });

    it('shows default tooltip when no diskStatusReason', () => {
      render(() => <DiskList disks={[]} />);
      expect(screen.getByText('-')).toHaveAttribute(
        'title',
        'Disk stats unavailable. Guest agent may not be installed.',
      );
    });

    it.each([
      [
        'agent-not-running',
        'Guest agent not running. Install and start qemu-guest-agent in the VM.',
      ],
      ['agent-timeout', 'Guest agent timeout. Agent may need to be restarted.'],
      [
        'permission-denied',
        'Permission denied. Check that your Pulse user/token has VM.Monitor permission (PVE 8) or VM.GuestAgent.Audit permission (PVE 9).',
      ],
      ['agent-disabled', 'Guest agent is disabled in VM configuration. Enable it in VM Options.'],
      ['no-filesystems', 'No filesystems found. VM may be booting or using a Live ISO.'],
      [
        'special-filesystems-only',
        'Only special filesystems detected (ISO/squashfs). This is normal for Live systems.',
      ],
      ['agent-error', 'Error communicating with guest agent.'],
      ['no-data', 'No disk data available from Proxmox API.'],
    ] as const)('shows correct tooltip for diskStatusReason="%s"', (reason, expected) => {
      render(() => <DiskList disks={[]} diskStatusReason={reason} />);
      expect(screen.getByText('-')).toHaveAttribute('title', expected);
    });

    it('shows default tooltip for unknown diskStatusReason', () => {
      render(() => <DiskList disks={[]} diskStatusReason="some-unknown-reason" />);
      expect(screen.getByText('-')).toHaveAttribute(
        'title',
        'Disk stats unavailable. Guest agent may not be installed.',
      );
    });
  });

  describe('rendering disks', () => {
    it('renders a single disk with correct label, usage, and type', () => {
      const disk = makeDisk({ mountpoint: '/data', type: 'xfs' });
      render(() => <DiskList disks={[disk]} />);

      expect(screen.getByText('/data')).toBeInTheDocument();
      expect(screen.getByText('XFS')).toBeInTheDocument();
      expect(screen.getByText('50%')).toBeInTheDocument();
      // Should NOT show fallback
      expect(screen.queryByText('-')).not.toBeInTheDocument();
    });

    it('renders multiple disks', () => {
      const disks = [
        makeDisk({ mountpoint: '/', device: '/dev/sda1' }),
        makeDisk({ mountpoint: '/home', device: '/dev/sda2', used: 80530636800, usage: 75 }),
      ];
      render(() => <DiskList disks={disks} />);

      expect(screen.getByText('/')).toBeInTheDocument();
      expect(screen.getByText('/home')).toBeInTheDocument();
    });

    it('shows formatted usage text (used/total) for disks with capacity', () => {
      // 50 GB used / 100 GB total
      const disk = makeDisk({ used: 53687091200, total: 107374182400 });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('50.0 GB/100 GB')).toBeInTheDocument();
    });

    it('uses device as label when mountpoint is missing', () => {
      const disk = makeDisk({ mountpoint: undefined, device: '/dev/vda1' });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('/dev/vda1')).toBeInTheDocument();
    });

    it('uses "Unknown" label when both mountpoint and device are missing', () => {
      const disk = makeDisk({ mountpoint: undefined, device: undefined });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('Unknown')).toBeInTheDocument();
    });

    it('sets title attribute on label when label is not "Unknown"', () => {
      const disk = makeDisk({ mountpoint: '/var/log' });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('/var/log')).toHaveAttribute('title', '/var/log');
    });

    it('does not set title attribute when label is "Unknown"', () => {
      const disk = makeDisk({ mountpoint: undefined, device: undefined });
      render(() => <DiskList disks={[disk]} />);
      const label = screen.getByText('Unknown');
      expect(label).not.toHaveAttribute('title');
    });
  });

  describe('usage calculation', () => {
    it('calculates usage percent correctly', () => {
      // 25 GB used / 100 GB total = 25%
      const disk = makeDisk({ used: 26843545600, total: 107374182400 });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('25%')).toBeInTheDocument();
    });

    it('shows 0% for zero used', () => {
      const disk = makeDisk({ used: 0, total: 107374182400 });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('0%')).toBeInTheDocument();
    });

    it('shows 100% for fully used disk', () => {
      const disk = makeDisk({ used: 107374182400, total: 107374182400, usage: 100 });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('100%')).toBeInTheDocument();
    });

    it('handles zero total gracefully (shows dash instead of percent)', () => {
      const disk = makeDisk({ used: 500, total: 0 });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('—')).toBeInTheDocument();
    });

    it('handles negative total gracefully', () => {
      const disk = makeDisk({ used: 500, total: -1 });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('—')).toBeInTheDocument();
    });

    it('shows "Usage unavailable" when total is zero', () => {
      const disk = makeDisk({ total: 0 });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('Usage unavailable')).toBeInTheDocument();
    });
  });

  describe('progress bar', () => {
    it('sets progress bar width to usage percent', () => {
      const disk = makeDisk({ used: 53687091200, total: 107374182400 }); // 50%
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar).toHaveStyle({ width: '50%' });
    });

    it('caps progress bar width at 100% even when usage exceeds 100', () => {
      const disk = makeDisk({ used: 214748364800, total: 107374182400 }); // 200%
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar).toHaveStyle({ width: '100%' });
    });

    it('applies normal color class for usage below warning threshold', () => {
      // 50% disk usage — below warning (80)
      const disk = makeDisk({ used: 53687091200, total: 107374182400 });
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar!.className).toContain('bg-metric-normal-bg');
    });

    it('applies normal color class at exactly 79% (just below warning)', () => {
      // 79% — boundary just under disk warning threshold of 80
      const disk = makeDisk({
        used: Math.round(107374182400 * 0.79),
        total: 107374182400,
      });
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar!.className).toContain('bg-metric-normal-bg');
    });

    it('applies warning color class at exactly 80% (disk warning threshold)', () => {
      // 80% — exactly at disk warning threshold
      const disk = makeDisk({
        used: Math.round(107374182400 * 0.8),
        total: 107374182400,
      });
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar!.className).toContain('bg-metric-warning-bg');
    });

    it('applies warning color class at 85% (between warning and critical)', () => {
      const disk = makeDisk({
        used: 91268055040, // ~85 GB
        total: 107374182400,
      });
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar!.className).toContain('bg-metric-warning-bg');
    });

    it('applies warning color class at exactly 89% (just below critical)', () => {
      const disk = makeDisk({
        used: Math.round(107374182400 * 0.89),
        total: 107374182400,
      });
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar!.className).toContain('bg-metric-warning-bg');
    });

    it('applies critical color class at exactly 90% (disk critical threshold)', () => {
      const disk = makeDisk({
        used: Math.round(107374182400 * 0.9),
        total: 107374182400,
      });
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar!.className).toContain('bg-metric-critical-bg');
    });

    it('applies critical color class for very high usage (95%)', () => {
      const disk = makeDisk({
        used: 102005473280,
        total: 107374182400,
      });
      const { container } = render(() => <DiskList disks={[disk]} />);

      const bar = getBarFill(container);
      expect(bar).toBeInTheDocument();
      expect(bar!.className).toContain('bg-metric-critical-bg');
    });
  });

  describe('disk type display', () => {
    it('uppercases disk type', () => {
      const disk = makeDisk({ type: 'ext4' });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('EXT4')).toBeInTheDocument();
    });

    it('renders empty string when type is undefined', () => {
      const disk = makeDisk({ type: undefined });
      const { container } = render(() => <DiskList disks={[disk]} />);
      expect(container.textContent).not.toContain('UNDEFINED');
    });

    it('handles various disk types', () => {
      const disk = makeDisk({ type: 'zfs' });
      render(() => <DiskList disks={[disk]} />);
      expect(screen.getByText('ZFS')).toBeInTheDocument();
    });
  });
});
