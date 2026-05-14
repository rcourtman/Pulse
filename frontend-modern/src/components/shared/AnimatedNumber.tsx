import type { Component } from 'solid-js';
import { type AnimatedNumberStateProps, useAnimatedNumberState } from './useAnimatedNumberState';

export interface AnimatedNumberProps extends AnimatedNumberStateProps {
  class?: string;
}

export const AnimatedNumber: Component<AnimatedNumberProps> = (props) => {
  const state = useAnimatedNumberState(props);

  return (
    <span
      class={`animated-number tabular-nums ${props.class ?? ''}`.trim()}
      data-animated-number="true"
      aria-label={state.targetText()}
    >
      {state.displayText()}
    </span>
  );
};
