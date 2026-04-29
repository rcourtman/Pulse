import { formatTemperature, getTemperatureSymbol } from '@/utils/temperature';
import type { ThresholdSliderMetricType } from '@/utils/thresholdSliderPresentation';

export interface ThresholdSliderProps {
  value: number;
  onChange: (value: number) => void;
  type: ThresholdSliderMetricType;
  min?: number;
  max?: number;
  disabled?: boolean;
}

export const DEFAULT_THRESHOLD_SLIDER_MIN = 0;
export const DEFAULT_THRESHOLD_SLIDER_MAX = 100;

export function getThresholdSliderBounds(min?: number, max?: number): {
  min: number;
  max: number;
} {
  return {
    min: min ?? DEFAULT_THRESHOLD_SLIDER_MIN,
    max: max ?? DEFAULT_THRESHOLD_SLIDER_MAX,
  };
}

export function getThresholdSliderPosition(value: number, min?: number, max?: number): number {
  const bounds = getThresholdSliderBounds(min, max);
  const range = bounds.max - bounds.min;
  if (range <= 0) {
    return 0;
  }

  const percent = ((value - bounds.min) / range) * 100;
  return Math.max(0, Math.min(100, percent));
}

export function getThresholdSliderThumbTransform(position: number): string {
  if (position <= 1) {
    return 'translateY(-50%) translateX(0%)';
  }
  if (position >= 99) {
    return 'translateY(-50%) translateX(-100%)';
  }
  return 'translateY(-50%) translateX(-50%)';
}

export function getThresholdSliderTitle(type: ThresholdSliderMetricType, value: number): string {
  return type === 'temperature'
    ? `Temperature: ${formatTemperature(value)}`
    : `${type.toUpperCase()}: ${value}%`;
}

export function getThresholdSliderLabel(type: ThresholdSliderMetricType, value: number): string {
  return type === 'temperature' ? `${value}${getTemperatureSymbol()}` : `${value}%`;
}
