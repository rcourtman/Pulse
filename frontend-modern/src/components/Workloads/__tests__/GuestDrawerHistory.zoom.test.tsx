import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ChartsAPI, type AllMetricsHistoryResponse, type HistoryTimeRange } from '@/api/charts';
import { HOST_METRICS_HISTORY_GROUPS } from '@/components/shared/hostMetricsHistoryModel';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { GuestDrawerHistory } from '../GuestDrawerHistory';
import type { GuestDrawerHistoryTarget } from '../guestDrawerModel';
import { GUEST_DRAWER_HISTORY_POLL_MS } from '../guestDrawerHistoryQuery';

vi.mock('@/stores/license', () => ({
  loadRuntimeCapabilities: vi.fn(async () => undefined),
}));

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

const baseStart = Date.UTC(2026, 9, 10, 12, 0, 0);
const baseEnd = baseStart + 24 * 3_600_000;
const target: GuestDrawerHistoryTarget = { resourceType: 'agent', resourceId: 'host-zoom' };
const pt = (timestamp: number, value: number) => ({ timestamp, value, min: value, max: value });

const presetResponse = (
  overrides: Partial<AllMetricsHistoryResponse> = {},
): AllMetricsHistoryResponse => ({
  ...target,
  range: '24h',
  start: baseStart,
  end: baseEnd,
  source: 'store',
  metrics: {
    cpu: [
      pt(baseStart + 3_600_000, 15),
      pt(baseStart + 12 * 3_600_000, 82),
      pt(baseEnd - 60_000, 20),
    ],
    memory: [pt(baseStart + 3_600_000, 40), pt(baseEnd - 60_000, 45)],
    netin: [pt(baseStart + 3_600_000, 1000), pt(baseEnd - 60_000, 4000)],
  },
  ...overrides,
});

const customResponse = (
  start: number,
  end: number,
  metrics: AllMetricsHistoryResponse['metrics'] = {
    cpu: [pt(start + 10_000, 65), pt(end - 10_000, 91)],
    netin: [pt(start + 10_000, 2048), pt(end - 10_000, 8192)],
  },
): AllMetricsHistoryResponse => ({
  ...target,
  range: 'custom',
  start,
  end,
  source: 'store',
  metrics,
});

const mockPlotRect = (plot: Element) => {
  vi.spyOn(plot, 'getBoundingClientRect').mockReturnValue({
    left: 0,
    top: 0,
    width: 360,
    height: 92,
    right: 360,
    bottom: 92,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  } as DOMRect);
};

// Map fraction [0, 1] inside the drawable plot [34, 352] to clientX (width = 360).
const plotX = (fraction: number) => 34 + fraction * 318;

// jsdom lacks PointerEvent, so fireEvent.pointer* drops clientX/button/pointerId.
// Build a bubbling MouseEvent with PointerEvent fields attached.
const dispatchPointer = (
  target: Element,
  type: 'pointerdown' | 'pointermove' | 'pointerup' | 'pointercancel',
  init: {
    pointerId?: number;
    pointerType?: string;
    button?: number;
    isPrimary?: boolean;
    clientX?: number;
  } = {},
) => {
  const event = new MouseEvent(type, {
    bubbles: true,
    cancelable: true,
    button: init.button ?? 0,
    clientX: init.clientX ?? 0,
  });
  Object.defineProperties(event, {
    pointerId: { value: init.pointerId ?? 1 },
    pointerType: { value: init.pointerType ?? 'mouse' },
    isPrimary: { value: init.isPrimary ?? true },
  });
  fireEvent(target, event);
};

describe('GuestDrawerHistory drag-to-zoom and custom time window', () => {
  it('previews brush across all panels with zero requests during drag and fetches once on release', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(presetResponse())
      .mockImplementationOnce(async (params) => {
        if (params.start === undefined || params.end === undefined) {
          throw new Error('Expected custom window');
        }
        return customResponse(params.start, params.end);
      });

    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    render(() => (
      <GuestDrawerHistory target={target} range="24h" groups={HOST_METRICS_HISTORY_GROUPS} />
    ));
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    expect(fetch).toHaveBeenCalledTimes(1);

    const plots = screen.getAllByTestId('guest-history-plot');
    const utilPlot = plots[0];
    mockPlotRect(utilPlot);

    dispatchPointer(utilPlot, 'pointerdown', {
      pointerId: 1,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
      clientX: plotX(0.25),
    });
    dispatchPointer(utilPlot, 'pointermove', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });

    // Secondary touch pointercancel must NOT cancel the active mouse brush.
    dispatchPointer(utilPlot, 'pointercancel', { pointerId: 99, pointerType: 'touch' });
    expect(document.querySelectorAll('[data-history-brush-selection]')).toHaveLength(plots.length);
    expect(screen.getAllByTestId('guest-history-brush-preview')).toHaveLength(1);
    // Zero network calls during drag movement and zero SolidJS owner warnings.
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(warnSpy).not.toHaveBeenCalled();

    dispatchPointer(utilPlot, 'pointerup', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });

    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    const zoomCall = fetch.mock.calls[1][0];
    expect(zoomCall.start).toBe(baseStart + 6 * 3_600_000);
    expect(zoomCall.end).toBe(baseStart + 12 * 3_600_000);
    expect(zoomCall.range).toBeUndefined();

    expect(screen.getByTestId('guest-history-zoom-bar')).toBeInTheDocument();
    expect(screen.getByTestId('guest-history-reset-zoom')).toBeInTheDocument();
    expect(document.querySelectorAll('[data-history-brush-selection]')).toHaveLength(0);
  });

  it('supports reverse drag, ignores undersized drags, and cancels on Escape without closing drawer', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(presetResponse())
      .mockImplementationOnce(async (params) => customResponse(params.start!, params.end!));

    const drawerEscapeSpy = vi.fn();
    document.addEventListener('keydown', drawerEscapeSpy);
    try {
      render(() => <GuestDrawerHistory target={target} range="24h" />);
      await screen.findByRole('slider', { name: 'Inspect Utilization history' });
      const plot = screen.getAllByTestId('guest-history-plot')[0];
      mockPlotRect(plot);

      // Escape from a focused descendant inside the drawer cancels brush AND
      // stops propagation before the table's document Escape handler.
      const region = screen.getByTestId('guest-history-region');
      region.focus();
      dispatchPointer(plot, 'pointerdown', {
        pointerId: 2,
        pointerType: 'mouse',
        button: 0,
        isPrimary: true,
        clientX: plotX(0.2),
      });
      dispatchPointer(plot, 'pointermove', {
        pointerId: 2,
        pointerType: 'mouse',
        clientX: plotX(0.6),
      });
      expect(screen.getByTestId('guest-history-brush-preview')).toBeInTheDocument();
      fireEvent.keyDown(region, { key: 'Escape', bubbles: true, cancelable: true });
      expect(screen.queryByTestId('guest-history-brush-preview')).toBeNull();
      expect(drawerEscapeSpy).not.toHaveBeenCalled();
      expect(fetch).toHaveBeenCalledTimes(1);

      // Sub-threshold drag (< 6px) does not zoom.
      dispatchPointer(plot, 'pointerdown', {
        pointerId: 3,
        pointerType: 'mouse',
        button: 0,
        isPrimary: true,
        clientX: plotX(0.3),
      });
      dispatchPointer(plot, 'pointermove', {
        pointerId: 3,
        pointerType: 'mouse',
        clientX: plotX(0.3) + 3,
      });
      dispatchPointer(plot, 'pointerup', {
        pointerId: 3,
        pointerType: 'mouse',
        clientX: plotX(0.3) + 3,
      });
      expect(fetch).toHaveBeenCalledTimes(1);

      // Touch drag does not start brush.
      dispatchPointer(plot, 'pointerdown', {
        pointerId: 4,
        pointerType: 'touch',
        button: 0,
        isPrimary: true,
        clientX: plotX(0.2),
      });
      dispatchPointer(plot, 'pointermove', {
        pointerId: 4,
        pointerType: 'touch',
        clientX: plotX(0.6),
      });
      expect(screen.queryByTestId('guest-history-brush-preview')).toBeNull();

      // Reverse drag (right to left) sorts endpoints cleanly and zooms.
      dispatchPointer(plot, 'pointerdown', {
        pointerId: 5,
        pointerType: 'mouse',
        button: 0,
        isPrimary: true,
        clientX: plotX(0.75),
      });
      dispatchPointer(plot, 'pointermove', {
        pointerId: 5,
        pointerType: 'mouse',
        clientX: plotX(0.5),
      });
      dispatchPointer(plot, 'pointerup', {
        pointerId: 5,
        pointerType: 'mouse',
        clientX: plotX(0.5),
      });
      await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
      expect(fetch.mock.calls[1][0].start).toBe(baseStart + 12 * 3_600_000);
      expect(fetch.mock.calls[1][0].end).toBe(baseStart + 18 * 3_600_000);
    } finally {
      document.removeEventListener('keydown', drawerEscapeSpy);
    }
  });

  it('pauses polling while zoomed, suppresses live fallback on empty zoomed window, and resets via button or dblclick', async () => {
    vi.useFakeTimers();
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(presetResponse())
      .mockImplementationOnce(async (params) => customResponse(params.start!, params.end!, {}))
      .mockResolvedValue(presetResponse());

    render(() => (
      <GuestDrawerHistory target={target} range="24h" currentMetrics={{ cpu: 99, memory: 88 }} />
    ));
    await vi.advanceTimersByTimeAsync(0);
    const plot = screen.getAllByTestId('guest-history-plot')[0];
    mockPlotRect(plot);

    dispatchPointer(plot, 'pointerdown', {
      pointerId: 1,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
      clientX: plotX(0.25),
    });
    dispatchPointer(plot, 'pointermove', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });
    dispatchPointer(plot, 'pointerup', { pointerId: 1, pointerType: 'mouse', clientX: plotX(0.5) });
    await vi.advanceTimersByTimeAsync(0);

    expect(fetch).toHaveBeenCalledTimes(2);
    expect(screen.getAllByText('No stored history in this range').length).toBeGreaterThan(0);
    // Live current fallback must be suppressed inside a zoomed historical window.
    expect(screen.getAllByTestId('guest-history-group-chart')[0]).not.toHaveTextContent('99.0%');

    // Polling stays paused while zoomed.
    await vi.advanceTimersByTimeAsync(GUEST_DRAWER_HISTORY_POLL_MS * 2);
    expect(fetch).toHaveBeenCalledTimes(2);

    // Double-click resets zoom and resumes polling.
    fireEvent.dblClick(plot);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.queryByTestId('guest-history-zoom-bar')).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(3);
    expect(fetch.mock.calls[2][0].range).toBe('24h');
  });

  it('preserves zoom across snapshot target replacement and clears synchronously on resource or preset change', async () => {
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(presetResponse())
      .mockImplementationOnce(async (params) => customResponse(params.start!, params.end!))
      .mockResolvedValueOnce(presetResponse({ range: '6h' }))
      .mockImplementationOnce(async (params) => customResponse(params.start!, params.end!))
      .mockResolvedValueOnce(presetResponse({ resourceId: 'host-other', range: '6h' }));

    const [currentTarget, setCurrentTarget] = createSignal<GuestDrawerHistoryTarget>({ ...target });
    const [range, setRange] = createSignal<HistoryTimeRange>('24h');

    render(() => <GuestDrawerHistory target={currentTarget()} range={range()} />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const plot = screen.getAllByTestId('guest-history-plot')[0];
    mockPlotRect(plot);

    dispatchPointer(plot, 'pointerdown', {
      pointerId: 1,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
      clientX: plotX(0.25),
    });
    dispatchPointer(plot, 'pointermove', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });
    dispatchPointer(plot, 'pointerup', { pointerId: 1, pointerType: 'mouse', clientX: plotX(0.5) });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));

    // Replacing target object with identical coordinates keeps zoom and does NOT refetch.
    setCurrentTarget({ resourceType: 'agent', resourceId: 'host-zoom' });
    expect(screen.getByTestId('guest-history-zoom-bar')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledTimes(2);

    // Switching preset range while zoomed clears zoom synchronously and requests the new preset.
    setRange('6h');
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
    expect(fetch.mock.calls[2][0].range).toBe('6h');
    expect(fetch.mock.calls[2][0].start).toBeUndefined();
    expect(screen.queryByTestId('guest-history-zoom-bar')).toBeNull();
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });

    // Zoom again under 6h, then switch resource: zoom clears synchronously without issuing old window.
    dispatchPointer(plot, 'pointerdown', {
      pointerId: 2,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
      clientX: plotX(0.25),
    });
    dispatchPointer(plot, 'pointermove', {
      pointerId: 2,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });
    dispatchPointer(plot, 'pointerup', { pointerId: 2, pointerType: 'mouse', clientX: plotX(0.5) });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(4));
    expect(screen.getByTestId('guest-history-zoom-bar')).toBeInTheDocument();

    setCurrentTarget({ resourceType: 'agent', resourceId: 'host-other' });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(5));
    expect(fetch.mock.calls[4][0].resourceId).toBe('host-other');
    expect(fetch.mock.calls[4][0].range).toBe('6h');
    expect(fetch.mock.calls[4][0].start).toBeUndefined();
    expect(screen.queryByTestId('guest-history-zoom-bar')).toBeNull();
  });

  it('allows non-drag selection from an empty preset, preserves exact unchanged instants, and keeps keyboard focus on Apply', async () => {
    const exactStart = Date.UTC(2026, 9, 10, 12, 10, 0, 123);
    const exactEnd = Date.UTC(2026, 9, 10, 13, 10, 0, 456);
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(presetResponse({ start: exactStart, end: exactEnd, metrics: {} }))
      .mockImplementationOnce(async (params) => customResponse(params.start!, params.end!))
      .mockImplementationOnce(async (params) => customResponse(params.start!, params.end!))
      .mockResolvedValueOnce(presetResponse());

    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findAllByText('No stored history in this range');

    // Empty preset still exposes Select time window so users can inspect older incidents.
    fireEvent.click(screen.getByTestId('guest-history-select-window'));
    const applyBtn = screen.getByTestId('guest-history-window-apply');
    applyBtn.focus();
    fireEvent.click(applyBtn);

    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    // Reapplying unchanged endpoints preserves exact original milliseconds.
    expect(fetch.mock.calls[1][0].start).toBe(exactStart);
    expect(fetch.mock.calls[1][0].end).toBe(exactEnd);
    // Focus moves cleanly to Reset zoom instead of dropping to document.body.
    await waitFor(() => expect(screen.getByTestId('guest-history-reset-zoom')).toHaveFocus());

    // Edit inputs to validate < 1m error and submit an edited interval.
    fireEvent.click(screen.getByTestId('guest-history-select-window'));
    const editor = screen.getByTestId('guest-history-window-editor');
    const startInput = within(editor).getByLabelText('Start') as HTMLInputElement;
    const endInput = within(editor).getByLabelText('End') as HTMLInputElement;

    fireEvent.input(startInput, { target: { value: '2026-10-10T14:00:00' } });
    fireEvent.input(endInput, { target: { value: '2026-10-10T14:00:30' } });
    fireEvent.click(screen.getByTestId('guest-history-window-apply'));
    expect(screen.getByTestId('guest-history-window-error')).toHaveTextContent('at least 1 minute');
    expect(fetch).toHaveBeenCalledTimes(2);

    fireEvent.input(startInput, { target: { value: '2026-10-10T14:00:00.125' } });
    fireEvent.input(endInput, { target: { value: '2026-10-10T14:30:00.375' } });
    expect(startInput.validity.stepMismatch).toBe(false);
    expect(endInput.validity.stepMismatch).toBe(false);
    fireEvent.click(screen.getByTestId('guest-history-window-apply'));
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
    expect(fetch.mock.calls[2][0].start).toBe(new Date('2026-10-10T14:00:00.125').getTime());
    expect(fetch.mock.calls[2][0].end).toBe(new Date('2026-10-10T14:30:00.375').getTime());

    // Reset button restores preset.
    fireEvent.click(screen.getByTestId('guest-history-reset-zoom'));
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(4));
    expect(fetch.mock.calls[3][0].range).toBe('24h');
  });

  it('aborts an in-flight zoom request when Reset zoom is clicked before response settles and keeps preset data intact', async () => {
    let resolveZoom!: (value: AllMetricsHistoryResponse) => void;
    const pendingZoom = new Promise<AllMetricsHistoryResponse>((resolve) => {
      resolveZoom = resolve;
    });
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(presetResponse())
      .mockReturnValueOnce(pendingZoom)
      .mockResolvedValueOnce(presetResponse());

    const { container } = render(() => <GuestDrawerHistory target={target} range="24h" />);
    await screen.findByRole('slider', { name: 'Inspect Utilization history' });
    const presetPath = container
      .querySelector('[data-testid="guest-history-plot"] path[data-history-series="cpu"]')!
      .getAttribute('d');
    const plot = screen.getAllByTestId('guest-history-plot')[0];
    mockPlotRect(plot);

    dispatchPointer(plot, 'pointerdown', {
      pointerId: 1,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
      clientX: plotX(0.25),
    });
    dispatchPointer(plot, 'pointermove', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });
    dispatchPointer(plot, 'pointerup', { pointerId: 1, pointerType: 'mouse', clientX: plotX(0.5) });

    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    const zoomSignal = fetch.mock.calls[1][0].signal;
    expect(zoomSignal?.aborted).toBe(false);

    // Click Reset zoom before the custom request resolves.
    fireEvent.click(screen.getByTestId('guest-history-reset-zoom'));
    expect(zoomSignal?.aborted).toBe(true);
    expect(screen.queryByTestId('guest-history-zoom-bar')).toBeNull();

    // Resolving the aborted request late with different values must not overwrite the preset chart.
    resolveZoom(
      customResponse(baseStart + 6 * 3_600_000, baseStart + 12 * 3_600_000, {
        cpu: [pt(baseStart + 6 * 3_600_000, 99), pt(baseStart + 12 * 3_600_000, 99)],
      }),
    );
    await Promise.resolve();
    await waitFor(() =>
      expect(
        container
          .querySelector('[data-testid="guest-history-plot"] path[data-history-series="cpu"]')
          ?.getAttribute('d'),
      ).toBe(presetPath),
    );
    expect(screen.queryByTestId('guest-history-zoom-bar')).toBeNull();
  });

  it('cancels an active brush drag cleanly if a background poll shifts window bounds mid-drag', async () => {
    vi.useFakeTimers();
    const shift = 5 * 60_000;
    const fetch = vi
      .spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce(presetResponse())
      .mockResolvedValueOnce(presetResponse({ start: baseStart + shift, end: baseEnd + shift }));

    render(() => <GuestDrawerHistory target={target} range="24h" />);
    await vi.advanceTimersByTimeAsync(0);
    const plot = screen.getAllByTestId('guest-history-plot')[0];
    mockPlotRect(plot);

    dispatchPointer(plot, 'pointerdown', {
      pointerId: 1,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
      clientX: plotX(0.2),
    });
    dispatchPointer(plot, 'pointermove', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.6),
    });
    expect(screen.getByTestId('guest-history-brush-preview')).toBeInTheDocument();

    // Background poll lands while drag is held.
    await vi.advanceTimersByTimeAsync(GUEST_DRAWER_HISTORY_POLL_MS);
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(screen.queryByTestId('guest-history-brush-preview')).toBeNull();

    // Releasing the mouse after poll cancellation must NOT commit a zoom.
    dispatchPointer(plot, 'pointerup', { pointerId: 1, pointerType: 'mouse', clientX: plotX(0.6) });
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(screen.queryByTestId('guest-history-zoom-bar')).toBeNull();
  });

  it('keeps disk phone fallback working when zoomed to a custom window', async () => {
    const diskTarget: GuestDrawerHistoryTarget = { resourceType: 'disk', resourceId: 'disk-1' };
    const diskGroups = [
      {
        id: 'disk-activity',
        label: 'Activity',
        unit: '%',
        series: [{ metric: 'disk', label: 'Utilization', unit: '%', color: '#10b981' }],
      },
      {
        id: 'disk-thermal',
        label: 'Temperature',
        unit: 'C',
        series: [{ metric: 'smart_temp', label: 'Temp', unit: 'C', color: '#ef4444' }],
      },
    ];
    vi.spyOn(ChartsAPI, 'getMetricsHistory')
      .mockResolvedValueOnce({
        ...diskTarget,
        range: '24h',
        start: baseStart,
        end: baseEnd,
        source: 'store',
        metrics: { smart_temp: [pt(baseStart + 3_600_000, 35), pt(baseEnd - 60_000, 39)] },
      })
      .mockImplementationOnce(async (params) => ({
        ...diskTarget,
        range: 'custom',
        start: params.start!,
        end: params.end!,
        source: 'store',
        metrics: { smart_temp: [pt(params.start! + 60_000, 36), pt(params.end! - 60_000, 41)] },
      }));

    render(() => <GuestDrawerHistory target={diskTarget} range="24h" groups={diskGroups} />);
    await screen.findByRole('slider', { name: 'Inspect Temperature history' });

    const charts = screen.getAllByTestId('guest-history-group-chart');
    const thermalChart = charts.find((el) => el.dataset.historyGroup === 'disk-thermal')!;
    const activityChart = charts.find((el) => el.dataset.historyGroup === 'disk-activity')!;
    expect(thermalChart).not.toHaveAttribute('data-phone-collapsed', 'true');
    expect(activityChart).toHaveAttribute('data-phone-collapsed', 'true');

    const thermalPlot = within(thermalChart).getByTestId('guest-history-plot');
    mockPlotRect(thermalPlot);
    dispatchPointer(thermalPlot, 'pointerdown', {
      pointerId: 1,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
      clientX: plotX(0.25),
    });
    dispatchPointer(thermalPlot, 'pointermove', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });
    dispatchPointer(thermalPlot, 'pointerup', {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: plotX(0.5),
    });

    await screen.findByTestId('guest-history-zoom-bar');
    // Thermal remains the promoted phone panel once the custom-window reply settles.
    await waitFor(() => {
      expect(thermalChart).not.toHaveAttribute('data-phone-collapsed', 'true');
      expect(activityChart).toHaveAttribute('data-phone-collapsed', 'true');
    });
  });
});
