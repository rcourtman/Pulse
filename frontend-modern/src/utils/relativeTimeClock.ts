import { createSignal, getOwner, onCleanup } from 'solid-js';

// One clock for every relative time on screen ("2h ago"), so ages keep moving
// while rows stay mounted. Tables keep their rows across data refreshes, and a
// timestamp that does not change would otherwise freeze its age at whatever it
// read when the row first rendered. It ticks while anything reads it and stops
// when nothing does.
export const RELATIVE_TIME_TICK_MS = 30_000;

const [now, setNow] = createSignal(Date.now());
let readers = 0;
let timer: ReturnType<typeof setInterval> | undefined;

// Inside a component or reactive root, the returned accessor is the shared
// ticking signal. Outside one there is nothing to stop a timer, so it reads the
// time directly.
export function useRelativeTimeNow(): () => number {
  if (!getOwner()) return () => Date.now();
  readers += 1;
  if (timer === undefined) {
    setNow(Date.now());
    timer = setInterval(() => setNow(Date.now()), RELATIVE_TIME_TICK_MS);
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
