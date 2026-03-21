import { createMemo, createSignal, onCleanup } from 'solid-js';
import type { JSX } from 'solid-js';
import {
  getThresholdSliderLabel,
  getThresholdSliderPosition,
  getThresholdSliderThumbTransform,
  getThresholdSliderTitle,
  type ThresholdSliderProps,
} from './thresholdSliderModel';

type ThresholdSliderInputEvent = InputEvent & {
  currentTarget: HTMLInputElement;
  target: HTMLInputElement;
};

type ThresholdSliderWheelEvent<T extends HTMLElement> = WheelEvent & {
  currentTarget: T;
  target: Element;
};

export function useThresholdSliderState(props: ThresholdSliderProps) {
  const [isDragging, setIsDragging] = createSignal(false);
  let releaseDragLock: (() => void) | undefined;

  const clearDragLock = () => {
    releaseDragLock?.();
    releaseDragLock = undefined;
    setIsDragging(false);
  };

  const thumbPosition = createMemo(() =>
    getThresholdSliderPosition(props.value, props.min, props.max),
  );
  const thumbTransform = createMemo(() => getThresholdSliderThumbTransform(thumbPosition()));
  const sliderTitle = createMemo(() => getThresholdSliderTitle(props.type, props.value));
  const sliderLabel = createMemo(() => getThresholdSliderLabel(props.type, props.value));

  const handleInput = (event: ThresholdSliderInputEvent) => {
    props.onChange(parseInt(event.currentTarget.value, 10));
  };

  const handleInputWheel: JSX.EventHandlerUnion<
    HTMLInputElement,
    ThresholdSliderWheelEvent<HTMLInputElement>
  > = (event) => {
    if (!props.disabled) {
      event.preventDefault();
    }
  };

  const handleContainerWheel: JSX.EventHandlerUnion<
    HTMLDivElement,
    ThresholdSliderWheelEvent<HTMLDivElement>
  > = (event) => {
    if (!props.disabled && isDragging()) {
      event.preventDefault();
    }
  };

  const handleMouseDown = () => {
    if (props.disabled) {
      return;
    }

    clearDragLock();
    setIsDragging(true);

    const scrollY = window.scrollY;
    const scrollX = window.scrollX;

    const handleScroll = () => {
      window.scrollTo(scrollX, scrollY);
    };

    const handleMouseUp = () => {
      clearDragLock();
    };

    window.addEventListener('scroll', handleScroll, { capture: true });
    document.addEventListener('mouseup', handleMouseUp);

    releaseDragLock = () => {
      window.removeEventListener('scroll', handleScroll, { capture: true });
      document.removeEventListener('mouseup', handleMouseUp);
    };
  };

  onCleanup(() => {
    clearDragLock();
  });

  return {
    handleContainerWheel,
    handleInput,
    handleInputWheel,
    handleMouseDown: props.disabled ? undefined : handleMouseDown,
    isDragging,
    sliderLabel,
    sliderTitle,
    thumbPosition,
    thumbTransform,
  };
}
