import type { Temperature } from '@/types/api';

const isValidTemperature = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value);

export const getCpuTemperature = (temperature?: Temperature | null): number | null => {
  if (!temperature?.available) return null;

  const candidates: number[] = [];

  if (isValidTemperature(temperature.cpuPackage)) {
    candidates.push(temperature.cpuPackage);
  }
  if (isValidTemperature(temperature.cpuMax)) {
    candidates.push(temperature.cpuMax);
  }
  if (Array.isArray(temperature.cores)) {
    temperature.cores.forEach((core) => {
      if (isValidTemperature(core.temp)) {
        candidates.push(core.temp);
      }
    });
  }

  if (candidates.length === 0) {
    return null;
  }

  return Math.max(...candidates);
};
