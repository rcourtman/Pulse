const HOVER_CAPABLE_POINTER_QUERY = '(hover: hover) and (pointer: fine)';

/**
 * Floating hover UI is only appropriate when the device's primary pointer can
 * hover precisely. Touch browsers may synthesize mouseenter before click, so
 * viewport width alone cannot safely distinguish intent.
 */
export function supportsHoverTooltips(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return true;
  }
  return window.matchMedia(HOVER_CAPABLE_POINTER_QUERY)?.matches ?? true;
}
