import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { AlertHistoryFrequencyCard } from '../AlertHistoryFrequencyCard';
import type { AlertHistoryState } from '../useAlertHistoryState';

afterEach(cleanup);

function createFrequencyCardState(options?: { withAxis?: boolean }) {
  const [selectedBarIndex, setSelectedBarIndex] = createSignal<number | null>(null);
  const state = {
    alertData: () => [],
    selectedBucketDetails: () => null,
    bucketDurationLabel: () => '1 hour',
    rangeSummary: () => null,
    selectedBarIndex,
    setSelectedBarIndex,
    alertTrends: () => ({
      buckets: [0, 1, 2],
      bucketTimes: [0, 60 * 60 * 1000, 2 * 60 * 60 * 1000],
      bucketSize: 1,
    }),
    formatBucketRange: (start: number) => `Period ${start}`,
    axisTicks: () =>
      options?.withAxis
        ? [
            { label: 'Start', position: 0, align: 'start' },
            { label: 'End', position: 1, align: 'end' },
          ]
        : [],
  } as unknown as AlertHistoryState;

  return { selectedBarIndex, state };
}

describe('AlertHistoryFrequencyCard', () => {
  it('uses native, visible-focus period buttons with WCAG-sized pointer targets', () => {
    const { state } = createFrequencyCardState();
    const { container } = render(() => <AlertHistoryFrequencyCard state={state} />);

    const group = screen.getByRole('group', {
      name: 'Filter alert history by time period',
    });
    expect(group).toHaveClass('overflow-x-auto');

    const buckets = Array.from(
      container.querySelectorAll<HTMLButtonElement>('[data-alert-frequency-bucket]'),
    );
    expect(buckets).toHaveLength(3);
    for (const bucket of buckets) {
      expect(bucket.tagName).toBe('BUTTON');
      expect(bucket).toHaveAttribute('type', 'button');
      expect(bucket).toHaveClass('h-12');
      expect(bucket).toHaveClass('min-w-6');
      expect(bucket).toHaveClass('focus-visible:ring-2');
    }
  });

  it('selects and clears a period while keeping aria-pressed in sync', () => {
    const { selectedBarIndex, state } = createFrequencyCardState();
    const { container } = render(() => <AlertHistoryFrequencyCard state={state} />);
    const secondBucket = container.querySelector<HTMLButtonElement>(
      '[data-alert-frequency-bucket="1"]',
    );

    expect(secondBucket).not.toBeNull();
    expect(secondBucket).toHaveAttribute('aria-pressed', 'false');
    fireEvent.click(secondBucket!);
    expect(selectedBarIndex()).toBe(1);
    expect(secondBucket).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(secondBucket!);
    expect(selectedBarIndex()).toBeNull();
    expect(secondBucket).toHaveAttribute('aria-pressed', 'false');
  });

  it('keeps the time axis inside the same horizontal scroll surface as the bars', () => {
    const { state } = createFrequencyCardState({ withAxis: true });
    const { container } = render(() => <AlertHistoryFrequencyCard state={state} />);
    const scrollSurface = container.querySelector('[data-alert-frequency-scroll]');
    const axisTicks = screen.getAllByTestId('alert-frequency-axis-tick');

    expect(scrollSurface).not.toBeNull();
    expect(axisTicks).toHaveLength(2);
    expect(axisTicks.every((tick) => scrollSurface!.contains(tick))).toBe(true);
  });
});
