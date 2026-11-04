import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import {
  Tooltip,
  createTooltipSystem,
  showPressedTooltip,
  showTooltip,
  hideTooltip,
} from '@/components/shared/Tooltip';

describe('Tooltip', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('suppresses the singleton tooltip on a coarse touch pointer', () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: false,
        media: '(hover: hover) and (pointer: fine)',
      }),
    );

    const TooltipRoot = createTooltipSystem();
    render(() => <TooltipRoot />);

    showTooltip('touch detail', 120, 80);
    expect(screen.queryByText('touch detail')).toBeNull();
  });

  it('sanitizes tooltip content through the model owner', async () => {
    render(() => <Tooltip content={`<b>"unsafe"</b> & 'quoted'`} x={24} y={24} visible />);

    const tooltip = document.body.querySelector('[data-tooltip="true"]') as HTMLDivElement | null;
    expect(tooltip).not.toBeNull();
    if (!tooltip) return;

    await waitFor(() => {
      expect(tooltip.textContent).toBe(`<b>"unsafe"</b> & 'quoted'`);
      expect(tooltip.innerHTML).not.toContain('<b>');
      expect(tooltip.innerHTML).toContain('&lt;b&gt;');
    });
  });

  it('clamps tooltip position inside the viewport', async () => {
    const getBoundingClientRect = vi
      .spyOn(HTMLDivElement.prototype, 'getBoundingClientRect')
      .mockReturnValue({
        width: 180,
        height: 60,
        left: 0,
        top: 0,
        right: 180,
        bottom: 60,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      } as DOMRect);

    const raf = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation((callback: FrameRequestCallback) => {
        callback(0);
        return 1;
      });

    render(() => <Tooltip content="CPU" x={2} y={2} visible />);

    const tooltip = document.body.querySelector('[data-tooltip="true"]') as HTMLDivElement | null;
    expect(tooltip).not.toBeNull();
    if (!tooltip) return;

    await waitFor(() => {
      const portal = tooltip.closest('foreignObject');
      expect(portal).not.toBeNull();
      expect(Number.parseFloat(portal?.getAttribute('x') ?? '0')).toBeGreaterThanOrEqual(4);
      expect(Number.parseFloat(portal?.getAttribute('y') ?? '0')).toBeGreaterThanOrEqual(4);
    });

    expect(raf).toHaveBeenCalled();
    expect(getBoundingClientRect).toHaveBeenCalled();
  });

  it('preserves the singleton tooltip API', async () => {
    const TooltipRoot = createTooltipSystem();
    render(() => <TooltipRoot />);

    showTooltip('disk', 120, 80, { direction: 'down' });
    expect(await screen.findByText('disk')).toBeInTheDocument();

    hideTooltip();

    await waitFor(() => {
      expect(screen.queryByText('disk')).toBeNull();
    });
  });

  describe('showPressedTooltip', () => {
    const useTouchScreen = () =>
      vi.stubGlobal(
        'matchMedia',
        vi.fn().mockReturnValue({ matches: false, media: '(hover: hover) and (pointer: fine)' }),
      );

    afterEach(() => {
      vi.unstubAllGlobals();
      hideTooltip();
    });

    it('shows on a touch screen and outlasts the synthesized hover events', () => {
      useTouchScreen();
      const TooltipRoot = createTooltipSystem();
      render(() => <TooltipRoot />);

      showPressedTooltip('why', document.body);
      expect(screen.getByText('why')).toBeInTheDocument();

      hideTooltip();
      showTooltip('hover detail', 1, 1);
      expect(screen.getByText('why')).toBeInTheDocument();
      expect(screen.queryByText('hover detail')).toBeNull();

      fireEvent.pointerDown(document.body);
      expect(screen.queryByText('why')).toBeNull();
    });

    it('ends on a scroll anywhere, since it no longer sits by its target', () => {
      useTouchScreen();
      const TooltipRoot = createTooltipSystem();
      const { container } = render(() => (
        <>
          <TooltipRoot />
          <div data-testid="scroller" />
        </>
      ));

      showPressedTooltip('why', document.body);
      fireEvent.scroll(container.querySelector('[data-testid="scroller"]') as Element);
      expect(screen.queryByText('why')).toBeNull();
    });

    it('ends on mouse leave where the pointer can hover', () => {
      const TooltipRoot = createTooltipSystem();
      render(() => <TooltipRoot />);

      showPressedTooltip('why', document.body);
      expect(screen.getByText('why')).toBeInTheDocument();

      hideTooltip();
      expect(screen.queryByText('why')).toBeNull();
    });
  });
});
