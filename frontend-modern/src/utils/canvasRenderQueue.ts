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
export function scheduleSparkline(draw: () => void): () => void {
  pending.add(draw);

  if (rafId === null) {
    rafId = requestAnimationFrame(flush);
  }

  return () => {
    pending.delete(draw);
  };
}

