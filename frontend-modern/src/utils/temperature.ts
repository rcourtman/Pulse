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

export type NvmeTemperatureReading = {
  value: number;
  device?: string;
};

export const getHottestNvmeTemperature = (
  temperature?: Temperature | null,
): NvmeTemperatureReading | null => {
  if (!temperature?.available || !Array.isArray(temperature.nvme)) {
    return null;
  }

  const readings = temperature.nvme
    .filter((nvme) => isValidTemperature(nvme.temp))
    .map((nvme) => ({ value: nvme.temp, device: nvme.device }));

  if (readings.length === 0) {
    return null;
  }

  return readings.reduce((max, current) => (current.value > max.value ? current : max));
};
