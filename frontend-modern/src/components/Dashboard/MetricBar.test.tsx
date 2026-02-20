import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@solidjs/testing-library';
import { MetricBar } from './MetricBar';

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

describe('MetricBar', () => {
  beforeEach(() => {
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 100 });
    resizeCallback = undefined;
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('renders basic bar with label', () => {
    render(() => <MetricBar value={50} label="50%" type="cpu" />);
    expect(screen.getByText('50%')).toBeInTheDocument();
    const textEl = screen.getByText('50%');
    const container = textEl.closest('.relative') as HTMLElement;
    const bar = container.firstElementChild as HTMLElement;
    expect(bar).toHaveStyle({ width: '50%' });
  });

  it('renders correct color classes for CPU', () => {
    let result = render(() => <MetricBar value={50} label="val" type="cpu" />);
    let bar = result.container.querySelector('.bg-green-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={80} label="val" type="cpu" />);
    bar = result.container.querySelector('.bg-yellow-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={90} label="val" type="cpu" />);
    bar = result.container.querySelector('.bg-red-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();
  });

  it('renders correct color classes for Memory', () => {
    let result = render(() => <MetricBar value={50} label="val" type="memory" />);
    let bar = result.container.querySelector('.bg-green-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={75} label="val" type="memory" />);
    bar = result.container.querySelector('.bg-yellow-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={85} label="val" type="memory" />);
    bar = result.container.querySelector('.bg-red-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();
  });

  it('renders correct color classes for Disk', () => {
    let result = render(() => <MetricBar value={50} label="val" type="disk" />);
    let bar = result.container.querySelector('.bg-green-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={80} label="val" type="disk" />);
    bar = result.container.querySelector('.bg-yellow-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={90} label="val" type="disk" />);
    bar = result.container.querySelector('.bg-red-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();
  });

  it('renders correct color classes for Generic/Default (uses CPU thresholds)', () => {
    let result = render(() => <MetricBar value={50} label="val" />);
    let bar = result.container.querySelector('.bg-green-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={80} label="val" />);
    bar = result.container.querySelector('.bg-yellow-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();

    result = render(() => <MetricBar value={90} label="val" />);
    bar = result.container.querySelector('.bg-red-500\\/60');
    expect(bar).toBeInTheDocument();
    result.unmount();
  });

  it('shows sublabel when space permits', () => {
    render(() => <MetricBar value={50} label="val" sublabel="sub" />);
    expect(screen.getByText('(sub)')).toBeInTheDocument();
  });

  it('hides sublabel when space constrained', () => {
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 20 });
    render(() => <MetricBar value={50} label="VeryLongLabel" sublabel="LongSublabel" />);
    expect(screen.queryByText('(LongSublabel)')).not.toBeInTheDocument();
  });

  it('updates sublabel visibility on resize', async () => {
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 20 });
    render(() => <MetricBar value={50} label="Label" sublabel="Sub" />);
    expect(screen.queryByText('(Sub)')).not.toBeInTheDocument();

    resizeCallback?.([{ contentRect: { width: 200 } } as ResizeObserverEntry], {} as ResizeObserver);
    expect(await screen.findByText('(Sub)')).toBeInTheDocument();
  });
});
