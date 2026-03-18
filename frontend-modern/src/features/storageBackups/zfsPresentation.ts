import type { ZFSDevice } from '@/types/api';

export const getZfsDeviceBlockClass = (device: ZFSDevice): string => {
  const state = device.state?.toUpperCase();
  if (state === 'ONLINE') return 'bg-green-500 dark:bg-green-500 hover:bg-green-400';
  if (state === 'DEGRADED') return 'bg-yellow-500 dark:bg-yellow-500 hover:bg-yellow-400';
  if (state === 'FAULTED' || state === 'UNAVAIL' || state === 'OFFLINE') {
    return 'bg-red-500 dark:bg-red-500 hover:bg-red-400';
  }
  return 'bg-slate-400 hover:bg-slate-300';
};

export const getZfsDeviceStateTextClass = (device: ZFSDevice): string => {
  const state = device.state?.toUpperCase();
  if (state === 'ONLINE') return 'text-green-400';
  if (state === 'DEGRADED') return 'text-yellow-400';
  return 'text-red-400';
};

export const getZfsTooltipErrorTextClass = (hasErrors: boolean): string =>
  hasErrors ? 'text-red-400' : 'text-green-400';

export const getZfsPoolStateTextClass = (hasErrors: boolean): string =>
  hasErrors ? 'text-red-400 font-bold' : 'text-green-400';

export const getZfsPoolErrorOverlayClass = (hasErrors: boolean): string =>
  hasErrors ? 'absolute inset-0 rounded border-2 border-red-500 animate-pulse' : '';
