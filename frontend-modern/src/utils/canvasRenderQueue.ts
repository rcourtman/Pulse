/**
 * Canvas Render Queue
 *
 * Shared requestAnimationFrame scheduler for batching canvas redraws.
 * Prevents each sparkline from scheduling its own rAF cycle.
 */

import { logger } from './logger';

let rafId: number | null = null;
const pending = new Set<() => void>();

/**
 * Flush all pending render callbacks in a single rAF cycle
 */
const flush = (): void => {
  rafId = null;
  const count = pending.size;

  pending.forEach((draw) => {
    try {
      draw();
    } catch (error) {
      logger.error('[CanvasRenderQueue] Draw callback failed', { error });
    }
  });

  pending.clear();

  if (import.meta.env.DEV) {
    logger.debug('[CanvasRenderQueue] Flushed render queue', { count });
  }
};

/**
 * Schedule a canvas redraw
 * Returns an unregister callback for cleanup
 *
 * @param draw - The draw callback to execute on next animation frame
 * @returns Cleanup function to unregister the callback
 */
/**
 * Set up a canvas for HiDPI rendering.
 * Handles DPR scaling, resize detection (to avoid flicker), transform reset, and clearing.
 * Returns the logical (CSS) width and height for drawing commands.
 */
export function setupCanvasDPR(
  canvas: HTMLCanvasElement,
  ctx: CanvasRenderingContext2D,
  w: number,
  h: number,
): void {
  const dpr = window.devicePixelRatio || 1;
  const targetWidth = Math.round(w * dpr);
  const targetHeight = Math.round(h * dpr);

  const needsResize = canvas.width !== targetWidth || canvas.height !== targetHeight;

  if (needsResize) {
    canvas.width = targetWidth;
    canvas.height = targetHeight;
    canvas.style.width = `${w}px`;
    canvas.style.height = `${h}px`;
  }

  // Reset transform and apply DPR scale before drawing
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

  // Clear canvas (if not already cleared by resize)
  if (!needsResize) {
    ctx.clearRect(0, 0, w, h);
  }
}

export function scheduleSparkline(draw: () => void): () => void {
  pending.add(draw);

  if (rafId === null) {
    rafId = requestAnimationFrame(flush);
  }

  return () => {
    pending.delete(draw);
    if (pending.size === 0 && rafId !== null) {
      cancelAnimationFrame(rafId);
      rafId = null;
    }
  };
}
