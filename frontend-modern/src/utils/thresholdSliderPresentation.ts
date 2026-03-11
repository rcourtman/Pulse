export type ThresholdSliderMetricType = 'cpu' | 'memory' | 'disk' | 'temperature';

const THRESHOLD_SLIDER_TEXT_CLASSES: Record<ThresholdSliderMetricType, string> = {
  cpu: 'text-blue-500',
  memory: 'text-green-500',
  disk: 'text-amber-500',
  temperature: 'text-rose-500',
};

const THRESHOLD_SLIDER_FILL_CLASSES: Record<ThresholdSliderMetricType, string> = {
  cpu: 'bg-blue-500',
  memory: 'bg-green-500',
  disk: 'bg-amber-500',
  temperature: 'bg-rose-500',
};

export function getThresholdSliderTextClass(type: ThresholdSliderMetricType): string {
  return THRESHOLD_SLIDER_TEXT_CLASSES[type];
}

export function getThresholdSliderFillClass(type: ThresholdSliderMetricType): string {
  return THRESHOLD_SLIDER_FILL_CLASSES[type];
}
