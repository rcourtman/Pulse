export const DEFAULT_ANIMATED_NUMBER_DURATION_MS = 320;

export const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

export function sanitizeAnimatedNumberValue(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return value;
}

export function easeAnimatedNumberProgress(progress: number): number {
  const bounded = Math.max(0, Math.min(progress, 1));
  return 1 - Math.pow(1 - bounded, 3);
}

export function formatAnimatedInteger(value: number): string {
  return String(Math.round(sanitizeAnimatedNumberValue(value)));
}
