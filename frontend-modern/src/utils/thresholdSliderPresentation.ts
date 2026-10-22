export type ThresholdSliderMetricType = 'cpu' | 'memory' | 'disk' | 'temperature';

const THRESHOLD_SLIDER_TEXT_CLASSES: Record<ThresholdSliderMetricType, string> = {
  cpu: 'text-blue-700 dark:text-blue-300',
  memory: 'text-green-700 dark:text-green-300',
  disk: 'text-amber-700 dark:text-amber-300',
  temperature: 'text-rose-700 dark:text-rose-300',
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
