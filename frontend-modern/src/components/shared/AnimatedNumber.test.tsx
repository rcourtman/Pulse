import { createSignal } from 'solid-js';
import { cleanup, render } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { formatPercent } from '@/utils/format';
import { AnimatedNumber } from './AnimatedNumber';

let queuedFrames: FrameRequestCallback[] = [];
let reducedMotionMatches = false;

function installMotionRuntime() {
  queuedFrames = [];
  Object.defineProperty(window, 'requestAnimationFrame', {
    configurable: true,
    value: vi.fn((callback: FrameRequestCallback) => {
      queuedFrames.push(callback);
      return queuedFrames.length;
    }),
  });
  Object.defineProperty(window, 'cancelAnimationFrame', {
    configurable: true,
    value: vi.fn(),
  });
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: vi.fn(() => ({
      addEventListener: vi.fn(),
      addListener: vi.fn(),
      matches: reducedMotionMatches,
      media: '(prefers-reduced-motion: reduce)',
      removeEventListener: vi.fn(),
      removeListener: vi.fn(),
    })),
  });
}

function runNextFrame(timestamp: number) {
  const callback = queuedFrames.shift();
  if (!callback) {
    throw new Error('No queued animation frame');
  }
  callback(timestamp);
}

function getReadout(container: HTMLElement): HTMLElement {
  const readout = container.querySelector('[data-animated-number="true"]');
  if (!readout) {
    throw new Error('Animated number readout not found');
  }
  return readout as HTMLElement;
}

describe('AnimatedNumber', () => {
  beforeEach(() => {
    reducedMotionMatches = false;
    installMotionRuntime();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders the target value immediately on first paint', () => {
    const { container } = render(() => <AnimatedNumber value={42} format={formatPercent} />);
    const readout = getReadout(container);
    expect(readout).toHaveTextContent('42%');
    expect(readout).toHaveAttribute('aria-label', '42%');
    expect(queuedFrames).toHaveLength(0);
  });

  it('eases changed values through the shared frame loop', () => {
    const [value, setValue] = createSignal(10);
    const { container } = render(() => <AnimatedNumber value={value()} format={formatPercent} />);
    const readout = getReadout(container);

    setValue(50);
    expect(readout).toHaveAttribute('aria-label', '50%');
    expect(readout).toHaveTextContent('10%');

    runNextFrame(0);
    expect(readout).toHaveTextContent('10%');

    runNextFrame(160);
    expect(readout).toHaveTextContent('45%');

    runNextFrame(320);
    expect(readout).toHaveTextContent('50%');
  });

  it('snaps changed values when reduced motion is preferred', () => {
    reducedMotionMatches = true;
    installMotionRuntime();
    const [value, setValue] = createSignal(10);
    const { container } = render(() => <AnimatedNumber value={value()} format={formatPercent} />);
    const readout = getReadout(container);

    setValue(50);
    expect(readout).toHaveTextContent('50%');
    expect(readout).toHaveAttribute('aria-label', '50%');
    expect(queuedFrames).toHaveLength(0);
  });
});
