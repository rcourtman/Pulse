import { Component, createEffect, createSignal, onCleanup } from 'solid-js';
import { formatBytes } from '@/utils/format';

interface AnimatedMetricProps {
  value: number;
  formatter?: (value: number) => string;
  className?: string;
}

export const AnimatedMetric: Component<AnimatedMetricProps> = (props) => {
  const [displayValue, setDisplayValue] = createSignal(props.value);
  const [oldValue, setOldValue] = createSignal(props.value);
  const [showGhost, setShowGhost] = createSignal(false);
  const [animClass, setAnimClass] = createSignal('');
  let timeoutId: number;
  let hasInitialized = false;

  createEffect(() => {
    const newVal = props.value;
    const prevVal = displayValue();

    // Skip first render
    if (!hasInitialized) {
      hasInitialized = true;
      setDisplayValue(newVal);
      setOldValue(newVal);
      return;
    }

    // Only animate if value changed
    if (newVal !== prevVal) {
      clearTimeout(timeoutId);

      // Store old value for ghost
      setOldValue(prevVal);
      setShowGhost(true);

      // Set animation direction
      if (newVal > prevVal) {
        setAnimClass('up');
        console.log('[AnimatedMetric] Going UP:', prevVal, '->', newVal);
      } else {
        setAnimClass('down');
        console.log('[AnimatedMetric] Going DOWN:', prevVal, '->', newVal);
      }

      // Update to new value
      setDisplayValue(newVal);

      // Remove ghost after animation
      timeoutId = window.setTimeout(() => {
        setShowGhost(false);
        setAnimClass('');
      }, 500);
    }
  });

  onCleanup(() => clearTimeout(timeoutId));

  const format = props.formatter || ((v: number) => formatBytes(v) + '/s');

  return (
    <div
      class="metric-container"
      style="position: relative; display: inline-block; overflow: visible;"
    >
      {showGhost() && (
        <span
          class={`metric-ghost metric-ghost-${animClass()}`}
          style="position: absolute; top: 0; left: 0; z-index: 1;"
        >
          {format(oldValue())}
        </span>
      )}
      <span
        class={`metric-value ${showGhost() ? `metric-entering-${animClass()}` : ''}`}
        style="position: relative; z-index: 2; display: inline-block;"
      >
        {format(displayValue())}
      </span>
    </div>
  );
};
