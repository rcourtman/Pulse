import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import tooltipPortalSource from '@/components/shared/TooltipPortal.tsx?raw';
import tooltipStateSource from '@/components/shared/useTooltipState.ts?raw';

describe('TooltipPortal', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('keeps tooltip portal on shell and runtime owners', () => {
    expect(tooltipPortalSource).toContain('useTooltipPortalState');
    expect(tooltipPortalSource).toContain('foreignObject');
    expect(tooltipPortalSource).not.toContain('createSignal');
    expect(tooltipPortalSource).not.toContain('resolveTooltipPosition');
    expect(tooltipPortalSource).not.toContain('style={');

    expect(tooltipStateSource).toContain('export function useTooltipPortalState');
    expect(tooltipStateSource).toContain('resolveTooltipPosition');
  });

  it('renders portal content without inline styles', async () => {
    const raf = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation((callback: FrameRequestCallback) => {
        callback(0);
        return 1;
      });

    render(() => (
      <TooltipPortal when x={120} y={80}>
        <span>Memory Composition</span>
      </TooltipPortal>
    ));

    await waitFor(() => {
      expect(document.body.querySelector('[data-tooltip-portal="true"]')).not.toBeNull();
    });

    const tooltip = document.body.querySelector('[data-tooltip-portal="true"]') as HTMLElement | null;
    const portal = tooltip?.closest('foreignObject');
    expect(tooltip?.textContent).toContain('Memory Composition');
    expect(Number.parseFloat(portal?.getAttribute('x') ?? '0')).toBeGreaterThan(0);
    expect(Number.parseFloat(portal?.getAttribute('y') ?? '0')).toBeGreaterThan(0);
    expect(document.body.querySelector('[style]')).toBeNull();
    expect(raf).toHaveBeenCalled();
  });

  it('repositions when a hidden portal becomes visible with live coordinates', async () => {
    const raf = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation((callback: FrameRequestCallback) => {
        callback(0);
        return 1;
      });

    const getBoundingClientRect = vi
      .spyOn(HTMLDivElement.prototype, 'getBoundingClientRect')
      .mockReturnValue({
        width: 180,
        height: 40,
        left: 0,
        top: 0,
        right: 180,
        bottom: 40,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      } as DOMRect);

    const [when, setWhen] = createSignal(false);
    const [x, setX] = createSignal(0);
    const [y, setY] = createSignal(0);

    render(() => (
      <TooltipPortal when={when()} x={x()} y={y()}>
        <span>Memory Composition</span>
      </TooltipPortal>
    ));

    setX(240);
    setY(180);
    setWhen(true);

    await waitFor(() => {
      const tooltip = document.body.querySelector('[data-tooltip-portal="true"]');
      const portal = tooltip?.closest('foreignObject');
      expect(tooltip).not.toBeNull();
      expect(Number.parseFloat(portal?.getAttribute('x') ?? '0')).toBeGreaterThan(0);
      expect(Number.parseFloat(portal?.getAttribute('y') ?? '0')).toBeGreaterThan(0);
    });

    expect(raf).toHaveBeenCalled();
    expect(getBoundingClientRect).toHaveBeenCalled();
  });
});
