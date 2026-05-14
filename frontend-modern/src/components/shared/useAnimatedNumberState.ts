import { createEffect, createMemo, createSignal, onCleanup, onMount, untrack } from 'solid-js';
import {
  DEFAULT_ANIMATED_NUMBER_DURATION_MS,
  REDUCED_MOTION_QUERY,
  easeAnimatedNumberProgress,
  formatAnimatedInteger,
  sanitizeAnimatedNumberValue,
} from './animatedNumberModel';

export interface AnimatedNumberStateProps {
  value: number;
  durationMs?: number;
  disabled?: boolean;
  format?: (value: number) => string;
}

interface AnimatedNumberFrameEntry {
  durationMs: number;
  from: number;
  onDone: () => void;
  onFrame: (value: number) => void;
  startedAt?: number;
  to: number;
}

type MotionMediaQueryList = MediaQueryList & {
  addListener?: (listener: (event: MediaQueryListEvent) => void) => void;
  removeListener?: (listener: (event: MediaQueryListEvent) => void) => void;
};

const activeFrameEntries = new Map<number, AnimatedNumberFrameEntry>();
let nextFrameEntryId = 1;
let frameRequestId: number | undefined;

function canUseAnimationFrame(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.requestAnimationFrame === 'function' &&
    typeof window.cancelAnimationFrame === 'function'
  );
}

function cancelFrameLoopIfIdle() {
  if (activeFrameEntries.size > 0 || frameRequestId === undefined || !canUseAnimationFrame()) {
    return;
  }
  window.cancelAnimationFrame(frameRequestId);
  frameRequestId = undefined;
}

function flushAnimatedNumberFrame(now: number) {
  frameRequestId = undefined;

  for (const [id, entry] of activeFrameEntries) {
    if (entry.startedAt === undefined) {
      entry.startedAt = now;
    }

    const elapsed = Math.max(0, now - entry.startedAt);
    const progress = entry.durationMs > 0 ? Math.min(elapsed / entry.durationMs, 1) : 1;
    const eased = easeAnimatedNumberProgress(progress);
    entry.onFrame(entry.from + (entry.to - entry.from) * eased);

    if (progress >= 1) {
      activeFrameEntries.delete(id);
      entry.onDone();
    }
  }

  requestNextAnimatedNumberFrame();
}

function requestNextAnimatedNumberFrame() {
  if (activeFrameEntries.size === 0 || frameRequestId !== undefined || !canUseAnimationFrame()) {
    return;
  }
  frameRequestId = window.requestAnimationFrame(flushAnimatedNumberFrame);
}

function startAnimatedNumberFrame(entry: AnimatedNumberFrameEntry): () => void {
  if (!canUseAnimationFrame()) {
    entry.onFrame(entry.to);
    entry.onDone();
    return () => undefined;
  }

  const id = nextFrameEntryId;
  nextFrameEntryId += 1;
  activeFrameEntries.set(id, entry);
  requestNextAnimatedNumberFrame();

  return () => {
    activeFrameEntries.delete(id);
    cancelFrameLoopIfIdle();
  };
}

function readReducedMotionPreference(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false;
  }
  return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

export function useAnimatedNumberState(props: AnimatedNumberStateProps) {
  const [displayValue, setDisplayValue] = createSignal(sanitizeAnimatedNumberValue(props.value));
  const [reducedMotion, setReducedMotion] = createSignal(readReducedMotionPreference());
  let cancelAnimation: (() => void) | undefined;
  let motionQuery: MotionMediaQueryList | undefined;

  const formatter = createMemo(() => props.format ?? formatAnimatedInteger);
  const targetValue = () => sanitizeAnimatedNumberValue(props.value);
  const duration = () => Math.max(0, props.durationMs ?? DEFAULT_ANIMATED_NUMBER_DURATION_MS);
  const shouldSnap = () => props.disabled === true || reducedMotion() || duration() === 0;

  const cancelActiveAnimation = () => {
    cancelAnimation?.();
    cancelAnimation = undefined;
  };

  createEffect(() => {
    const target = targetValue();
    cancelActiveAnimation();

    if (shouldSnap()) {
      setDisplayValue(target);
      return;
    }

    const from = untrack(displayValue);
    if (from === target) {
      return;
    }

    cancelAnimation = startAnimatedNumberFrame({
      durationMs: duration(),
      from,
      onDone: () => {
        setDisplayValue(target);
        cancelAnimation = undefined;
      },
      onFrame: setDisplayValue,
      to: target,
    });
  });

  const handleMotionPreferenceChange = () => {
    setReducedMotion(Boolean(motionQuery?.matches));
  };

  onMount(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
      return;
    }

    motionQuery = window.matchMedia(REDUCED_MOTION_QUERY) as MotionMediaQueryList;
    setReducedMotion(motionQuery.matches);

    if (typeof motionQuery.addEventListener === 'function') {
      motionQuery.addEventListener('change', handleMotionPreferenceChange);
      return;
    }
    motionQuery.addListener?.(handleMotionPreferenceChange);
  });

  onCleanup(() => {
    cancelActiveAnimation();
    if (!motionQuery) {
      return;
    }
    if (typeof motionQuery.removeEventListener === 'function') {
      motionQuery.removeEventListener('change', handleMotionPreferenceChange);
      return;
    }
    motionQuery.removeListener?.(handleMotionPreferenceChange);
  });

  return {
    displayText: () => formatter()(displayValue()),
    targetText: () => formatter()(targetValue()),
  };
}
