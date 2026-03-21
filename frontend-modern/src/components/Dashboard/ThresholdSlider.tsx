import {
  getThresholdSliderFillClass,
  getThresholdSliderTextClass,
} from '@/utils/thresholdSliderPresentation';
import { type ThresholdSliderProps } from './thresholdSliderModel';
import { useThresholdSliderState } from './useThresholdSliderState';

export function ThresholdSlider(props: ThresholdSliderProps) {
  const state = useThresholdSliderState(props);

  return (
    <div
      class={`relative w-full h-3.5 overflow-visible transition-opacity ${props.disabled ? 'opacity-30 grayscale pointer-events-none' : ''}`}
      onWheel={state.handleContainerWheel}
      style={{ 'touch-action': state.isDragging() ? 'none' : 'auto' }}
    >
      {/* Track background */}
      <div class="absolute inset-0 h-3.5 rounded bg-surface-hover"></div>

      {/* Colored fill */}
      <div
        class={`absolute left-0 h-3.5 rounded ${getThresholdSliderFillClass(props.type)}`}
        style={{ width: `${state.thumbPosition()}%` }}
      ></div>

      {/* Native range input (invisible but functional) */}
      <input
        type="range"
        min={props.min ?? 0}
        max={props.max ?? 100}
        value={props.value}
        onInput={state.handleInput}
        onMouseDown={state.handleMouseDown}
        onWheel={state.handleInputWheel}
        class={`absolute inset-0 w-full h-3.5 opacity-0 ${props.disabled ? 'cursor-not-allowed' : 'cursor-pointer'} z-20`}
        disabled={props.disabled}
        style={{ 'touch-action': 'none' }}
        title={state.sliderTitle()}
      />

      {/* Custom thumb with value */}
      <div
        class={`absolute top-1/2 pointer-events-none z-10 ${getThresholdSliderTextClass(props.type)}`}
        style={{
          left: `${state.thumbPosition()}%`,
          transform: state.thumbTransform(),
        }}
      >
        <div class="relative">
          <div class="w-9 h-4 bg-surface rounded-full shadow-sm border-2 border-current flex items-center justify-center">
            <span class="text-[9px] font-semibold">{state.sliderLabel()}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
