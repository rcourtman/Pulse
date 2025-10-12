import type { Temperature } from '@/types/api';

export type PrimaryTemperatureReading = {
  value: number;
  source: 'cpu' | 'nvme';
  device?: string;
};

const isValidTemperature = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value);

export const getPrimaryTemperature = (
  temperature?: Temperature | null,
): PrimaryTemperatureReading | null => {
  if (!temperature?.available) return null;

  const cpuCandidates: number[] = [];

  if (isValidTemperature(temperature.cpuPackage)) {
    cpuCandidates.push(temperature.cpuPackage);
  }
  if (isValidTemperature(temperature.cpuMax)) {
    cpuCandidates.push(temperature.cpuMax);
  }

  if (cpuCandidates.length > 0) {
    return {
      value: Math.max(...cpuCandidates),
      source: 'cpu',
    };
  }

  const nvmeCandidates = (temperature.nvme ?? [])
    .filter((nvme) => isValidTemperature(nvme.temp))
    .map((nvme) => ({
      device: nvme.device,
      temp: nvme.temp,
    }));

  if (nvmeCandidates.length > 0) {
    const hottest = nvmeCandidates.reduce((max, current) =>
      current.temp > max.temp ? current : max,
    );

    return {
      value: hottest.temp,
      source: 'nvme',
      device: hottest.device,
    };
  }

  return null;
};
