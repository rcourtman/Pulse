import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import { AlertHistoryFrequencyCard } from '../AlertHistoryFrequencyCard';
import type { AlertHistoryState } from '../useAlertHistoryState';

afterEach(cleanup);

const counts = (critical: number, warning: number, info: number) => ({ critical, warning, info });

function createFrequencyCardState(options?: {
  withAxis?: boolean;
  buckets?: number[];
  severityBuckets?: ReturnType<typeof counts>[];
  severities?: string[];
}) {
  const [selectedBarIndex, setSelectedBarIndex] = createSignal<number | null>(null);
  const buckets = options?.buckets ?? [0, 1, 2];
  const state = {
    alertData: () => (options?.severities ?? []).map((severity) => ({ severity })),
    selectedBucketDetails: () => null,
    bucketDurationLabel: () => '1 hour',
    rangeSummary: () => null,
    selectedBarIndex,
    setSelectedBarIndex,
    alertTrends: () => ({
      buckets,
      severityBuckets: options?.severityBuckets ?? buckets.map((value) => counts(0, value, 0)),
      max: Math.max(...buckets, 1),
      bucketTimes: buckets.map((_, index) => index * 60 * 60 * 1000),
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
  it('stacks each period by severity in the legend colours, critical at the base', () => {
    const { state } = createFrequencyCardState({
      buckets: [3, 0],
      severityBuckets: [counts(1, 2, 0), counts(0, 0, 0)],
    });
    const { container } = render(() => <AlertHistoryFrequencyCard state={state} />);

    const bar = container.querySelector(
      '[data-alert-frequency-bucket="0"] [data-alert-frequency-bar]',
    );
    const segments = Array.from(
      bar!.querySelectorAll<HTMLElement>('[data-alert-frequency-segment]'),
    );
    // The bar stacks from the bottom (flex-col-reverse), so the first segment
    // sits on the baseline.
    expect(bar).toHaveClass('flex-col-reverse');
    expect(segments.map((segment) => segment.dataset.alertFrequencySegment)).toEqual([
      'critical',
      'warning',
    ]);
    expect(segments[0]).toHaveClass('bg-red-500');
    expect(segments[1]).toHaveClass('bg-amber-700', 'dark:bg-yellow-500');
    expect(segments[0].style.height).toBe(`${(1 / 3) * 100}%`);
    expect(
      container.querySelectorAll(
        '[data-alert-frequency-bucket="1"] [data-alert-frequency-segment]',
      ),
    ).toHaveLength(0);

    const button = container.querySelector('[data-alert-frequency-bucket="0"]');
    expect(button?.getAttribute('aria-label')).toContain('3 alerts (1 critical, 2 warnings)');
  });

  it('scales bars from the busiest period so a burst stands well clear', () => {
    const { state } = createFrequencyCardState({ buckets: [1, 4, 25, 0] });
    const { container } = render(() => <AlertHistoryFrequencyCard state={state} />);
    const heights = Array.from(
      container.querySelectorAll<HTMLElement>('[data-alert-frequency-bar]'),
    ).map((bar) => bar.style.height);

    // Square root of each period's share of the busiest: 25 alerts read five
    // times taller than one (the old log scale drew 8px, 13px and 26px, so a
    // 25-alert burst looked barely three times one alert).
    expect(heights).toEqual(['8px', '16px', '40px', '0px']);
  });

  it('counts every severity in the legend, info only when present', () => {
    const { state } = createFrequencyCardState({
      severities: ['critical', 'warning', 'warning', 'info', 'Critical'],
    });
    const { container } = render(() => <AlertHistoryFrequencyCard state={state} />);
    const legend = container.querySelector('[data-alert-frequency-legend]');
    expect(legend?.textContent).toBe('2 critical2 warnings1 info');

    cleanup();
    const quiet = createFrequencyCardState({ severities: ['warning'] });
    const view = render(() => <AlertHistoryFrequencyCard state={quiet.state} />);
    expect(view.container.querySelector('[data-alert-frequency-legend]')?.textContent).toBe(
      '0 critical1 warning',
    );
  });

  it('rings the selected period and keeps every other bar at full strength', () => {
    const { state } = createFrequencyCardState();
    const { container } = render(() => <AlertHistoryFrequencyCard state={state} />);
    fireEvent.click(container.querySelector('[data-alert-frequency-bucket="2"]')!);

    // Dimming the rest would drop their fills under 3:1 on the light surface,
    // and they still carry severity and remain clickable.
    const bars = Array.from(container.querySelectorAll('[data-alert-frequency-bar]'));
    expect(bars[2]).toHaveClass('ring-2');
    expect(bars[1]).not.toHaveClass('ring-2');
    for (const bar of bars) expect(bar.className).not.toMatch(/opacity-/);
  });
});
