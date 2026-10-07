import { createSignal, getOwner, onCleanup } from 'solid-js';

// One clock for every relative time on screen ("2h ago"), so ages keep moving
// while rows stay mounted. Tables keep their rows across data refreshes, and a
// timestamp that does not change would otherwise freeze its age at whatever it
// read when the row first rendered. It ticks while anything reads it and stops
// when nothing does.
export const RELATIVE_TIME_TICK_MS = 30_000;

// The tick only tells readers to re-read; every read returns the wall clock.
// A cell that mounts between ticks would otherwise measure from up to one tick
// ago, so a timestamp from the last few seconds could read as in the future
// and two cells formatted from the same time could disagree.
const [tick, setTick] = createSignal(0);
const now = (): number => {
  tick();
  return Date.now();
};
let readers = 0;
let timer: ReturnType<typeof setInterval> | undefined;

// Inside a component or reactive root, the returned accessor tracks the shared
// tick. Outside one there is nothing to stop a timer, so it reads the time
// directly.
export function useRelativeTimeNow(): () => number {
  if (!getOwner()) return () => Date.now();
  readers += 1;
  if (timer === undefined) {
    timer = setInterval(() => setTick((count) => count + 1), RELATIVE_TIME_TICK_MS);
  }
  onCleanup(() => {
    readers -= 1;
    if (readers === 0 && timer !== undefined) {
      clearInterval(timer);
      timer = undefined;
    }
  });
  return now;
}
