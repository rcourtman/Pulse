export interface PatrolScheduleOption {
  value: number;
  label: string;
}

export const PATROL_SCHEDULE_PRESETS: readonly PatrolScheduleOption[] = [
  { value: 0, label: 'Disabled' },
  { value: 10, label: '10 min' },
  { value: 15, label: '15 min' },
  { value: 30, label: '30 min' },
  { value: 60, label: '1 hour' },
  { value: 180, label: '3 hours' },
  { value: 360, label: '6 hours' },
  { value: 720, label: '12 hours' },
  { value: 1440, label: '24 hours' },
];

export function getPatrolScheduleLabel(minutes: number): string {
  const preset = PATROL_SCHEDULE_PRESETS.find((option) => option.value === minutes);
  if (preset) return preset.label;
  return `${minutes} min`;
}

export function buildPatrolScheduleOptions(current: number): PatrolScheduleOption[] {
  const options = [...PATROL_SCHEDULE_PRESETS];
  if (Number.isFinite(current) && !options.some((option) => option.value === current)) {
    options.push({ value: current, label: getPatrolScheduleLabel(current) });
    options.sort((a, b) => a.value - b.value);
  }
  return options;
}
