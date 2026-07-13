/**
 * Branch-coverage tests for canvasRenderQueue.ts — setupCanvasDPR pass.
 *
 * The sibling canvasRenderQueue.test.ts only exercises the
 * scheduleSparkline/flush rAF scheduler; it never touches setupCanvasDPR.
 * This file targets every branch of setupCanvasDPR:
 *
 * - `window.devicePixelRatio || 1`: the truthy arm (real DPR) and the falsy
 *   fallback arm for both `undefined` and `0` (every value the `||` would
 *   surrender to the right-hand `1`).
 * - `canvas.width !== targetWidth || canvas.height !== targetHeight`: the
 *   short-circuit-left-truthy arm (only width differs), the left-falsy /
 *   right-truthy arm (only height differs), the both-differ arm, and the
 *   both-falsy arm (no resize needed).
 * - `if (needsResize)`: the resize block (mutates backing size + CSS size)
 *   versus the skip arm.
 * - `if (!needsResize)`: the clearRect arm (canvas is cleared in logical
 *   pixels) versus the skip arm (resize already wiped the canvas).
 * - `Math.round(w * dpr)` rounding on a fractional product, and the w=0/h=0
 *   boundary.
 *
 * setupCanvasDPR reads `window.devicePixelRatio` at call time (not import
 * time), so a static import is safe regardless of the DPR stub in force.
 * jsdom does not implement CanvasRenderingContext2D, so the 2D context is a
 * minimal structural double carrying `setTransform`/`clearRect` spies cast
 * through the real type — the canvas element itself is a genuine
 * HTMLCanvasElement from document.createElement, so every width/height/style
 * assertion is against real DOM state.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { setupCanvasDPR } from '@/utils/canvasRenderQueue';

type CanvasAndCtx = {
  canvas: HTMLCanvasElement;
  ctx: CanvasRenderingContext2D;
  setTransform: ReturnType<typeof vi.fn>;
  clearRect: ReturnType<typeof vi.fn>;
};

/**
 * Build a real HTMLCanvasElement paired with a minimal 2D-context double.
 * jsdom returns null from getContext('2d'), so the context is a structural
 * stand-in carrying the two methods setupCanvasDPR actually calls.
 */
function makeCanvasAndCtx(): CanvasAndCtx {
  const canvas = document.createElement('canvas');
  const setTransform = vi.fn();
  const clearRect = vi.fn();
  const ctx = { setTransform, clearRect } as unknown as CanvasRenderingContext2D;
  return { canvas, ctx, setTransform, clearRect };
}

const originalDprDescriptor = Object.getOwnPropertyDescriptor(window, 'devicePixelRatio');

function setDevicePixelRatio(value: number | undefined): void {
  // Define as an own, configurable, writable value property so it shadows any
  // Window.prototype getter jsdom installs.
  Object.defineProperty(window, 'devicePixelRatio', {
    configurable: true,
    writable: true,
    value,
  });
}

describe('setupCanvasDPR (branch coverage)', () => {
  beforeEach(() => {
    // jsdom ships devicePixelRatio = 1; pin a known baseline per test.
    setDevicePixelRatio(1);
  });

  afterEach(() => {
    if (originalDprDescriptor) {
      Object.defineProperty(window, 'devicePixelRatio', originalDprDescriptor);
    } else {
      // No own descriptor to restore from -> drop our shadow and let the
      // prototype getter resurface.
      Reflect.deleteProperty(window, 'devicePixelRatio');
    }
    vi.restoreAllMocks();
  });

  it('resizes both dimensions and applies the DPR scale when nothing matches yet (dpr truthy arm, both-differ arm)', () => {
    setDevicePixelRatio(2);
    const { canvas, ctx, setTransform, clearRect } = makeCanvasAndCtx();

    // Fresh jsdom canvas is 300x150; targets at dpr 2 are 200x100.
    expect(canvas.width).toBe(300);
    expect(canvas.height).toBe(150);

    setupCanvasDPR(canvas, ctx, 100, 50);

    expect(canvas.width).toBe(200);
    expect(canvas.height).toBe(100);
    expect(canvas.style.width).toBe('100px');
    expect(canvas.style.height).toBe('50px');
    // Resize path is taken, so the explicit clear is skipped (resize wipes it).
    expect(setTransform).toHaveBeenCalledTimes(1);
    expect(setTransform).toHaveBeenCalledWith(2, 0, 0, 2, 0, 0);
    expect(clearRect).not.toHaveBeenCalled();
  });

  it('skips the resize and clears in logical pixels when backing size already matches (needsResize false + clearRect arm)', () => {
    setDevicePixelRatio(2);
    const { canvas, ctx, setTransform, clearRect } = makeCanvasAndCtx();

    // Pre-size the backing store to the exact dpr-scaled targets so neither
    // operand of the `||` is truthy.
    canvas.width = 200;
    canvas.height = 100;

    setupCanvasDPR(canvas, ctx, 100, 50);

    // No resize: backing size and CSS size are untouched.
    expect(canvas.width).toBe(200);
    expect(canvas.height).toBe(100);
    expect(canvas.style.width).toBe('');
    expect(canvas.style.height).toBe('');
    // setTransform always runs; clearRect runs because the resize did not.
    expect(setTransform).toHaveBeenCalledTimes(1);
    expect(setTransform).toHaveBeenCalledWith(2, 0, 0, 2, 0, 0);
    expect(clearRect).toHaveBeenCalledTimes(1);
    expect(clearRect).toHaveBeenCalledWith(0, 0, 100, 50);
  });

  it('falls back to dpr 1 when window.devicePixelRatio is undefined (|| falsy arm, undefined)', () => {
    setDevicePixelRatio(undefined);
    const { canvas, ctx, setTransform } = makeCanvasAndCtx();

    setupCanvasDPR(canvas, ctx, 100, 50);

    // dpr = undefined || 1 = 1, so targets equal the logical size verbatim.
    expect(canvas.width).toBe(100);
    expect(canvas.height).toBe(50);
    expect(canvas.style.width).toBe('100px');
    expect(setTransform).toHaveBeenCalledWith(1, 0, 0, 1, 0, 0);
  });

  it('falls back to dpr 1 when window.devicePixelRatio is 0 (|| falsy arm, numeric zero)', () => {
    setDevicePixelRatio(0);
    const { canvas, ctx, setTransform } = makeCanvasAndCtx();

    setupCanvasDPR(canvas, ctx, 80, 40);

    // 0 || 1 === 1; a naive `dpr` use would have produced 0x0 targets.
    expect(canvas.width).toBe(80);
    expect(canvas.height).toBe(40);
    expect(setTransform).toHaveBeenCalledWith(1, 0, 0, 1, 0, 0);
  });

  it('resizes when only the height differs while width already matches (|| left-falsy / right-truthy arm)', () => {
    setDevicePixelRatio(1);
    const { canvas, ctx, setTransform, clearRect } = makeCanvasAndCtx();

    canvas.width = 100; // matches targetWidth (100 * 1) -> left operand falsy.
    canvas.height = 777; // differs from targetHeight (50 * 1) -> right truthy.

    setupCanvasDPR(canvas, ctx, 100, 50);

    expect(canvas.width).toBe(100);
    expect(canvas.height).toBe(50);
    expect(canvas.style.width).toBe('100px');
    expect(canvas.style.height).toBe('50px');
    expect(setTransform).toHaveBeenCalledWith(1, 0, 0, 1, 0, 0);
    expect(clearRect).not.toHaveBeenCalled();
  });

  it('resizes when only the width differs while height already matches (|| left-truthy short-circuit arm)', () => {
    setDevicePixelRatio(1);
    const { canvas, ctx, clearRect } = makeCanvasAndCtx();

    canvas.width = 999; // differs from targetWidth (100 * 1) -> left truthy.
    canvas.height = 50; // matches targetHeight -> would be right operand.

    setupCanvasDPR(canvas, ctx, 100, 50);

    expect(canvas.width).toBe(100);
    expect(canvas.height).toBe(50);
    expect(canvas.style.width).toBe('100px');
    expect(canvas.style.height).toBe('50px');
    expect(clearRect).not.toHaveBeenCalled();
  });

  it('rounds fractional width*dpr and height*dpr products via Math.round', () => {
    setDevicePixelRatio(1.5);
    const { canvas, ctx } = makeCanvasAndCtx();

    // 101 * 1.5 = 151.5 -> rounds to 152 (Math.round rounds half up).
    setupCanvasDPR(canvas, ctx, 101, 101);

    expect(canvas.width).toBe(152);
    expect(canvas.height).toBe(152);
    expect(canvas.style.width).toBe('101px');
    expect(canvas.style.height).toBe('101px');
  });

  it('handles the zero-size boundary (w=0, h=0) without producing NaN', () => {
    setDevicePixelRatio(2);
    const { canvas, ctx, setTransform, clearRect } = makeCanvasAndCtx();

    setupCanvasDPR(canvas, ctx, 0, 0);

    expect(canvas.width).toBe(0);
    expect(canvas.height).toBe(0);
    expect(canvas.style.width).toBe('0px');
    expect(canvas.style.height).toBe('0px');
    expect(setTransform).toHaveBeenCalledWith(2, 0, 0, 2, 0, 0);
    expect(clearRect).not.toHaveBeenCalled();
  });

  it('re-resizes after a backing-store change on a subsequent call (idempotent second pass takes the clear path)', () => {
    setDevicePixelRatio(2);
    const { canvas, ctx, setTransform, clearRect } = makeCanvasAndCtx();

    // First call: fresh canvas -> resize path.
    setupCanvasDPR(canvas, ctx, 120, 60);
    expect(canvas.width).toBe(240);
    expect(canvas.height).toBe(120);
    expect(clearRect).not.toHaveBeenCalled();

    // Second call with the same logical size: backing store now matches ->
    // no resize, clearRect path.
    setTransform.mockClear();
    clearRect.mockClear();
    setupCanvasDPR(canvas, ctx, 120, 60);
    expect(canvas.width).toBe(240);
    expect(canvas.height).toBe(120);
    expect(setTransform).toHaveBeenCalledTimes(1);
    expect(setTransform).toHaveBeenCalledWith(2, 0, 0, 2, 0, 0);
    expect(clearRect).toHaveBeenCalledTimes(1);
    expect(clearRect).toHaveBeenCalledWith(0, 0, 120, 60);
  });
});
