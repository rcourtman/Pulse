import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ResponsiveMetricCell } from '../responsive/ResponsiveMetricCell';

beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('ResponsiveMetricCell observed CPU labels', () => {
  it.each([false, true])('keeps fractions in desktop/mobile mode %s', (showMobile) => {
    const { container } = render(() => (
      <ResponsiveMetricCell value={0.026} type="cpu" showMobile={showMobile} />
    ));
    const numbers = container.querySelectorAll('[data-animated-number]');
    expect(numbers.length).toBeGreaterThan(0);
    for (const label of numbers) {
      expect(label).toHaveTextContent('<0.1%');
      expect(label).toHaveAttribute('aria-label', '<0.1%');
    }
    expect(container.querySelector('[data-progress-fill]')).toHaveAttribute('width', '0.026');
  });

  it('updates the formatted target while retaining raw width and severity', async () => {
    const [cpu, setCpu] = createSignal(0);
    const { container } = render(() => (
      <ResponsiveMetricCell
        value={cpu()}
        type="cpu"
        thresholds={{ warning: 0.18, critical: 0.3 }}
      />
    ));
    expect(screen.getByText('0%')).toBeInTheDocument();
    setCpu(0.17);
    await waitFor(() => expect(screen.getByLabelText('0.2%')).toHaveTextContent('0.2%'));
    const fill = container.querySelector('[data-progress-fill]');
    expect(fill).toHaveAttribute('width', '0.17');
    expect(fill?.firstElementChild?.className).not.toMatch(/orange|red/);
    setCpu(0);
    await waitFor(() => expect(screen.getByLabelText('0%')).toHaveTextContent('0%'));
  });

  it('preserves custom labels, non-CPU formatting and offline fallback', () => {
    const { container } = render(() => (
      <>
        <ResponsiveMetricCell value={0.17} type="cpu" label="Unavailable" />
        <ResponsiveMetricCell value={0.17} type="memory" />
        <ResponsiveMetricCell value={0.17} type="cpu" isRunning={false} />
      </>
    ));
    expect(screen.getByText('Unavailable')).toBeInTheDocument();
    expect(screen.getByLabelText('0%')).toBeInTheDocument();
    expect(screen.getByText('—')).toBeInTheDocument();
    expect(container.querySelectorAll('[data-animated-number]')).toHaveLength(1);
  });
});
