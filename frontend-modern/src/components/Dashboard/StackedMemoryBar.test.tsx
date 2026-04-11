import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { StackedMemoryBar } from './StackedMemoryBar';

let resizeCallback: ResizeObserverCallback | undefined;
const mockObserve = vi.fn();
const mockDisconnect = vi.fn();

global.ResizeObserver = class ResizeObserver {
  constructor(cb: ResizeObserverCallback) {
    resizeCallback = cb;
  }
  observe = mockObserve;
  disconnect = mockDisconnect;
  unobserve = vi.fn();
};

function getSegments(container: HTMLElement): SVGRectElement[] {
  return Array.from(
    container.querySelectorAll<SVGRectElement>('rect[data-stacked-memory-segment="true"]'),
  );
}

function getSwapBar(container: HTMLElement): SVGRectElement | null {
  return container.querySelector('rect[data-stacked-memory-swap="true"]');
}

describe('StackedMemoryBar', () => {
  beforeEach(() => {
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 200 });
    resizeCallback = undefined;
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  // ---- Utilization percent computation ----

  it('renders percentage label when used/total provided', () => {
    // 4GB used of 8GB total = 50%
    render(() => <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} />);
    expect(screen.getByText('50%')).toBeInTheDocument();
  });

  it('renders 0% when both used and total are 0', () => {
    render(() => <StackedMemoryBar used={0} total={0} />);
    expect(screen.getByText('0%')).toBeInTheDocument();
  });

  it('uses percentOnly when total is 0', () => {
    render(() => <StackedMemoryBar used={0} total={0} percentOnly={73} />);
    expect(screen.getByText('73%')).toBeInTheDocument();
  });

  it('clamps percentOnly to 0-100 range', () => {
    render(() => <StackedMemoryBar used={0} total={0} percentOnly={150} />);
    expect(screen.getByText('100%')).toBeInTheDocument();
    cleanup();

    render(() => <StackedMemoryBar used={0} total={0} percentOnly={-20} />);
    expect(screen.getByText('0%')).toBeInTheDocument();
  });

  it('prefers used/total over percentOnly', () => {
    // 2GB/8GB = 25%, percentOnly = 90 should be ignored
    render(() => <StackedMemoryBar used={2 * 1024 ** 3} total={8 * 1024 ** 3} percentOnly={90} />);
    expect(screen.getByText('25%')).toBeInTheDocument();
  });

  it('allows used > total without clamping (intentional for VM overcommit)', () => {
    // 10GB used / 8GB total = 125% — intentionally not clamped
    const { container } = render(() => (
      <StackedMemoryBar used={10 * 1024 ** 3} total={8 * 1024 ** 3} />
    ));
    expect(screen.getByText('125%')).toBeInTheDocument();
    // Segment width also exceeds 100% (the SVG viewBox clips visually)
    const segment = getSegments(container)[0];
    expect(segment).toHaveAttribute('width', '125');
  });

  // ---- Segments ----

  it('renders a single active segment for normal memory usage', () => {
    const { container } = render(() => (
      <StackedMemoryBar used={2 * 1024 ** 3} total={8 * 1024 ** 3} />
    ));
    // The bar fills 25%
    const segments = getSegments(container);
    expect(segments.length).toBe(1);
    expect(segments[0]).toHaveAttribute('width', '25');
    expect(segments[0]).toHaveAttribute('x', '0');
  });

  it('renders balloon segment when active ballooning is in effect', () => {
    // used=2GB, total=8GB, balloon=4GB — active ballooning
    const { container } = render(() => (
      <StackedMemoryBar used={2 * 1024 ** 3} total={8 * 1024 ** 3} balloon={4 * 1024 ** 3} />
    ));
    const segments = getSegments(container);
    // Active segment + Balloon segment
    expect(segments.length).toBe(2);
    // Active: 2/8 = 25%
    expect(segments[0]).toHaveAttribute('width', '25');
    // Balloon: (4/8)*100 - 25 = 25%
    expect(segments[1]).toHaveAttribute('width', '25');
    expect(segments[1]).toHaveAttribute('x', '25');
  });

  it('does not render balloon segment when balloon equals total', () => {
    // balloon == total means ballooning configured but at max — no actual ballooning
    const { container } = render(() => (
      <StackedMemoryBar used={2 * 1024 ** 3} total={8 * 1024 ** 3} balloon={8 * 1024 ** 3} />
    ));
    const segments = getSegments(container);
    expect(segments.length).toBe(1);
  });

  it('does not render balloon segment when balloon is 0', () => {
    const { container } = render(() => (
      <StackedMemoryBar used={2 * 1024 ** 3} total={8 * 1024 ** 3} balloon={0} />
    ));
    const segments = getSegments(container);
    expect(segments.length).toBe(1);
  });

  it('does not render balloon segment when used exceeds balloon', () => {
    // used=5GB > balloon=4GB — no room for balloon segment
    const { container } = render(() => (
      <StackedMemoryBar used={5 * 1024 ** 3} total={8 * 1024 ** 3} balloon={4 * 1024 ** 3} />
    ));
    const segments = getSegments(container);
    // Only active segment (balloon filtered out because used > balloon)
    expect(segments.length).toBe(1);
  });

  it('renders no segments when used is 0 and total > 0', () => {
    const { container } = render(() => <StackedMemoryBar used={0} total={8 * 1024 ** 3} />);
    const segments = getSegments(container);
    // bytes=0 is filtered out
    expect(segments.length).toBe(0);
  });

  it('renders percent-only segment (no bytes) when total is 0 but percentOnly > 0', () => {
    const { container } = render(() => <StackedMemoryBar used={0} total={0} percentOnly={60} />);
    const segments = getSegments(container);
    expect(segments.length).toBe(1);
    expect(segments[0]).toHaveAttribute('width', '60');
  });

  // ---- Swap ----

  it('renders swap indicator when swap data is present and used > 0', () => {
    const { container } = render(() => (
      <StackedMemoryBar
        used={4 * 1024 ** 3}
        total={8 * 1024 ** 3}
        swapUsed={1 * 1024 ** 3}
        swapTotal={2 * 1024 ** 3}
      />
    ));
    // Swap indicator is the 3px bar at the bottom
    const swapBar = getSwapBar(container);
    expect(swapBar).toBeInTheDocument();
    // 1/2 = 50%
    expect(swapBar).toHaveAttribute('width', '50');
  });

  it('does not render swap indicator when swapUsed is 0', () => {
    const { container } = render(() => (
      <StackedMemoryBar
        used={4 * 1024 ** 3}
        total={8 * 1024 ** 3}
        swapUsed={0}
        swapTotal={2 * 1024 ** 3}
      />
    ));
    const swapBar = getSwapBar(container);
    expect(swapBar).not.toBeInTheDocument();
  });

  it('does not render swap indicator when swapTotal is 0', () => {
    const { container } = render(() => (
      <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} swapTotal={0} />
    ));
    const swapBar = getSwapBar(container);
    expect(swapBar).not.toBeInTheDocument();
  });

  it('clamps swap indicator width to 100% when swapUsed exceeds swapTotal', () => {
    const { container } = render(() => (
      <StackedMemoryBar
        used={4 * 1024 ** 3}
        total={8 * 1024 ** 3}
        swapUsed={3 * 1024 ** 3}
        swapTotal={2 * 1024 ** 3}
      />
    ));
    const swapBar = getSwapBar(container);
    expect(swapBar).toBeInTheDocument();
    // Math.min(150, 100) = 100%
    expect(swapBar).toHaveAttribute('width', '100');
  });

  // ---- Sublabel (bytes display) ----

  it('shows sublabel when space permits and total > 0', () => {
    render(() => <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} />);
    // Should show sublabel in parentheses with bytes format
    const sublabel = screen.getByText(/4\.00 GB/);
    expect(sublabel).toBeInTheDocument();
  });

  it('hides sublabel when container is too narrow', () => {
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 20 });
    render(() => <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} />);
    expect(screen.queryByText(/4\.00 GB/)).not.toBeInTheDocument();
  });

  it('does not show sublabel when total is 0', () => {
    render(() => <StackedMemoryBar used={0} total={0} percentOnly={50} />);
    const sublabelContainer = screen.queryByText(/\(/);
    expect(sublabelContainer).not.toBeInTheDocument();
  });

  it('updates sublabel visibility on resize', async () => {
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 20 });
    render(() => <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} />);
    expect(screen.queryByText(/4\.00 GB/)).not.toBeInTheDocument();

    // Simulate resize to wider container
    resizeCallback?.(
      [{ contentRect: { width: 300 } } as ResizeObserverEntry],
      {} as ResizeObserver,
    );
    expect(await screen.findByText(/4\.00 GB/)).toBeInTheDocument();
  });

  // ---- Anomaly indicator ----

  it('renders anomaly indicator for high ratio anomaly', () => {
    const anomaly = {
      resource_id: 'vm-100',
      resource_name: 'test-vm',
      resource_type: 'vm',
      metric: 'memory',
      current_value: 90,
      baseline_mean: 30,
      baseline_std_dev: 5,
      z_score: 12,
      severity: 'critical',
      description: 'Memory usage 3x above baseline',
    };
    render(() => <StackedMemoryBar used={7 * 1024 ** 3} total={8 * 1024 ** 3} anomaly={anomaly} />);
    // 90/30 = 3.0x
    expect(screen.getByText('3.0x')).toBeInTheDocument();
    expect(screen.getByText('3.0x')).toHaveClass('text-red-400');
  });

  it('renders up-arrow anomaly for moderate anomaly', () => {
    const anomaly = {
      resource_id: 'vm-100',
      resource_name: 'test-vm',
      resource_type: 'vm',
      metric: 'memory',
      current_value: 50,
      baseline_mean: 30,
      baseline_std_dev: 5,
      z_score: 4,
      severity: 'medium',
      description: 'Memory usage above baseline',
    };
    render(() => <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} anomaly={anomaly} />);
    // 50/30 = 1.67x → '↑↑'
    expect(screen.getByText('↑↑')).toBeInTheDocument();
    expect(screen.getByText('↑↑')).toHaveClass('text-yellow-400');
  });

  it('does not render anomaly indicator when anomaly is null', () => {
    render(() => <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} anomaly={null} />);
    // No anomaly ratio text (Nx, ↑↑, ↑) should be present
    expect(screen.queryByText(/\dx$/)).not.toBeInTheDocument();
    expect(screen.queryByText('↑↑')).not.toBeInTheDocument();
    expect(screen.queryByText('↑')).not.toBeInTheDocument();
  });

  // ---- Threshold-based coloring ----

  it('uses green color for low memory usage', () => {
    const { container } = render(() => (
      <StackedMemoryBar used={2 * 1024 ** 3} total={8 * 1024 ** 3} />
    ));
    const segment = getSegments(container)[0];
    // 25% → normal → green
    expect(segment.getAttribute('fill')).toContain('34, 197, 94');
  });

  it('uses yellow color for warning-level memory usage', () => {
    const { container } = render(() => (
      <StackedMemoryBar used={6 * 1024 ** 3} total={8 * 1024 ** 3} />
    ));
    const segment = getSegments(container)[0];
    // 75% → warning → yellow
    expect(segment.getAttribute('fill')).toContain('234, 179, 8');
  });

  it('uses red color for critical memory usage', () => {
    const { container } = render(() => (
      <StackedMemoryBar used={7 * 1024 ** 3} total={8 * 1024 ** 3} />
    ));
    const segment = getSegments(container)[0];
    // 87.5% → critical → red
    expect(segment.getAttribute('fill')).toContain('239, 68, 68');
  });

  // ---- ResizeObserver lifecycle ----

  it('registers and cleans up ResizeObserver', () => {
    const { unmount } = render(() => (
      <StackedMemoryBar used={4 * 1024 ** 3} total={8 * 1024 ** 3} />
    ));
    expect(mockObserve).toHaveBeenCalledTimes(1);
    unmount();
    expect(mockDisconnect).toHaveBeenCalledTimes(1);
  });
});
