import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import {
  Tooltip,
  createTooltipSystem,
  showTooltip,
  hideTooltip,
} from '@/components/shared/Tooltip';
import tooltipSource from '@/components/shared/Tooltip.tsx?raw';
import tooltipPortalSource from '@/components/shared/TooltipPortal.tsx?raw';
import tooltipModelSource from '@/components/shared/tooltipModel.ts?raw';
import tooltipStateSource from '@/components/shared/useTooltipState.ts?raw';

describe('Tooltip', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('keeps tooltip on shell, runtime, and model owners', () => {
    expect(tooltipSource).toContain('useTooltipState');
    expect(tooltipSource).toContain('createTooltipSystemState');
    expect(tooltipSource).toContain('foreignObject');
    expect(tooltipSource).not.toContain('createSignal');
    expect(tooltipSource).not.toContain('requestAnimationFrame');
    expect(tooltipSource).not.toContain('sanitizeTooltipContent');
    expect(tooltipSource).not.toContain('resolveTooltipPosition');
    expect(tooltipSource).not.toContain('style={');

    expect(tooltipPortalSource).toContain('useTooltipPortalState');
    expect(tooltipPortalSource).toContain('foreignObject');
    expect(tooltipPortalSource).not.toContain('createSignal');
    expect(tooltipPortalSource).not.toContain('resolveTooltipPosition');
    expect(tooltipPortalSource).not.toContain('style={');

    expect(tooltipStateSource).toContain('export function useTooltipState');
    expect(tooltipStateSource).toContain('export function useTooltipPortalState');
    expect(tooltipStateSource).toContain('export function createTooltipSystemState');
    expect(tooltipStateSource).toContain('createSignal');
    expect(tooltipStateSource).toContain('requestAnimationFrame');
    expect(tooltipStateSource).toContain('tooltipInstance');
    expect(tooltipStateSource).toContain('resolveTooltipPosition');
    expect(tooltipStateSource).toContain('sanitizeTooltipContent');

    expect(tooltipModelSource).toContain('export function sanitizeTooltipContent');
    expect(tooltipModelSource).toContain('export function resolveTooltipPosition');
    expect(tooltipModelSource).toContain("export type TooltipAlignment = 'left' | 'center'");
    expect(tooltipModelSource).toContain("export type TooltipDirection = 'up' | 'down'");
  });

  it('sanitizes tooltip content through the model owner', async () => {
    render(() => <Tooltip content={`<b>"unsafe"</b> & 'quoted'`} x={24} y={24} visible />);

    const tooltip = document.body.querySelector('[data-tooltip="true"]') as HTMLDivElement | null;
    expect(tooltip).not.toBeNull();
    if (!tooltip) return;

    await waitFor(() => {
      expect(tooltip.textContent).toBe('&quot;unsafe&quot; &amp; &#x27;quoted&#x27;');
      expect(tooltip.innerHTML).not.toContain('<b>');
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
});
